package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func okHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func TestPerIPLimit(t *testing.T) {
	// 2 req/s, burst 2 → third immediate request from the same IP must be rejected.
	rl := NewRateLimiter(2, 2, 100, 100, time.Minute)
	h := rl.WrapFunc(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: got %d", i, rr.Code)
		}
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rr.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "rate limit exceeded" {
		t.Fatalf("body %v", body)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After header")
	}
}

func TestGlobalLimit(t *testing.T) {
	// Global burst 1, per-IP generous → second request from any IP fails.
	rl := NewRateLimiter(100, 100, rate.Limit(1), 1, time.Minute)
	h := rl.WrapFunc(okHandler())

	req1 := httptest.NewRequest(http.MethodGet, "/api/a", nil)
	req1.RemoteAddr = "10.0.0.1:1"
	req2 := httptest.NewRequest(http.MethodGet, "/api/b", nil)
	req2.RemoteAddr = "10.0.0.2:2"

	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first: %d", rr1.Code)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second: expected 429, got %d", rr2.Code)
	}
}

func TestDifferentIPsGetSeparateBuckets(t *testing.T) {
	rl := NewRateLimiter(1, 1, 100, 100, time.Minute)
	h := rl.WrapFunc(okHandler())

	for _, ip := range []string{"10.0.0.1:1", "10.0.0.2:2", "10.0.0.3:3"} {
		req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("ip %s: got %d", ip, rr.Code)
		}
	}
}

func TestXForwardedFor(t *testing.T) {
	tests := []struct {
		name     string
		xff      string
		remote   string
		wantIP   string
	}{
		{"single", "1.2.3.4", "127.0.0.1:9999", "1.2.3.4"},
		{"chain", "1.2.3.4, 5.6.7.8", "127.0.0.1:9999", "1.2.3.4"},
		{"empty xff", "", "192.168.1.1:8080", "192.168.1.1"},
		{"no port", "", "10.0.0.1", "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			got := ClientIP(req)
			if got != tt.wantIP {
				t.Fatalf("ClientIP = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	rl := NewRateLimiter(10, 10, 100, 100, 50*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	req.RemoteAddr = "10.0.0.1:1"
	rr := httptest.NewRecorder()
	rl.WrapFunc(okHandler()).ServeHTTP(rr, req)

	rl.mu.Lock()
	n := len(rl.clients)
	rl.mu.Unlock()
	if n != 1 {
		t.Fatalf("expected 1 client, got %d", n)
	}

	time.Sleep(80 * time.Millisecond)
	rl.Cleanup()

	rl.mu.Lock()
	n = len(rl.clients)
	rl.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected 0 clients after cleanup, got %d", n)
	}
}
