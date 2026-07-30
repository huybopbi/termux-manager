package server

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// guard rejects cross-origin requests and DNS-rebinding when bound to loopback.
// When listen is 0.0.0.0 / :: (LAN/ngrok), Host allowlist is skipped; Origin still
// must match Host when present.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			if !originMatchesHost(origin, r.Host) {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		}

		s.mu.RLock()
		listen := s.Listen
		s.mu.RUnlock()
		if isLoopbackListen(listen) {
			hostOnly := hostWithoutPort(r.Host)
			if !isLoopbackHost(hostOnly) {
				http.Error(w, "forbidden host", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func originMatchesHost(origin, reqHost string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, reqHost)
}

func hostWithoutPort(hostport string) string {
	h, _, err := net.SplitHostPort(hostport)
	if err != nil {
		// No port, or bare IPv6 without brackets — treat whole string as host.
		return hostport
	}
	return h
}

func isLoopbackListen(listen string) bool {
	switch strings.ToLower(strings.TrimSpace(listen)) {
	case "127.0.0.1", "::1", "localhost":
		return true
	default:
		return false
	}
}

func isLoopbackHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	switch h {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
