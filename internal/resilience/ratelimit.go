package resilience

import (
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*limiterEntry

	limit rate.Limit
	burst int
	ttl   time.Duration
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPRateLimiter(limit rate.Limit, burst int, ttl time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		limiters: make(map[string]*limiterEntry),
		limit:    limit,
		burst:    burst,
		ttl:      ttl,
	}
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, ok := l.limiters[ip]
	if !ok {
		entry = &limiterEntry{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.limiters[ip] = entry
	}
	entry.lastSeen = now

	for key, e := range l.limiters {
		if now.Sub(e.lastSeen) > l.ttl {
			delete(l.limiters, key)
		}
	}

	return entry.limiter.Allow()
}

func RateLimitMiddleware(cfg RateLimitConfig, log *slog.Logger) func(http.Handler) http.Handler {
	if !cfg.Enabled {
		return func(next http.Handler) http.Handler { return next }
	}

	limit := rate.Every(cfg.Period / time.Duration(max(cfg.Requests, 1)))
	burst := cfg.Burst
	if burst <= 0 {
		burst = cfg.Requests
	}
	limiter := newIPRateLimiter(limit, burst, cfg.TTL)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !limiter.allow(ip) {
				log.Warn("rate limit exceeded",
					slog.String("ip", ip),
					slog.String("path", r.URL.Path),
				)
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
