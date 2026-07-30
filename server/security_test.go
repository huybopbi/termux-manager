package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGuardRejectsBadOrigin(t *testing.T) {
	s := &Server{Listen: "127.0.0.1", Port: 9876}
	h := s.guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9876/api/info", nil)
	req.Host = "127.0.0.1:9876"
	req.Header.Set("Origin", "http://evil.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rr.Code)
	}
}

func TestGuardRejectsDNSRebindWhenLoopback(t *testing.T) {
	s := &Server{Listen: "127.0.0.1", Port: 9876}
	h := s.guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://evil.example:9876/api/info", nil)
	req.Host = "evil.example:9876"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rr.Code)
	}
}

func TestGuardAllowsMatchingOrigin(t *testing.T) {
	s := &Server{Listen: "127.0.0.1", Port: 9876}
	h := s.guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9876/api/info", nil)
	req.Host = "127.0.0.1:9876"
	req.Header.Set("Origin", "http://127.0.0.1:9876")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestGuardSkipsHostAllowlistWhenAllInterfaces(t *testing.T) {
	s := &Server{Listen: "0.0.0.0", Port: 9876}
	h := s.guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://192.168.1.10:9876/api/info", nil)
	req.Host = "192.168.1.10:9876"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 for LAN host when listen=0.0.0.0, got %d", rr.Code)
	}
}
