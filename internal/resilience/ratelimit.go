package resilience

import (
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimitConfig struct {
	Enabled bool
	RPS     int
	Burst   int
	TTL     time.Duration
}

type IPRateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*limiterEntry

	limit rate.Limit
	burst int
	ttl   time.Duration

	enabled bool

	done chan struct{}
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewIPRateLimiter(cfg RateLimitConfig) *IPRateLimiter {
	rl := &IPRateLimiter{
		limiters: make(map[string]*limiterEntry),
		limit:    rate.Limit(cfg.RPS),
		burst:    cfg.Burst,
		ttl:      cfg.TTL,
		enabled:  cfg.Enabled,
		done:     make(chan struct{}),
	}

	if rl.enabled {
		go rl.cleaner(rl.ttl)
	}

	return rl
}

func (rl *IPRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.limiters[ip]
	if !ok {
		entry = &limiterEntry{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.limiters[ip] = entry
	}
	entry.lastSeen = now

	return entry.limiter.Allow()
}

func (rl *IPRateLimiter) cleaner(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	select {
	case <-ticker.C:
		now := time.Now()

		rl.mu.Lock()
		for key, e := range rl.limiters {
			if now.Sub(e.lastSeen) > rl.ttl {
				delete(rl.limiters, key)
			}
		}

		rl.mu.Unlock()
	case <-rl.done:
		return
	}
}

func (rl *IPRateLimiter) Middleware(next http.HandlerFunc, log *slog.Logger) http.HandlerFunc {
	if !rl.enabled {
		return next
	}

	return func(w http.ResponseWriter, req *http.Request) {
		ip := clientIP(req)
		if !rl.allow(ip) {
			log.Warn("rate limit exceeded",
				slog.String("ip", ip),
				slog.String("path", req.URL.Path),
			)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, req)
	}
}

func clientIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

func (rl *IPRateLimiter) Stop() {
	if rl.done != nil {
		close(rl.done)
		rl.done = nil
	}
}
