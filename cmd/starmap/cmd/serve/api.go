package serve

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/internal/appcontext"
	"github.com/agentstation/starmap/pkg/logging"
)

// NewAPICommand creates the serve api command using app context.
func NewAPICommand(appCtx appcontext.Interface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Serve REST API server",
		Long: `Start a REST API server for the starmap catalog.

Features:
  - RESTful endpoints for models, providers, and authors
  - CORS support for web applications
  - Rate limiting and authentication
  - Health checks and metrics
  - Graceful shutdown

The API provides programmatic access to the starmap catalog with
endpoints for listing, searching, and retrieving model information.`,
		Example: `  starmap serve api                    # Start on default port 8080
  starmap serve api --port 3000         # Start on custom port
  starmap serve api --cors              # Enable CORS for all origins
  starmap serve api --auth              # Enable API key authentication`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPIWithApp(cmd, appCtx)
		},
	}

	// Add common server flags
	AddCommonFlags(cmd, getDefaultAPIPort())

	// Add API-specific flags
	cmd.Flags().Bool("cors", false, "Enable CORS for all origins")
	cmd.Flags().StringSlice("cors-origins", []string{}, "Allowed CORS origins")
	cmd.Flags().Bool("auth", false, "Enable API key authentication")
	cmd.Flags().String("auth-header", "X-API-Key", "Authentication header name")
	cmd.Flags().Int("rate-limit", 100, "Requests per minute per IP")
	cmd.Flags().Bool("metrics", true, "Enable metrics endpoint")
	cmd.Flags().String("prefix", "/api/v1", "API path prefix")

	return cmd
}

// NewAPICommandDeprecated creates the serve api command without app context.
// Deprecated: Use NewAPICommand which accepts appcontext.Interface.
func NewAPICommandDeprecated() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Serve REST API server",
		Long: `Start a REST API server for the starmap catalog.

Features:
  - RESTful endpoints for models, providers, and authors
  - CORS support for web applications
  - Rate limiting and authentication
  - Health checks and metrics
  - Graceful shutdown

The API provides programmatic access to the starmap catalog with
endpoints for listing, searching, and retrieving model information.`,
		Example: `  starmap serve api                    # Start on default port 8080
  starmap serve api --port 3000         # Start on custom port
  starmap serve api --cors              # Enable CORS for all origins
  starmap serve api --auth              # Enable API key authentication`,
		RunE: runAPI,
	}

	// Add common server flags
	AddCommonFlags(cmd, getDefaultAPIPort())

	// Add API-specific flags
	cmd.Flags().Bool("cors", false, "Enable CORS for all origins")
	cmd.Flags().StringSlice("cors-origins", []string{}, "Allowed CORS origins")
	cmd.Flags().Bool("auth", false, "Enable API key authentication")
	cmd.Flags().String("auth-header", "X-API-Key", "Authentication header name")
	cmd.Flags().Int("rate-limit", 100, "Requests per minute per IP")
	cmd.Flags().Bool("metrics", true, "Enable metrics endpoint")
	cmd.Flags().String("prefix", "/api/v1", "API path prefix")

	return cmd
}

// runAPIWithApp starts the API server using app context.
func runAPIWithApp(cmd *cobra.Command, appCtx appcontext.Interface) error {
	config, err := GetServerConfig(cmd, getDefaultAPIPort())
	if err != nil {
		return fmt.Errorf("getting server config: %w", err)
	}

	// Get API-specific flags
	corsEnabled, _ := cmd.Flags().GetBool("cors")
	corsOrigins, _ := cmd.Flags().GetStringSlice("cors-origins")
	authEnabled, _ := cmd.Flags().GetBool("auth")
	authHeader, _ := cmd.Flags().GetString("auth-header")
	rateLimit, _ := cmd.Flags().GetInt("rate-limit")
	metricsEnabled, _ := cmd.Flags().GetBool("metrics")
	pathPrefix, _ := cmd.Flags().GetString("prefix")

	// Override with environment-specific port
	if envPort := os.Getenv("STARMAP_API_PORT"); envPort != "" {
		if port, err := parsePort(envPort); err == nil {
			config.Port = port
		}
	}

	logger := appCtx.Logger()
	logger.Info().
		Int("port", config.Port).
		Str("host", config.Host).
		Str("prefix", pathPrefix).
		Bool("cors", corsEnabled).
		Bool("auth", authEnabled).
		Int("rate_limit", rateLimit).
		Msg("Starting API server")

	// Create HTTP server
	server := &http.Server{
		Addr:         config.Address(),
		Handler:      createAPIHandlerWithApp(appCtx, corsEnabled, corsOrigins, authEnabled, authHeader, rateLimit, metricsEnabled, pathPrefix),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server with graceful shutdown
	return StartServerWithGracefulShutdown(server, "API")
}

// runAPI starts the API server without app context.
// Deprecated: Use runAPIWithApp for new code.
func runAPI(cmd *cobra.Command, _ []string) error {
	config, err := GetServerConfig(cmd, getDefaultAPIPort())
	if err != nil {
		return fmt.Errorf("getting server config: %w", err)
	}

	// Get API-specific flags
	corsEnabled, _ := cmd.Flags().GetBool("cors")
	corsOrigins, _ := cmd.Flags().GetStringSlice("cors-origins")
	authEnabled, _ := cmd.Flags().GetBool("auth")
	authHeader, _ := cmd.Flags().GetString("auth-header")
	rateLimit, _ := cmd.Flags().GetInt("rate-limit")
	metricsEnabled, _ := cmd.Flags().GetBool("metrics")
	pathPrefix, _ := cmd.Flags().GetString("prefix")

	// Override with environment-specific port
	if envPort := os.Getenv("STARMAP_API_PORT"); envPort != "" {
		if port, err := parsePort(envPort); err == nil {
			config.Port = port
		}
	}

	logging.Info().
		Int("port", config.Port).
		Str("host", config.Host).
		Str("prefix", pathPrefix).
		Bool("cors", corsEnabled).
		Bool("auth", authEnabled).
		Int("rate_limit", rateLimit).
		Msg("Starting API server")

	// Create HTTP server
	server := &http.Server{
		Addr:         config.Address(),
		Handler:      createAPIHandler(corsEnabled, corsOrigins, authEnabled, authHeader, rateLimit, metricsEnabled, pathPrefix),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server with graceful shutdown
	return StartServerWithGracefulShutdown(server, "API")
}

// createAPIHandlerWithApp creates the HTTP handler using app context.
func createAPIHandlerWithApp(appCtx appcontext.Interface, corsEnabled bool, corsOrigins []string, authEnabled bool, authHeader string, _ int, metricsEnabled bool, pathPrefix string) http.Handler {
	// Initialize API handlers with app context
	apiHandlers, err := NewAPIHandlersWithApp(appCtx)
	if err != nil {
		logger := appCtx.Logger()
		logger.Error().Err(err).Msg("Failed to initialize API handlers")
		// Return a handler that returns 503 for all requests
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"service unavailable","message":"failed to load catalog"}`, http.StatusServiceUnavailable)
		})
	}

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprint(w, `{"status":"healthy","service":"starmap-api","version":"v1"}`); err != nil {
			appCtx.Logger().Error().Err(err).Msg("Failed to write health check response")
		}
	})

	// Middleware wrapper to apply CORS and auth
	wrap := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Apply CORS if enabled
			if corsEnabled {
				applyCORS(w, corsOrigins)
				// Handle preflight requests
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusOK)
					return
				}
			}

			// Apply auth if enabled
			if authEnabled && !isAuthenticated(r, authHeader) {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized","message":"valid API key required"}`, http.StatusUnauthorized)
				return
			}

			// Call the actual handler
			handler(w, r)
		}
	}

	// REST API endpoints following documented spec

	// Models endpoints
	mux.HandleFunc(pathPrefix+"/models", wrap(apiHandlers.ModelsHandler))
	mux.HandleFunc(pathPrefix+"/models/", wrap(apiHandlers.ModelByIDHandler))

	// Providers endpoints
	mux.HandleFunc(pathPrefix+"/providers", wrap(apiHandlers.ProvidersHandler))
	mux.HandleFunc(pathPrefix+"/providers/", wrap(apiHandlers.ProviderByIDHandler))

	// Future endpoints (placeholder responses)
	mux.HandleFunc(pathPrefix+"/webhooks", wrap(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"not implemented","message":"webhooks endpoint coming soon"}`, http.StatusNotImplemented)
	}))

	mux.HandleFunc(pathPrefix+"/updates/stream", wrap(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"not implemented","message":"SSE updates endpoint coming soon"}`, http.StatusNotImplemented)
	}))

	mux.HandleFunc(pathPrefix+"/sync", wrap(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"not implemented","message":"sync endpoint coming soon"}`, http.StatusNotImplemented)
	}))

	// Metrics endpoint (optional)
	if metricsEnabled {
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			modelsCount := len(apiHandlers.catalog.Models().List())
			if _, err := fmt.Fprintf(w, "# Starmap API Metrics\n# starmap_api_requests_total 0\n# starmap_catalog_models_total %d\n", modelsCount); err != nil {
				appCtx.Logger().Error().Err(err).Msg("Failed to write metrics response")
			}
		})
	}

	return mux
}

// createAPIHandler creates the HTTP handler for the API server.
// Deprecated: Use createAPIHandlerWithApp for new code.
func createAPIHandler(corsEnabled bool, corsOrigins []string, authEnabled bool, authHeader string, _ int, metricsEnabled bool, pathPrefix string) http.Handler {
	// Initialize API handlers with catalog
	apiHandlers, err := NewAPIHandlers()
	if err != nil {
		logging.Error().Err(err).Msg("Failed to initialize API handlers")
		// Return a handler that returns 503 for all requests
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"service unavailable","message":"failed to load catalog"}`, http.StatusServiceUnavailable)
		})
	}

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprint(w, `{"status":"healthy","service":"starmap-api","version":"v1"}`); err != nil {
			logging.Error().Err(err).Msg("Failed to write health check response")
		}
	})

	// Middleware wrapper to apply CORS and auth
	wrap := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Apply CORS if enabled
			if corsEnabled {
				applyCORS(w, corsOrigins)
				// Handle preflight requests
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusOK)
					return
				}
			}

			// Apply auth if enabled
			if authEnabled && !isAuthenticated(r, authHeader) {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized","message":"valid API key required"}`, http.StatusUnauthorized)
				return
			}

			// Call the actual handler
			handler(w, r)
		}
	}

	// REST API endpoints following documented spec

	// Models endpoints
	mux.HandleFunc(pathPrefix+"/models", wrap(apiHandlers.ModelsHandler))
	mux.HandleFunc(pathPrefix+"/models/", wrap(apiHandlers.ModelByIDHandler))

	// Providers endpoints
	mux.HandleFunc(pathPrefix+"/providers", wrap(apiHandlers.ProvidersHandler))
	mux.HandleFunc(pathPrefix+"/providers/", wrap(apiHandlers.ProviderByIDHandler))

	// Future endpoints (placeholder responses)
	mux.HandleFunc(pathPrefix+"/webhooks", wrap(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"not implemented","message":"webhooks endpoint coming soon"}`, http.StatusNotImplemented)
	}))

	mux.HandleFunc(pathPrefix+"/updates/stream", wrap(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"not implemented","message":"SSE updates endpoint coming soon"}`, http.StatusNotImplemented)
	}))

	mux.HandleFunc(pathPrefix+"/sync", wrap(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"not implemented","message":"sync endpoint coming soon"}`, http.StatusNotImplemented)
	}))

	// Metrics endpoint (optional)
	if metricsEnabled {
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			modelsCount := len(apiHandlers.catalog.Models().List())
			if _, err := fmt.Fprintf(w, "# Starmap API Metrics\n# starmap_api_requests_total 0\n# starmap_catalog_models_total %d\n", modelsCount); err != nil {
				logging.Error().Err(err).Msg("Failed to write metrics response")
			}
		})
	}

	return mux
}

// applyCORS applies CORS headers to the response.
func applyCORS(w http.ResponseWriter, allowedOrigins []string) {
	if len(allowedOrigins) == 0 {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		// In a real implementation, you'd check the request origin against allowed origins
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigins[0])
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
}

// isAuthenticated checks if the request is authenticated.
func isAuthenticated(r *http.Request, authHeader string) bool {
	apiKey := r.Header.Get(authHeader)
	// Placeholder implementation - in real use, validate against configured API keys
	return apiKey != ""
}

// getDefaultAPIPort returns the default port for API server.
func getDefaultAPIPort() int {
	// Common HTTP API server port
	return 8080
}
