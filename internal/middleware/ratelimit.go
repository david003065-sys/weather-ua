package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter enforces per-IP and global request rate limits.
type RateLimiter struct {
	global  *rate.Limiter
	mu      sync.Mutex
	clients map[string]*ipEntry
	perIP   rate.Limit
	burst   int
	window  time.Duration // entries older than this are eligible for eviction
}

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a limiter with the given per-IP and global caps.
// perIPRate/perIPBurst control individual IPs; globalRate/globalBurst are shared across all clients.
// staleWindow controls how long idle IP entries survive before the cleanup goroutine removes them.
func NewRateLimiter(perIPRate rate.Limit, perIPBurst int, globalRate rate.Limit, globalBurst int, staleWindow time.Duration) *RateLimiter {
	return &RateLimiter{
		global:  rate.NewLimiter(globalRate, globalBurst),
		clients: make(map[string]*ipEntry),
		perIP:   perIPRate,
		burst:   perIPBurst,
		window:  staleWindow,
	}
}

func (rl *RateLimiter) getClient(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	e, ok := rl.clients[ip]
	if !ok {
		e = &ipEntry{limiter: rate.NewLimiter(rl.perIP, rl.burst)}
		rl.clients[ip] = e
	}
	e.lastSeen = time.Now()
	return e.limiter
}

// Cleanup removes IP entries that haven't been seen within the stale window.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-rl.window)
	for ip, e := range rl.clients {
		if e.lastSeen.Before(cutoff) {
			delete(rl.clients, ip)
		}
	}
}

// StartCleanup launches a background goroutine that calls Cleanup at the given interval.
// It stops when done is closed.
func (rl *RateLimiter) StartCleanup(interval time.Duration, done <-chan struct{}) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				rl.Cleanup()
			case <-done:
				return
			}
		}
	}()
}

// Wrap returns middleware that rejects requests exceeding the rate limit with 429.
func (rl *RateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.global.Allow() {
			reject(w)
			return
		}
		ip := ClientIP(r)
		if !rl.getClient(ip).Allow() {
			reject(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// WrapFunc is a convenience wrapper for http.HandlerFunc.
func (rl *RateLimiter) WrapFunc(next http.HandlerFunc) http.Handler {
	return rl.Wrap(next)
}

func reject(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Retry-After", "60")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
}

// ClientIP extracts the caller IP from X-Forwarded-For (first entry) with RemoteAddr fallback.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			first = strings.TrimSpace(first)
			if first != "" {
				return first
			}
		} else {
			xff = strings.TrimSpace(xff)
			if xff != "" {
				return xff
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
