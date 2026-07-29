package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/agentstation/starmap/internal/server/handlers"
	"github.com/agentstation/starmap/internal/server/middleware"
	"github.com/agentstation/starmap/internal/server/openrouter"
)

// setupRouter creates the HTTP handler with routes and middleware.
func (s *Server) setupRouter() http.Handler {
	mux := http.NewServeMux()

	// Create handlers instance
	h := handlers.New(
		s.app,
		s.cache,
		s.sseBroadcaster,
		s.logger,
		s.startTime,
	)

	// Register routes
	s.registerRoutes(mux, h)

	// Apply middleware chain
	handler := s.applyMiddleware(mux)

	return handler
}

// registerRoutes registers all HTTP routes.
func (s *Server) registerRoutes(mux *http.ServeMux, h *handlers.Handlers) {
	prefix := s.config.PathPrefix

	// Favicon handler (return 204 No Content to avoid 404 logs)
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Public health endpoints (no auth required)
	mux.HandleFunc("/health", h.HandleHealth)
	mux.HandleFunc(prefix+"/health", h.HandleHealth)
	mux.HandleFunc(prefix+"/ready", h.HandleReady)

	s.registerModelRoutes(mux, h)

	// Providers endpoints
	mux.HandleFunc(prefix+"/providers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.HandleListProviders(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc(prefix+"/providers/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[len(prefix+"/providers/"):]
		parts := splitPath(path)

		if len(parts) == 0 {
			http.Error(w, "Provider ID required", http.StatusBadRequest)
			return
		}

		providerID := parts[0]

		if len(parts) == 1 {
			// GET /providers/{id}
			if r.Method == http.MethodGet {
				h.HandleGetProvider(w, r, providerID)
				return
			}
		} else if len(parts) == 2 && parts[1] == "models" {
			// GET /providers/{id}/models
			if r.Method == http.MethodGet {
				h.HandleGetProviderModels(w, r, providerID)
				return
			}
		}

		http.Error(w, "Not found", http.StatusNotFound)
	})

	// Admin endpoints
	mux.HandleFunc(prefix+"/catalog/manifest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.HandleCatalogManifest(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})
	mux.HandleFunc(prefix+"/catalog/generations/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, prefix+"/catalog/generations/")
		generationID, suffix, found := strings.Cut(path, "/")
		if r.Method == http.MethodGet && found && generationID != "" {
			switch suffix {
			case "manifest":
				h.HandleCatalogGenerationManifest(w, r, generationID)
				return
			case "payload":
				h.HandleCatalogPayload(w, r, generationID)
				return
			}
		}
		http.Error(w, "Not found", http.StatusNotFound)
	})

	if s.app.UpdatesEnabled() {
		mux.HandleFunc(prefix+"/update", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				h.HandleUpdate(w, r)
				return
			}
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		})
	}

	mux.HandleFunc(prefix+"/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.HandleStats(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// Real-time endpoints
	mux.HandleFunc(prefix+"/updates/stream", h.HandleSSE)

	// OpenAPI specification endpoints
	mux.HandleFunc(prefix+"/openapi.json", h.HandleOpenAPIJSON)
	mux.HandleFunc(prefix+"/openapi.yaml", h.HandleOpenAPIYAML)

	// Metrics endpoint (optional)
	if s.config.MetricsEnabled {
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprintf(w, "# Starmap API Metrics\n")
			_, _ = fmt.Fprintf(w, "# TYPE starmap_api_info gauge\n")
			_, _ = fmt.Fprintf(w, "starmap_api_info{version=\"v1\"} 1\n")
		})
	}
}

// applyMiddleware wraps handler with middleware chain.
func (s *Server) applyMiddleware(handler http.Handler) http.Handler {
	cfg := s.config

	// Rate limiting (if enabled)
	if cfg.RateLimit > 0 {
		rateLimiter := middleware.NewRateLimiter(cfg.RateLimit, s.logger)
		handler = middleware.RateLimit(rateLimiter)(handler)
	}

	// Authentication (if enabled)
	if cfg.AuthEnabled {
		authConfig := middleware.DefaultAuthConfig()
		authConfig.Enabled = true
		authConfig.HeaderName = cfg.AuthHeader
		authConfig.FailureOverride = func(
			w http.ResponseWriter,
			r *http.Request,
		) bool {
			if !openrouter.IsCompatibilityPath(r.URL.EscapedPath(), cfg.PathPrefix) {
				return false
			}
			openrouter.WriteError(
				w,
				http.StatusUnauthorized,
				"No auth credentials found",
			)
			return true
		}
		handler = middleware.Auth(authConfig, s.logger)(handler)
	}

	// CORS (if enabled)
	if cfg.CORSEnabled {
		corsConfig := middleware.DefaultCORSConfig()
		if len(cfg.CORSOrigins) > 0 {
			corsConfig.AllowedOrigins = cfg.CORSOrigins
			corsConfig.AllowAll = false
		} else {
			corsConfig.AllowAll = true
		}
		handler = middleware.CORS(corsConfig)(handler)
	}

	// Logging and recovery (always enabled)
	handler = middleware.Logger(s.logger)(handler)
	handler = middleware.Recovery(s.logger)(handler)

	return handler
}

func extractExactPathSegments(
	request *http.Request,
	prefix string,
	count int,
) ([]string, error) {
	escapedPath := request.URL.EscapedPath()
	escapedPrefix := (&url.URL{Path: prefix}).EscapedPath()
	if !strings.HasPrefix(escapedPath, escapedPrefix) {
		return nil, nil
	}
	rawParts := strings.Split(strings.TrimPrefix(escapedPath, escapedPrefix), "/")
	if len(rawParts) != count {
		return nil, nil
	}
	parts := make([]string, len(rawParts))
	for index, raw := range rawParts {
		value, err := url.PathUnescape(raw)
		if err != nil {
			return nil, err
		}
		if value == "" || value == "." || value == ".." ||
			strings.ContainsAny(value, `/\`) {
			return nil, fmt.Errorf("invalid path segment")
		}
		parts[index] = value
	}
	return parts, nil
}

// extractPathParam extracts path parameter from URL.
func extractPathParam(request *http.Request, prefix string) (string, error) {
	escapedPath := request.URL.EscapedPath()
	escapedPrefix := (&url.URL{Path: prefix}).EscapedPath()
	if !strings.HasPrefix(escapedPath, escapedPrefix) {
		return "", nil
	}
	value, err := url.PathUnescape(strings.TrimPrefix(escapedPath, escapedPrefix))
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(value, "/"), nil
}

// splitPath splits a URL path into parts, removing empty strings.
func splitPath(path string) []string {
	parts := []string{}
	for part := range strings.SplitSeq(path, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
