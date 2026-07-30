package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/huybopbi/termux-manager/server"
	"github.com/huybopbi/termux-manager/termux"
)

//go:embed all:embed
var staticFiles embed.FS

var version = "0.1.0"

func main() {
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
