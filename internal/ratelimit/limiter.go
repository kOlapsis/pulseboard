package ratelimit

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type visitor struct {
	mu        sync.Mutex
	tokens    float64
	lastRefil time.Time
}

// Limiter implements a per-IP token bucket rate limiter.
type Limiter struct {
	rate     float64
	burst    int
	visitors sync.Map // IP string → *visitor
}

// New creates a rate limiter allowing rate tokens per second with the given burst capacity.
func New(rate float64, burst int) *Limiter {
	return &Limiter{
		rate:  rate,
		burst: burst,
	}
}

// Allow consumes one token for the given IP and reports whether the request is allowed.
func (l *Limiter) Allow(ip string) bool {
	now := time.Now()

	val, loaded := l.visitors.LoadOrStore(ip, &visitor{
		tokens:    float64(l.burst) - 1,
		lastRefil: now,
	})
	if !loaded {
		return true
	}

	v := val.(*visitor)
	v.mu.Lock()
	defer v.mu.Unlock()

	elapsed := now.Sub(v.lastRefil).Seconds()
	v.tokens += elapsed * l.rate
	if v.tokens > float64(l.burst) {
		v.tokens = float64(l.burst)
	}
	v.lastRefil = now

	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

// Start launches a background goroutine that evicts inactive visitors every 60 seconds.
// It stops when ctx is cancelled.
func (l *Limiter) Start(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			cutoff := now.Add(-3 * time.Minute)
			l.visitors.Range(func(key, val any) bool {
				v := val.(*visitor)
				v.mu.Lock()
				inactive := v.lastRefil.Before(cutoff)
				v.mu.Unlock()
				if inactive {
					l.visitors.Delete(key)
				}
				return true
			})
		}
	}
}

// ClientIP extracts the client IP from the request, respecting reverse proxy headers.
func ClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
