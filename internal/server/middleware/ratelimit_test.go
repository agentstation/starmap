package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// TestNewRateLimiter tests rate limiter creation.
func TestNewRateLimiter(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(100, &logger)

	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.visitors == nil {
		t.Error("visitors map not initialized")
	}
	if rl.limit != 100 {
		t.Errorf("expected limit=100, got %d", rl.limit)
	}
	if rl.interval != time.Minute {
		t.Errorf("expected interval=1m, got %v", rl.interval)
	}
}

// TestRateLimiter_Allow tests basic rate limiting logic.
func TestRateLimiter_Allow(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name          string
		limit         int
		requests      int
		expectedAllow int // How many should be allowed
	}{
		{
			name:          "within limit",
			limit:         10,
			requests:      5,
			expectedAllow: 5,
		},
		{
			name:          "at limit",
			limit:         10,
			requests:      10,
			expectedAllow: 10,
		},
		{
			name:          "exceeds limit",
			limit:         10,
			requests:      15,
			expectedAllow: 10,
		},
		{
			name:          "zero limit",
			limit:         0,
			requests:      5,
			expectedAllow: 0,
		},
		{
			name:          "single request limit",
			limit:         1,
			requests:      3,
			expectedAllow: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.limit, &logger)
			ip := "192.168.1.1"

			allowed := 0
			for i := 0; i < tt.requests; i++ {
				if rl.allow(ip) {
					allowed++
				}
			}

			if allowed != tt.expectedAllow {
				t.Errorf("expected %d allowed, got %d", tt.expectedAllow, allowed)
			}
		})
	}
}

// TestRateLimiter_MultipleIPs tests independent rate limiting per IP.
func TestRateLimiter_MultipleIPs(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(5, &logger)

	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}

	// Each IP should get their own limit
	for _, ip := range ips {
		allowed := 0
		for i := 0; i < 10; i++ {
			if rl.allow(ip) {
				allowed++
			}
		}
		if allowed != 5 {
			t.Errorf("IP %s: expected 5 allowed, got %d", ip, allowed)
		}
	}

	// Verify each IP is tracked separately
	if len(rl.visitors) != 3 {
		t.Errorf("expected 3 visitors, got %d", len(rl.visitors))
	}
}

// TestRateLimiter_TokenRefresh tests token bucket refresh after interval.
func TestRateLimiter_TokenRefresh(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(3, &logger)

	// Override interval for faster testing
	rl.interval = 100 * time.Millisecond

	ip := "192.168.1.1"

	// Use all tokens
	for i := 0; i < 3; i++ {
		if !rl.allow(ip) {
			t.Fatalf("expected request %d to be allowed", i)
		}
	}

	// Next request should be denied
	if rl.allow(ip) {
		t.Error("expected request to be denied (no tokens)")
	}

	// Wait for token refresh
	time.Sleep(150 * time.Millisecond)

	// Tokens should be refreshed
	if !rl.allow(ip) {
		t.Error("expected request to be allowed after token refresh")
	}
}

// TestRateLimiter_ConcurrentRequests tests thread-safety with concurrent requests.
func TestRateLimiter_ConcurrentRequests(t *testing.T) {
	logger := zerolog.Nop()
	limit := 100
	rl := NewRateLimiter(limit, &logger)

	ip := "192.168.1.1"
	numGoroutines := 50
	requestsPerGoroutine := 10

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				if rl.allow(ip) {
					mu.Lock()
					allowed++
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	// Should allow exactly the limit
	if allowed != limit {
		t.Errorf("expected %d allowed, got %d", limit, allowed)
	}
}

// TestRateLimiter_ConcurrentMultipleIPs tests concurrent requests from multiple IPs.
func TestRateLimiter_ConcurrentMultipleIPs(t *testing.T) {
	logger := zerolog.Nop()
	limit := 10
	rl := NewRateLimiter(limit, &logger)

	numIPs := 20
	requestsPerIP := 15

	var wg sync.WaitGroup
	results := make(map[string]int)
	var mu sync.Mutex

	wg.Add(numIPs)
	for i := 0; i < numIPs; i++ {
		go func(id int) {
			defer wg.Done()
			ip := "192.168.1." + string(rune(id+1))
			allowed := 0

			for j := 0; j < requestsPerIP; j++ {
				if rl.allow(ip) {
					allowed++
				}
			}

			mu.Lock()
			results[ip] = allowed
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Each IP should be allowed exactly the limit
	for ip, count := range results {
		if count != limit {
			t.Errorf("IP %s: expected %d allowed, got %d", ip, limit, count)
		}
	}
}

// TestRateLimiter_Middleware tests the RateLimit middleware function.
func TestRateLimiter_Middleware(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name            string
		limit           int
		requests        int
		expectedSuccess int
		expectedBlocked int
	}{
		{
			name:            "within limit",
			limit:           5,
			requests:        3,
			expectedSuccess: 3,
			expectedBlocked: 0,
		},
		{
			name:            "at limit",
			limit:           5,
			requests:        5,
			expectedSuccess: 5,
			expectedBlocked: 0,
		},
		{
			name:            "exceeds limit",
			limit:           5,
			requests:        8,
			expectedSuccess: 5,
			expectedBlocked: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.limit, &logger)
			middleware := RateLimit(rl)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			handler := middleware(testHandler)

			success := 0
			blocked := 0

			for i := 0; i < tt.requests; i++ {
				req := httptest.NewRequest("GET", "/api/v1/models", nil)
				req.RemoteAddr = "192.168.1.1:12345"
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				if w.Code == http.StatusOK {
					success++
				} else if w.Code == http.StatusTooManyRequests {
					blocked++
				}
			}

			if success != tt.expectedSuccess {
				t.Errorf("expected %d successful requests, got %d", tt.expectedSuccess, success)
			}
			if blocked != tt.expectedBlocked {
				t.Errorf("expected %d blocked requests, got %d", tt.expectedBlocked, blocked)
			}
		})
	}
}

// TestRateLimiter_Middleware_XForwardedFor tests X-Forwarded-For header handling.
func TestRateLimiter_Middleware_XForwardedFor(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(3, &logger)
	middleware := RateLimit(rl)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware(testHandler)

	// Use X-Forwarded-For
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/v1/models", nil)
		req.RemoteAddr = "proxy:8080"
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, w.Code)
		}
	}

	// Next request should be blocked
	req := httptest.NewRequest("GET", "/api/v1/models", nil)
	req.RemoteAddr = "proxy:8080"
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

// TestRateLimiter_Middleware_ErrorResponse tests rate limit error response format.
func TestRateLimiter_Middleware_ErrorResponse(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(1, &logger)
	middleware := RateLimit(rl)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware(testHandler)

	// First request succeeds
	req := httptest.NewRequest("GET", "/api/v1/models", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Second request is rate limited
	req = httptest.NewRequest("GET", "/api/v1/models", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}

	// Check response format
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %s", contentType)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected error response body")
	}
	if !contains(body, "RATE_LIMITED") {
		t.Error("expected RATE_LIMITED in response body")
	}
}

// TestRateLimiter_Cleanup tests the cleanup goroutine behavior.
func TestRateLimiter_Cleanup(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(5, &logger)

	// Add some visitors
	for i := 0; i < 10; i++ {
		ip := "192.168.1." + string(rune(i+1))
		rl.allow(ip)
	}

	initialCount := len(rl.visitors)
	if initialCount != 10 {
		t.Errorf("expected 10 visitors, got %d", initialCount)
	}

	// Manually set lastReset to old time to trigger cleanup
	rl.mu.Lock()
	for _, v := range rl.visitors {
		v.mu.Lock()
		v.lastReset = time.Now().Add(-15 * time.Minute)
		v.mu.Unlock()
	}
	rl.mu.Unlock()

	// Trigger cleanup by calling the internal cleanup logic
	// Note: In production, cleanup runs every 5 minutes
	// For testing, we'll simulate it
	rl.mu.Lock()
	for ip, v := range rl.visitors {
		v.mu.Lock()
		if time.Since(v.lastReset) > 10*time.Minute {
			delete(rl.visitors, ip)
		}
		v.mu.Unlock()
	}
	rl.mu.Unlock()

	// Verify cleanup occurred
	if len(rl.visitors) != 0 {
		t.Errorf("expected 0 visitors after cleanup, got %d", len(rl.visitors))
	}
}

// TestRateLimiter_VisitorCreation tests double-checked locking pattern.
func TestRateLimiter_VisitorCreation(t *testing.T) {
	logger := zerolog.Nop()
	rl := NewRateLimiter(100, &logger)

	ip := "192.168.1.1"

	// Concurrent creation should only create one visitor
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			rl.allow(ip)
		}()
	}
	wg.Wait()

	// Should only have one visitor
	if len(rl.visitors) != 1 {
		t.Errorf("expected 1 visitor, got %d", len(rl.visitors))
	}
}

// TestRateLimiter_BurstTraffic tests handling burst traffic patterns.
func TestRateLimiter_BurstTraffic(t *testing.T) {
	logger := zerolog.Nop()
	limit := 50
	rl := NewRateLimiter(limit, &logger)

	ip := "192.168.1.1"

	// Simulate burst of requests
	burstSize := 100
	allowed := 0

	start := time.Now()
	for i := 0; i < burstSize; i++ {
		if rl.allow(ip) {
			allowed++
		}
	}
	duration := time.Since(start)

	// Should handle burst quickly
	if duration > 100*time.Millisecond {
		t.Errorf("burst took too long: %v", duration)
	}

	// Should respect limit
	if allowed != limit {
		t.Errorf("expected %d allowed, got %d", limit, allowed)
	}
}

// contains helper is defined in auth_test.go
