package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// TestChain tests middleware composition.
func TestChain(t *testing.T) {
	tests := []struct {
		name              string
		numMiddleware     int
		expectedCallOrder []string
	}{
		{
			name:              "no middleware",
			numMiddleware:     0,
			expectedCallOrder: []string{"handler"},
		},
		{
			name:              "single middleware",
			numMiddleware:     1,
			expectedCallOrder: []string{"m1", "handler"},
		},
		{
			name:              "two middleware",
			numMiddleware:     2,
			expectedCallOrder: []string{"m1", "m2", "handler"},
		},
		{
			name:              "three middleware",
			numMiddleware:     3,
			expectedCallOrder: []string{"m1", "m2", "m3", "handler"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callOrder []string

			// Create middleware that track call order
			middlewares := make([]func(http.Handler) http.Handler, tt.numMiddleware)
			for i := 0; i < tt.numMiddleware; i++ {
				name := "m" + string(rune('1'+i))
				middlewares[i] = func(n string) func(http.Handler) http.Handler {
					return func(next http.Handler) http.Handler {
						return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							callOrder = append(callOrder, n)
							next.ServeHTTP(w, r)
						})
					}
				}(name)
			}

			// Create handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callOrder = append(callOrder, "handler")
				w.WriteHeader(http.StatusOK)
			})

			// Chain middleware
			chained := Chain(middlewares...)(handler)

			// Execute request
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			chained.ServeHTTP(w, req)

			// Verify call order
			if len(callOrder) != len(tt.expectedCallOrder) {
				t.Fatalf("expected %d calls, got %d", len(tt.expectedCallOrder), len(callOrder))
			}

			for i, expected := range tt.expectedCallOrder {
				if callOrder[i] != expected {
					t.Errorf("call %d: expected %s, got %s", i, expected, callOrder[i])
				}
			}

			// Verify response
			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}
		})
	}
}

// TestChain_ExecutionOrder verifies first added is outermost middleware.
func TestChain_ExecutionOrder(t *testing.T) {
	var executionLog []string

	// Middleware 1: Adds "start-1" before and "end-1" after
	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionLog = append(executionLog, "start-1")
			next.ServeHTTP(w, r)
			executionLog = append(executionLog, "end-1")
		})
	}

	// Middleware 2: Adds "start-2" before and "end-2" after
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executionLog = append(executionLog, "start-2")
			next.ServeHTTP(w, r)
			executionLog = append(executionLog, "end-2")
		})
	}

	// Handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executionLog = append(executionLog, "handler")
		w.WriteHeader(http.StatusOK)
	})

	// Chain: m1 first, then m2
	chained := Chain(m1, m2)(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	chained.ServeHTTP(w, req)

	// Expected order: start-1 → start-2 → handler → end-2 → end-1
	expected := []string{"start-1", "start-2", "handler", "end-2", "end-1"}
	if len(executionLog) != len(expected) {
		t.Fatalf("expected %d log entries, got %d", len(expected), len(executionLog))
	}

	for i, exp := range expected {
		if executionLog[i] != exp {
			t.Errorf("log[%d]: expected %s, got %s", i, exp, executionLog[i])
		}
	}
}

// TestLogger tests request logging middleware.
func TestLogger(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		handlerStatus  int
		expectLogEntry bool
	}{
		{
			name:           "GET request",
			method:         "GET",
			path:           "/api/v1/models",
			handlerStatus:  http.StatusOK,
			expectLogEntry: true,
		},
		{
			name:           "POST request",
			method:         "POST",
			path:           "/api/v1/sync",
			handlerStatus:  http.StatusCreated,
			expectLogEntry: true,
		},
		{
			name:           "DELETE request",
			method:         "DELETE",
			path:           "/api/v1/cache",
			handlerStatus:  http.StatusNoContent,
			expectLogEntry: true,
		},
		{
			name:           "error status",
			method:         "GET",
			path:           "/api/v1/unknown",
			handlerStatus:  http.StatusNotFound,
			expectLogEntry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger that writes to buffer
			var buf bytes.Buffer
			logger := zerolog.New(&buf).With().Timestamp().Logger()

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.handlerStatus)
			})

			// Wrap with logger middleware
			middleware := Logger(&logger)
			wrapped := middleware(handler)

			// Execute request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.RemoteAddr = "192.168.1.1:12345"
			req.Header.Set("User-Agent", "test-agent")
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			// Verify response status
			if w.Code != tt.handlerStatus {
				t.Errorf("expected status %d, got %d", tt.handlerStatus, w.Code)
			}

			// Parse log output
			logOutput := buf.String()
			if !tt.expectLogEntry {
				if logOutput != "" {
					t.Error("expected no log output")
				}
				return
			}

			// Verify log contains expected fields
			if !strings.Contains(logOutput, tt.method) {
				t.Errorf("log missing method %s: %s", tt.method, logOutput)
			}
			if !strings.Contains(logOutput, tt.path) {
				t.Errorf("log missing path %s: %s", tt.path, logOutput)
			}
			if !strings.Contains(logOutput, "192.168.1.1:12345") {
				t.Errorf("log missing remote_addr: %s", logOutput)
			}
			if !strings.Contains(logOutput, "HTTP request") {
				t.Errorf("log missing message: %s", logOutput)
			}

			// Verify log is valid JSON
			var logEntry map[string]interface{}
			if err := json.Unmarshal([]byte(logOutput), &logEntry); err != nil {
				t.Errorf("log is not valid JSON: %v", err)
			}

			// Verify required fields in JSON
			if logEntry["method"] != tt.method {
				t.Errorf("log method: expected %s, got %v", tt.method, logEntry["method"])
			}
			if logEntry["path"] != tt.path {
				t.Errorf("log path: expected %s, got %v", tt.path, logEntry["path"])
			}
			if statusFloat, ok := logEntry["status"].(float64); !ok || int(statusFloat) != tt.handlerStatus {
				t.Errorf("log status: expected %d, got %v", tt.handlerStatus, logEntry["status"])
			}
			if _, ok := logEntry["duration_ms"]; !ok {
				t.Error("log missing duration_ms field")
			}
		})
	}
}

// TestLogger_StatusCodeCapture verifies responseWriter captures status codes.
func TestLogger_StatusCodeCapture(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	statusCodes := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusNotFound,
		http.StatusInternalServerError,
	}

	for _, expectedStatus := range statusCodes {
		t.Run("status_"+http.StatusText(expectedStatus), func(t *testing.T) {
			buf.Reset()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(expectedStatus)
			})

			middleware := Logger(&logger)
			wrapped := middleware(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			// Parse log and verify status
			var logEntry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
				t.Fatalf("failed to parse log: %v", err)
			}

			statusFloat, ok := logEntry["status"].(float64)
			if !ok {
				t.Fatalf("status field not found or wrong type")
			}

			if int(statusFloat) != expectedStatus {
				t.Errorf("expected status %d, got %d", expectedStatus, int(statusFloat))
			}
		})
	}
}

// TestLogger_Duration verifies duration logging.
func TestLogger_Duration(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	// Handler that sleeps
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	middleware := Logger(&logger)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	wrapped.ServeHTTP(w, req)
	elapsed := time.Since(start)

	// Parse log
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	// Verify duration is present and reasonable
	durationFloat, ok := logEntry["duration_ms"].(float64)
	if !ok {
		t.Fatal("duration_ms field not found or wrong type")
	}

	// Duration is logged in milliseconds as a float
	durationMs := time.Duration(durationFloat * float64(time.Millisecond))

	// Duration should be at least 50ms (sleep time)
	if durationMs < 50*time.Millisecond {
		t.Errorf("duration too short: %v (expected >= 50ms)", durationMs)
	}

	// Duration should be close to actual elapsed time (within 100ms)
	diff := elapsed - durationMs
	if diff < 0 {
		diff = -diff
	}
	if diff > 100*time.Millisecond {
		t.Errorf("duration mismatch: logged %v, actual %v (diff %v)", durationMs, elapsed, diff)
	}
}

// TestRecovery tests panic recovery middleware.
func TestRecovery(t *testing.T) {
	tests := []struct {
		name          string
		shouldPanic   bool
		panicValue    interface{}
		expectStatus  int
		expectLogPanic bool
	}{
		{
			name:          "no panic - normal execution",
			shouldPanic:   false,
			expectStatus:  http.StatusOK,
			expectLogPanic: false,
		},
		{
			name:          "panic with string",
			shouldPanic:   true,
			panicValue:    "something went wrong",
			expectStatus:  http.StatusInternalServerError,
			expectLogPanic: true,
		},
		{
			name:          "panic with error",
			shouldPanic:   true,
			panicValue:    http.ErrAbortHandler,
			expectStatus:  http.StatusInternalServerError,
			expectLogPanic: true,
		},
		{
			name:          "panic with nil",
			shouldPanic:   true,
			panicValue:    nil,
			expectStatus:  http.StatusInternalServerError,
			expectLogPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger that writes to buffer
			var buf bytes.Buffer
			logger := zerolog.New(&buf).With().Timestamp().Logger()

			// Create handler that may panic
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.shouldPanic {
					panic(tt.panicValue)
				}
				w.WriteHeader(http.StatusOK)
			})

			// Wrap with recovery middleware
			middleware := Recovery(&logger)
			wrapped := middleware(handler)

			// Execute request
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			// Should not panic at this level
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panic not recovered: %v", r)
					}
				}()
				wrapped.ServeHTTP(w, req)
			}()

			// Verify response status
			if w.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, w.Code)
			}

			// Verify log output
			logOutput := buf.String()
			if tt.expectLogPanic {
				if !strings.Contains(logOutput, "Panic recovered") {
					t.Error("expected panic log entry")
				}
				if !strings.Contains(logOutput, "GET") {
					t.Error("log missing method")
				}
				if !strings.Contains(logOutput, "/test") {
					t.Error("log missing path")
				}

				// Verify error response JSON
				contentType := w.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("expected Content-Type=application/json, got %s", contentType)
				}

				body := w.Body.String()
				if !strings.Contains(body, "INTERNAL_ERROR") {
					t.Error("response missing INTERNAL_ERROR code")
				}
				if !strings.Contains(body, "Internal server error") {
					t.Error("response missing error message")
				}

				// Verify valid JSON
				var errorResp map[string]interface{}
				if err := json.Unmarshal([]byte(body), &errorResp); err != nil {
					t.Errorf("response is not valid JSON: %v", err)
				}
			} else {
				if strings.Contains(logOutput, "Panic recovered") {
					t.Error("unexpected panic log entry")
				}
			}
		})
	}
}

// TestRecovery_OtherRequestsStillWork verifies other requests work after panic.
func TestRecovery_OtherRequestsStillWork(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	requestCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 2 {
			panic("intentional panic")
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := Recovery(&logger)
	wrapped := middleware(handler)

	// Request 1: Success
	req1 := httptest.NewRequest("GET", "/test1", nil)
	w1 := httptest.NewRecorder()
	wrapped.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("request 1: expected 200, got %d", w1.Code)
	}

	// Request 2: Panic
	req2 := httptest.NewRequest("GET", "/test2", nil)
	w2 := httptest.NewRecorder()
	wrapped.ServeHTTP(w2, req2)
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("request 2: expected 500, got %d", w2.Code)
	}

	// Request 3: Should still work
	req3 := httptest.NewRequest("GET", "/test3", nil)
	w3 := httptest.NewRecorder()
	wrapped.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("request 3: expected 200, got %d", w3.Code)
	}

	if requestCount != 3 {
		t.Errorf("expected 3 requests, got %d", requestCount)
	}
}

// TestResponseWriter tests the responseWriter wrapper.
func TestResponseWriter(t *testing.T) {
	tests := []struct {
		name         string
		writeHeader  bool
		statusCode   int
		expectedCode int
	}{
		{
			name:         "explicit WriteHeader",
			writeHeader:  true,
			statusCode:   http.StatusCreated,
			expectedCode: http.StatusCreated,
		},
		{
			name:         "default status (no WriteHeader)",
			writeHeader:  false,
			expectedCode: http.StatusOK,
		},
		{
			name:         "error status",
			writeHeader:  true,
			statusCode:   http.StatusBadRequest,
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			rw := &responseWriter{
				ResponseWriter: recorder,
				statusCode:     http.StatusOK,
			}

			if tt.writeHeader {
				rw.WriteHeader(tt.statusCode)
			}

			// Verify wrapped status code
			if rw.statusCode != tt.expectedCode {
				t.Errorf("expected statusCode=%d, got %d", tt.expectedCode, rw.statusCode)
			}

			// Verify underlying recorder
			if tt.writeHeader && recorder.Code != tt.statusCode {
				t.Errorf("expected recorder.Code=%d, got %d", tt.statusCode, recorder.Code)
			}
		})
	}
}
