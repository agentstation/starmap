// Package middleware provides HTTP middleware for the Starmap API server.
// It includes logging, recovery, CORS, authentication, and rate limiting.
package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// Chain combines multiple middleware functions into a single middleware.
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Logger logs HTTP requests with structured logging.
func Logger(logger *zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Add logger to request context
			ctx := logger.With().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Logger().
				WithContext(r.Context())

			// Process request
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Log request completion
			duration := time.Since(start)
			logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapped.statusCode).
				Dur("duration_ms", duration).
				Str("remote_addr", r.RemoteAddr).
				Msg("HTTP request")
		})
	}
}

// Recovery recovers from panics and returns 500 error.
func Recovery(logger *zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error().
						Interface("panic", err).
						Str("method", r.Method).
						Str("path", r.URL.Path).
						Msg("Panic recovered")

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					// Write error response; if this fails, connection is likely broken
					if _, writeErr := w.Write([]byte(`{"data":null,"error":{"code":"INTERNAL_ERROR","message":"Internal server error","details":"An unexpected error occurred"}}`)); writeErr != nil {
						logger.Error().Err(writeErr).Msg("Failed to write panic recovery error response")
					}
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
