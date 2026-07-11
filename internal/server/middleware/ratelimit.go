package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// RateLimiter implements token bucket rate limiting per IP address.
type RateLimiter struct {
	mu          sync.RWMutex
	visitors    map[string]*visitor
	limit       int           // requests per minute
	interval    time.Duration // token reset interval
	lastCleanup time.Time
	logger      *zerolog.Logger
}

const (
	rateLimitCleanupInterval = 5 * time.Minute
	rateLimitVisitorMaxIdle  = 10 * time.Minute
)

// visitor tracks rate limit state for a single IP.
type visitor struct {
	tokens    int
	lastReset time.Time
	mu        sync.Mutex
}

// NewRateLimiter creates a new rate limiter.
// limit is requests per minute per IP.
func NewRateLimiter(limit int, logger *zerolog.Logger) *RateLimiter {
	rl := &RateLimiter{
		visitors:    make(map[string]*visitor),
		limit:       limit,
		interval:    time.Minute,
		lastCleanup: time.Now(),
		logger:      logger,
	}
	return rl
}

// cleanup removes stale visitors opportunistically on request traffic so the
// middleware owns no goroutine or shutdown lifecycle.
func (rl *RateLimiter) cleanup(now time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if now.Sub(rl.lastCleanup) < rateLimitCleanupInterval {
		return
	}
	for ip, v := range rl.visitors {
		v.mu.Lock()
		if now.Sub(v.lastReset) > rateLimitVisitorMaxIdle {
			delete(rl.visitors, ip)
		}
		v.mu.Unlock()
	}
	rl.lastCleanup = now
}

// getVisitor returns or creates a visitor for the IP.
func (rl *RateLimiter) getVisitor(ip string) *visitor {
	now := time.Now()
	rl.cleanup(now)
	rl.mu.RLock()
	v, exists := rl.visitors[ip]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		v, exists = rl.visitors[ip]
		if !exists {
			v = &visitor{
				tokens:    rl.limit,
				lastReset: now,
			}
			rl.visitors[ip] = v
		}
		rl.mu.Unlock()
	}

	return v
}

// allow checks if a request from the IP is allowed.
func (rl *RateLimiter) allow(ip string) bool {
	v := rl.getVisitor(ip)

	v.mu.Lock()
	defer v.mu.Unlock()

	// Reset tokens if interval has passed
	if time.Since(v.lastReset) > rl.interval {
		v.tokens = rl.limit
		v.lastReset = time.Now()
	}

	// Check if tokens available
	if v.tokens > 0 {
		v.tokens--
		return true
	}

	return false
}

// RateLimit middleware limits requests per IP address.
func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Trust only the socket peer. Forwarded headers are attacker-controlled
			// unless an explicitly configured trusted-proxy boundary validates them.
			ip := clientAddress(r.RemoteAddr)

			// Check rate limit
			if !rl.allow(ip) {
				rl.logger.Warn().
					Str("ip", ip).
					Str("path", r.URL.Path).
					Msg("Rate limit exceeded")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				// Write error response; if this fails, connection is likely broken
				if _, writeErr := w.Write([]byte(`{"data":null,"error":{"code":"RATE_LIMITED","message":"Rate limit exceeded","details":"Too many requests. Please try again later."}}`)); writeErr != nil {
					rl.logger.Error().Err(writeErr).Msg("Failed to write rate limit error response")
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientAddress(remoteAddress string) string {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err == nil && host != "" {
		return host
	}
	return remoteAddress
}
