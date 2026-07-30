package server

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

func (s *Server) Routes(static embed.FS) http.Handler {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/list", s.method("GET", s.handleList))
	mux.HandleFunc("/api/read", s.method("GET", s.handleRead))
	mux.HandleFunc("/api/write", s.method("POST", s.handleWrite))
	mux.HandleFunc("/api/delete", s.method("DELETE", s.handleDelete))
	mux.HandleFunc("/api/rename", s.method("POST", s.handleRename))
	mux.HandleFunc("/api/move", s.method("POST", s.handleMove))
	mux.HandleFunc("/api/copy", s.method("POST", s.handleCopy))
	mux.HandleFunc("/api/mkdir", s.method("POST", s.handleMkdir))
	mux.HandleFunc("/api/touch", s.method("POST", s.handleTouch))
	mux.HandleFunc("/api/search", s.method("GET", s.handleSearch))
	mux.HandleFunc("/api/download", s.method("GET", s.handleDownload))
	mux.HandleFunc("/api/upload", s.method("POST", s.handleUpload))
	mux.HandleFunc("/api/zip", s.method("POST", s.handleZip))
	mux.HandleFunc("/api/unzip", s.method("POST", s.handleUnzip))
	mux.HandleFunc("/api/untar", s.method("POST", s.handleUntar))
	mux.HandleFunc("/api/tar", s.method("POST", s.handleTar))
	mux.HandleFunc("/api/info", s.method("GET", s.handleInfo))
	mux.HandleFunc("/api/root", s.method("POST", s.handleSetRoot))
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/terminal/sessions", s.handleTermSessionsAPI)

	// Database browser
	mux.HandleFunc("/api/db/connect", s.method("POST", s.handleDBConnect))
	mux.HandleFunc("/api/db/disconnect", s.method("POST", s.handleDBDisconnect))
	mux.HandleFunc("/api/db/databases", s.method("GET", s.handleDBDatabases))
	mux.HandleFunc("/api/db/use", s.method("POST", s.handleDBUse))
	mux.HandleFunc("/api/db/tables", s.method("GET", s.handleDBTables))
	mux.HandleFunc("/api/db/columns", s.method("GET", s.handleDBColumns))
	mux.HandleFunc("/api/db/rows", s.method("GET", s.handleDBRows))
	mux.HandleFunc("/api/db/row", s.handleDBRow)
	mux.HandleFunc("/api/db/query", s.method("POST", s.handleDBQuery))

	// Termux-specific
	mux.HandleFunc("/api/termux/share", s.method("POST", s.handleTermuxShare))
	mux.HandleFunc("/api/termux/clipboard", s.method("POST", s.handleTermuxClipboard))
	mux.HandleFunc("/api/termux/exec", s.method("POST", s.handleTermuxExec))

	// WebSocket terminal (PTY) — persistent sessions
	mux.Handle("/ws/terminal", s.TerminalHandler())

	// Static frontend (embedded)
	sub, err := fs.Sub(static, "embed")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	return s.logging(mux)
}

func (s *Server) method(m string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method != m {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
