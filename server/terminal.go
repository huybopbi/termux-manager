package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"golang.org/x/net/websocket"
)

const (
	termBufMax       = 256 * 1024 // replay buffer per session
	termIdleTTL      = 6 * time.Hour
	termGCInterval   = time.Minute
)

// cleanTermEnv builds the PTY shell environment without SSH markers.
// Otherwise Powerlevel10k shows "with user@host" when the manager itself
// was started over SSH (inherited SSH_CLIENT / SSH_CONNECTION).
func cleanTermEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+2)
	for _, e := range env {
		switch {
		case strings.HasPrefix(e, "SSH_CLIENT="),
			strings.HasPrefix(e, "SSH_CONNECTION="),
			strings.HasPrefix(e, "SSH_TTY="):
			continue
		}
		out = append(out, e)
	}
	return append(out, "TERM=xterm-256color", "COLORTERM=truecolor")
}

// termMsg is the wire format between browser and server.
// type: "input"   → data: keystrokes from user
// type: "resize"  → cols/rows: terminal dimensions
// type: "kill"    → destroy persistent session
// type: "output"  → data: bytes from shell (server→client)
// type: "history" → buffered output on (re)attach
// type: "ready"   → id: session id
// type: "error"   → data: message
type termMsg struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	ID   string `json:"id,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

type termSession struct {
	id      string
	ptmx    *os.File
	cmd     *exec.Cmd
	mu      sync.Mutex
	writeMu sync.Mutex
	sub     *websocket.Conn
	buf     []byte
	closed  bool
	created time.Time
	seen    time.Time // last attach / input
}

type termManager struct {
	mu       sync.Mutex
	sessions map[string]*termSession
	once     sync.Once
}

var terms = &termManager{sessions: make(map[string]*termSession)}

func newSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func (m *termManager) startGC() {
	m.once.Do(func() {
		go func() {
			t := time.NewTicker(termGCInterval)
			defer t.Stop()
			for range t.C {
				m.gc()
			}
		}()
	})
}

func (m *termManager) gc() {
	cutoff := time.Now().Add(-termIdleTTL)
	m.mu.Lock()
	var stale []*termSession
	for _, s := range m.sessions {
		s.mu.Lock()
		idle := s.sub == nil && s.seen.Before(cutoff)
		s.mu.Unlock()
		if idle {
			stale = append(stale, s)
		}
	}
	m.mu.Unlock()
	for _, s := range stale {
		log.Printf("terminal: gc idle session %s", s.id)
		s.destroy()
	}
}

func (m *termManager) list() []map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]map[string]interface{}, 0, len(m.sessions))
	for _, s := range m.sessions {
		s.mu.Lock()
		if !s.closed {
			out = append(out, map[string]interface{}{
				"id":         s.id,
				"created":    s.created,
				"last_seen":  s.seen,
				"connected":  s.sub != nil,
				"title":      "bash " + s.id[:min(4, len(s.id))],
			})
		}
		s.mu.Unlock()
	}
	return out
}

func (m *termManager) get(id string) *termSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	if s == nil || s.closed {
		return nil
	}
	return s
}

func (m *termManager) create(workdir string) (*termSession, error) {
	m.startGC()

	shell := os.Getenv("SHELL")
	if shell == "" {
		if path, err := exec.LookPath("bash"); err == nil {
			shell = path
		} else {
			shell = "/bin/sh"
		}
	}

	cmd := exec.Command(shell)
	cmd.Env = cleanTermEnv()
	cmd.Dir = workdir

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	s := &termSession{
		id:      newSessionID(),
		ptmx:    ptmx,
		cmd:     cmd,
		created: time.Now(),
		seen:    time.Now(),
		buf:     make([]byte, 0, 4096),
	}

	m.mu.Lock()
	m.sessions[s.id] = s
	m.mu.Unlock()

	go s.readLoop()
	go s.waitExit()

	log.Printf("terminal: created session %s (cwd=%s)", s.id, workdir)
	return s, nil
}

func (m *termManager) remove(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

func (s *termSession) appendBuf(p []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = append(s.buf, p...)
	if len(s.buf) > termBufMax {
		s.buf = append([]byte(nil), s.buf[len(s.buf)-termBufMax:]...)
	}
}

func (s *termSession) history() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buf) == 0 {
		return ""
	}
	return string(s.buf)
}

func (s *termSession) send(ws *websocket.Conn, msg termMsg) error {
	if ws == nil {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return websocket.JSON.Send(ws, msg)
}

func (s *termSession) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			s.appendBuf(chunk)

			s.mu.Lock()
			ws := s.sub
			s.mu.Unlock()
			if ws != nil {
				if werr := s.send(ws, termMsg{Type: "output", Data: string(chunk)}); werr != nil {
					s.detach(ws)
				}
			}
		}
		if err != nil {
			s.destroy()
			return
		}
	}
}

func (s *termSession) waitExit() {
	_ = s.cmd.Wait()
	s.destroy()
}

func (s *termSession) detach(ws *websocket.Conn) {
	s.mu.Lock()
	if s.sub == ws {
		s.sub = nil
		s.seen = time.Now()
	}
	s.mu.Unlock()
}

func (s *termSession) attach(ws *websocket.Conn) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("session closed")
	}
	old := s.sub
	s.sub = nil
	s.seen = time.Now()
	hist := string(s.buf)
	id := s.id
	s.mu.Unlock()

	if old != nil && old != ws {
		_ = s.send(old, termMsg{Type: "error", Data: "session taken by another client\r\n"})
		old.Close()
	}

	if err := s.send(ws, termMsg{Type: "ready", ID: id}); err != nil {
		return err
	}
	if hist != "" {
		if err := s.send(ws, termMsg{Type: "history", Data: hist}); err != nil {
			return err
		}
	}

	s.mu.Lock()
	if !s.closed {
		s.sub = ws
		s.seen = time.Now()
	}
	s.mu.Unlock()
	return nil
}

func (s *termSession) destroy() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	ws := s.sub
	s.sub = nil
	s.mu.Unlock()

	if ws != nil {
		_ = s.send(ws, termMsg{Type: "error", Data: "session ended\r\n"})
		ws.Close()
	}
	_ = s.ptmx.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	terms.remove(s.id)
	log.Printf("terminal: destroyed session %s", s.id)
}

func (s *termSession) handleClient(ws *websocket.Conn) {
	defer func() {
		s.detach(ws)
		ws.Close()
	}()

	var msg termMsg
	for {
		if err := websocket.JSON.Receive(ws, &msg); err != nil {
			return
		}
		s.mu.Lock()
		closed := s.closed
		s.seen = time.Now()
		s.mu.Unlock()
		if closed {
			return
		}
		switch msg.Type {
		case "input":
			if _, err := s.ptmx.Write([]byte(msg.Data)); err != nil {
				return
			}
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				_ = pty.Setsize(s.ptmx, &pty.Winsize{Cols: msg.Cols, Rows: msg.Rows})
			}
		case "kill":
			s.destroy()
			return
		}
	}
}

// TerminalHandler upgrades to WebSocket and attaches to a persistent PTY session.
// Query: ?id=<session> to reattach; omit to create a new session.
func (s *Server) TerminalHandler() http.Handler {
	return websocket.Handler(func(ws *websocket.Conn) {
		req := ws.Request()
		wantID := ""
		if req != nil {
			wantID = req.URL.Query().Get("id")
		}

		var sess *termSession
		var err error

		if wantID != "" {
			sess = terms.get(wantID)
			if sess == nil {
				_ = websocket.JSON.Send(ws, termMsg{Type: "error", Data: "session not found — creating new\r\n"})
			}
		}
		if sess == nil {
			sess, err = terms.create(s.rootPath())
			if err != nil {
				log.Printf("pty start error: %v", err)
				_ = websocket.JSON.Send(ws, termMsg{Type: "error", Data: "Failed to start shell: " + err.Error() + "\r\n"})
				ws.Close()
				return
			}
		}

		if err := sess.attach(ws); err != nil {
			_ = websocket.JSON.Send(ws, termMsg{Type: "error", Data: err.Error() + "\r\n"})
			ws.Close()
			return
		}
		sess.handleClient(ws)
	})
}

// GET/DELETE /api/terminal/sessions[?id=...]
func (s *Server) handleTermSessionsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.ok(w, terms.list())
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			s.fail(w, http.StatusBadRequest, fmt.Errorf("id required"))
			return
		}
		sess := terms.get(id)
		if sess == nil {
			s.fail(w, http.StatusNotFound, fmt.Errorf("session not found"))
			return
		}
		sess.destroy()
		s.ok(w, nil)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
