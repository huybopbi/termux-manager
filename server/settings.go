package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/huybopbi/termux-manager/config"
)

// GET|PUT /api/settings
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	switch r.Method {
	case http.MethodGet:
		s.handleGetSettings(w, r)
	case http.MethodPut, http.MethodPost:
		s.handlePutSettings(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GET /api/settings
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	listen, port, showHidden := s.Listen, s.Port, s.ShowHidden
	s.mu.RUnlock()
	cfgPath, _ := config.Path()
	s.ok(w, map[string]interface{}{
		"listen":       listen,
		"port":         port,
		"show_hidden":  showHidden,
		"config_path":  cfgPath,
		"presets": []map[string]string{
			{"value": "127.0.0.1", "label": "Localhost only (safe)"},
			{"value": "0.0.0.0", "label": "All interfaces (LAN / ngrok)"},
		},
	})
}

// PUT /api/settings  body: {"listen":"0.0.0.0","port":9876,"show_hidden":true}
func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Listen     string `json:"listen"`
		Port       int    `json:"port"`
		ShowHidden *bool  `json:"show_hidden"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}

	listen, err := normalizeListen(req.Listen)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if req.Port <= 0 || req.Port > 65535 {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("port must be 1–65535"))
		return
	}

	s.mu.RLock()
	oldListen, oldPort, showHidden := s.Listen, s.Port, s.ShowHidden
	s.mu.RUnlock()
	if req.ShowHidden != nil {
		showHidden = *req.ShowHidden
	}

	cfg := config.Config{Listen: listen, Port: req.Port, ShowHidden: showHidden}
	if err := config.Save(cfg); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}

	s.mu.Lock()
	s.Listen = listen
	s.Port = req.Port
	s.ShowHidden = showHidden
	s.mu.Unlock()

	changed := listen != oldListen || req.Port != oldPort
	resp := map[string]interface{}{
		"listen":       listen,
		"port":         req.Port,
		"show_hidden":  showHidden,
		"saved":        true,
		"relisten":     false,
		"needs_rebind": changed,
	}

	if changed && s.Relisten != nil {
		host, port := listen, req.Port
		go func() {
			time.Sleep(300 * time.Millisecond)
			if err := s.Relisten(host, port); err != nil {
				// Keep serving on old address if rebind fails; config is still saved.
				fmt.Printf("relisten failed: %v (saved for next start)\n", err)
			}
		}()
		resp["relisten"] = true
		resp["hint"] = "Server is rebinding. If you opened via localhost and switched to LAN, use your device IP."
	}

	s.ok(w, resp)
}

func normalizeListen(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", fmt.Errorf("listen address required")
	}
	switch strings.ToLower(v) {
	case "localhost", "loopback":
		return "127.0.0.1", nil
	case "any", "all", "*":
		return "0.0.0.0", nil
	}
	// Host only — reject host:port here (port is separate field)
	if strings.Contains(v, ":") && net.ParseIP(v) == nil {
		// Allow bare IPv6 without brackets? Reject for simplicity except :: and ::1
		if v != "::" && v != "::1" {
			return "", fmt.Errorf("listen must be a host (e.g. 127.0.0.1 or 0.0.0.0), not host:port")
		}
		return v, nil
	}
	if ip := net.ParseIP(v); ip != nil {
		return v, nil
	}
	// Hostname like "0.0.0.0" already handled; reject unknown hostnames for safety
	if v == "0.0.0.0" || v == "127.0.0.1" {
		return v, nil
	}
	return "", fmt.Errorf("invalid listen address %q (use 127.0.0.1 or 0.0.0.0)", v)
}

func FormatAddr(listen string, port int) string {
	host := listen
	if host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}
