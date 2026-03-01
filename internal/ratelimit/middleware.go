package ratelimit

import "net/http"

// Middleware wraps an http.Handler and rejects requests that exceed the rate limit
// with a 429 status code.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(ClientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"code":"rate_limited","message":"Too many requests"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
