package auth

import (
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	tokens   map[string]int
	max      int
	interval time.Duration
	done     chan struct{}
}

func newRateLimiter(max int, interval time.Duration) *rateLimiter {
	rl := &rateLimiter{
		tokens:   make(map[string]int),
		max:      max,
		interval: interval,
		done:     make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if len(rl.tokens) > 100000 {
		return false
	}
	count := rl.tokens[key]
	if count >= rl.max {
		return false
	}
	rl.tokens[key] = count + 1
	return true
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(rl.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			rl.tokens = make(map[string]int)
			rl.mu.Unlock()
		case <-rl.done:
			return
		}
	}
}

// RateLimitMiddleware returns middleware that limits requests per remote address.
func RateLimitMiddleware(max int, interval time.Duration) func(http.Handler) http.Handler {
	rl := newRateLimiter(max, interval)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(r.RemoteAddr) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
