package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/huybopbi/termux-manager/config"
	"github.com/huybopbi/termux-manager/server"
	"github.com/huybopbi/termux-manager/termux"
)

//go:embed all:embed
var staticFiles embed.FS

var version = "0.1.0"

const repo = "huybopbi/termux-manager"

func assetName() string {
	arch := runtime.GOARCH
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("manager-windows-%s.exe", arch)
	case "darwin":
		return fmt.Sprintf("manager-darwin-%s", arch)
	case "android":
		return fmt.Sprintf("manager-android-%s", arch)
	default:
		return fmt.Sprintf("manager-linux-%s", arch)
	}
}

func selfUpdate() {
	fmt.Println("Checking for updates...")

	resp, err := http.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		log.Fatal("Cannot reach GitHub:", err)
	}
	defer resp.Body.Close()

	var rel struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		log.Fatal("Cannot parse release info:", err)
	}

	if rel.TagName == "" {
		log.Fatal("No release found")
	}

	if strings.TrimPrefix(rel.TagName, "v") == strings.TrimPrefix(version, "v") {
		fmt.Printf("Already up to date (%s)\n", rel.TagName)
		return
	}

	name := assetName()
	var downloadURL string
	for _, a := range rel.Assets {
		if a.Name == name {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		log.Fatalf("No asset %q found in release %s", name, rel.TagName)
	}

	fmt.Printf("Updating %s → %s\n", version, rel.TagName)

	exe, err := os.Executable()
	if err != nil {
		log.Fatal("Cannot find executable path:", err)
	}

	dlResp, err := http.Get(downloadURL)
	if err != nil {
		log.Fatal("Download failed:", err)
	}
	defer dlResp.Body.Close()

	tmp := exe + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatal("Cannot write temp file:", err)
	}
	if _, err := io.Copy(f, dlResp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		log.Fatal("Download error:", err)
	}
	f.Close()

	if err := os.Rename(tmp, exe); err != nil {
		os.Remove(tmp)
		log.Fatal("Cannot replace binary:", err)
	}

	fmt.Printf("✓ Updated to %s — restart manager to apply\n", rel.TagName)
}

type httpListener struct {
	mu      sync.Mutex
	server  *http.Server
	handler http.Handler
}

func (h *httpListener) start(listen string, port int) error {
	addr := net.JoinHostPort(listen, strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: addr, Handler: h.handler}
	h.mu.Lock()
	old := h.server
	h.server = srv
	h.mu.Unlock()
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("http serve: %v", err)
		}
	}()
	if old != nil {
		go func() {
			time.Sleep(50 * time.Millisecond)
			_ = old.Close()
		}()
	}
	return nil
}

func (h *httpListener) close() {
	h.mu.Lock()
	srv := h.server
	h.mu.Unlock()
	if srv != nil {
		_ = srv.Close()
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "update" {
		selfUpdate()
		return
	}

	cfg := config.Load()

	listen := flag.String("listen", cfg.Listen, "Listen address (127.0.0.1 or 0.0.0.0 for LAN/ngrok)")
	port := flag.Int("port", cfg.Port, "Port to listen on")
	root := flag.String("root", "", "Root directory (default: $HOME)")
	hidden := flag.Bool("hidden", cfg.ShowHidden, "Show hidden files")
	noOpen := flag.Bool("no-open", false, "Don't open browser automatically")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("termux-manager", version)
		os.Exit(0)
	}

	rootDir := *root
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal("Cannot determine home directory:", err)
		}
		rootDir = home
	}

	if _, err := os.Stat(rootDir); err != nil {
		log.Fatalf("Root directory %q does not exist: %v", rootDir, err)
	}

	hl := &httpListener{}
	srv := &server.Server{
		Root:        rootDir,
		InitialRoot: rootDir,
		ShowHidden:  *hidden,
		Listen:      *listen,
		Port:        *port,
	}
	srv.Relisten = func(host string, p int) error {
		fmt.Printf("Rebinding to %s:%d …\n", host, p)
		if err := hl.start(host, p); err != nil {
			return err
		}
		fmt.Printf("Listening on http://%s\n", server.FormatAddr(host, p))
		return nil
	}

	hl.handler = srv.Routes(staticFiles)

	if err := hl.start(*listen, *port); err != nil {
		log.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		fmt.Println("\nShutting down...")
		hl.close()
		close(done)
	}()

	localURL := "http://" + server.FormatAddr(*listen, *port)
	fmt.Printf("termux-manager v%s\n", version)
	fmt.Printf("Root   : %s\n", rootDir)
	fmt.Printf("Listen : %s:%d\n", *listen, *port)
	fmt.Printf("URL    : %s\n", localURL)
	if *listen == "0.0.0.0" || *listen == "::" {
		fmt.Println("Note   : Bound on all interfaces — reachable via LAN IP / tunnels. No auth.")
	}
	fmt.Println("Press Ctrl+C to stop")

	if !*noOpen {
		go termux.OpenURL(localURL)
	}

	<-done
}
