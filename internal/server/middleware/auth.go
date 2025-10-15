package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Enabled        bool
	APIKey         string
	HeaderName     string
	PublicPaths    []string
	BearerPrefix   bool
}

// DefaultAuthConfig returns default authentication configuration.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		Enabled:      false,
		APIKey:       os.Getenv("API_KEY"),
		HeaderName:   "X-API-Key",
		PublicPaths:  []string{"/health", "/api/v1/health", "/api/v1/ready", "/api/v1/openapi.json"},
		BearerPrefix: false,
	}
}

// Auth middleware validates API keys for protected endpoints.
func Auth(config AuthConfig, logger *zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication if disabled
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip authentication for public paths
			if isPublicPath(r.URL.Path, config.PublicPaths) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract API key from header
			apiKey := extractAPIKey(r, config)

			// Validate API key
			if apiKey == "" || apiKey != config.APIKey {
				logger.Warn().
					Str("path", r.URL.Path).
					Str("remote_addr", r.RemoteAddr).
					Bool("key_provided", apiKey != "").
					Msg("Authentication failed")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"data":null,"error":{"code":"UNAUTHORIZED","message":"Invalid or missing API key","details":"Provide a valid API key in the ` + config.HeaderName + ` header"}}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isPublicPath checks if a path is in the public paths list.
func isPublicPath(path string, publicPaths []string) bool {
	for _, p := range publicPaths {
		if path == p {
			return true
		}
	}
	return false
}

// extractAPIKey extracts the API key from the request.
func extractAPIKey(r *http.Request, config AuthConfig) string {
	// Try custom header first (X-API-Key)
	apiKey := r.Header.Get(config.HeaderName)
	if apiKey != "" {
		return apiKey
	}

	// Try Authorization header
	auth := r.Header.Get("Authorization")
	if auth != "" {
		// Support both "Bearer <key>" and raw key
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
		return auth
	}

	return ""
}
