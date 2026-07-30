package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

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
	default:
		// linux and android-via-linux (Termux)
		return fmt.Sprintf("manager-android-%s", arch)
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

	if rel.TagName == "v"+version {
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

func main() {
	if len(os.Args) > 1 && os.Args[1] == "update" {
		selfUpdate()
		return
	}

	port := flag.Int("port", 9876, "Port to listen on")
	root := flag.String("root", "", "Root directory (default: $HOME)")
	hidden := flag.Bool("hidden", false, "Show hidden files")
	noOpen := flag.Bool("no-open", false, "Don't open browser automatically")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("termux-manager", version)
		os.Exit(0)
	}

	// Resolve root
	rootDir := *root
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal("Cannot determine home directory:", err)
		}
		rootDir = home
	}

	// Ensure root exists
	if _, err := os.Stat(rootDir); err != nil {
		log.Fatalf("Root directory %q does not exist: %v", rootDir, err)
	}

	srv := &server.Server{
		Root:        rootDir,
		InitialRoot: rootDir,
		ShowHidden:  *hidden,
	}

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	url := fmt.Sprintf("http://%s", addr)

	handler := srv.Routes(staticFiles)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Graceful shutdown on Ctrl+C / kill
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		fmt.Println("\nShutting down...")
		httpServer.Close()
	}()

	fmt.Printf("termux-manager v%s\n", version)
	fmt.Printf("Root : %s\n", rootDir)
	fmt.Printf("URL  : %s\n", url)
	fmt.Println("Press Ctrl+C to stop")

	if !*noOpen {
		go termux.OpenURL(url)
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
