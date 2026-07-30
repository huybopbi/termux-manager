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
	"os/exec"
	"os/signal"
	"path/filepath"
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

// findTool locates curl/wget. Prefer PATH, then Termux $PREFIX/bin.
// External tools use the system DNS resolver (unlike pure-Go net with CGO_ENABLED=0).
func findTool(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	candidates := []string{}
	if prefix := os.Getenv("PREFIX"); prefix != "" {
		candidates = append(candidates, filepath.Join(prefix, "bin", name))
	}
	candidates = append(candidates, "/data/data/com.termux/files/usr/bin/"+name)
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

func fetchURL(url string) ([]byte, error) {
	if curl := findTool("curl"); curl != "" {
		out, err := exec.Command(curl, "-fsSL", url).Output()
		if err == nil {
			return out, nil
		}
	}
	if wget := findTool("wget"); wget != "" {
		out, err := exec.Command(wget, "-qO-", url).Output()
		if err == nil {
			return out, nil
		}
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func downloadURL(url, dest string) error {
	_ = os.Remove(dest)
	if curl := findTool("curl"); curl != "" {
		cmd := exec.Command(curl, "-fL", "--progress-bar", "-o", dest, url)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return os.Chmod(dest, 0o755)
		}
	}
	if wget := findTool("wget"); wget != "" {
		cmd := exec.Command(wget, "--show-progress", "-qO", dest, url)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return os.Chmod(dest, 0o755)
		}
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(dest)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(dest)
		return closeErr
	}
	return nil
}

func selfUpdate() {
	fmt.Println("Checking for updates...")

	name := assetName()
	apiURL := "https://api.github.com/repos/" + repo + "/releases/latest"
	// Stable redirect URL — works even if API JSON parse fails
	latestURL := "https://github.com/" + repo + "/releases/latest/download/" + name

	tag := ""
	if body, err := fetchURL(apiURL); err == nil {
		var rel struct {
			TagName string `json:"tag_name"`
			Assets  []struct {
				Name               string `json:"name"`
				BrowserDownloadURL string `json:"browser_download_url"`
			} `json:"assets"`
		}
		if json.Unmarshal(body, &rel) == nil && rel.TagName != "" {
			tag = rel.TagName
			for _, a := range rel.Assets {
				if a.Name == name && a.BrowserDownloadURL != "" {
					latestURL = a.BrowserDownloadURL
					break
				}
			}
		}
	} else {
		fmt.Println("Note: GitHub API unreachable via Go DNS; using curl/wget /latest/download …")
	}

	if tag != "" && strings.TrimPrefix(tag, "v") == strings.TrimPrefix(version, "v") {
		fmt.Printf("Already up to date (%s)\n", tag)
		return
	}

	if tag != "" {
		fmt.Printf("Updating %s → %s\n", version, tag)
	} else {
		fmt.Printf("Updating %s → latest (%s)\n", version, name)
	}

	exe, err := os.Executable()
	if err != nil {
		log.Fatal("Cannot find executable path:", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		log.Fatal("Cannot resolve executable path:", err)
	}

	tmp := exe + ".tmp"
	fmt.Println("Downloading", latestURL)
	if err := downloadURL(latestURL, tmp); err != nil {
		os.Remove(tmp)
		log.Fatal("Download failed:", err)
	}

	if err := os.Rename(tmp, exe); err != nil {
		os.Remove(tmp)
		log.Fatal("Cannot replace binary:", err)
	}

	if tag == "" {
		tag = "latest"
	}
	fmt.Printf("✓ Updated to %s — restart manager to apply\n", tag)
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
