package serve

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/server/middleware"
)

// NewAPICommand creates the enhanced serve api command.
func NewAPICommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Serve REST API server with WebSocket and SSE support",
		Long: `Start a production-ready REST API server for the starmap catalog.

Features:
  - RESTful endpoints for models, providers, and catalog management
  - WebSocket support for real-time updates (/api/v1/updates/ws)
  - Server-Sent Events (SSE) for streaming updates (/api/v1/updates/stream)
  - In-memory caching with configurable TTL
  - Rate limiting (requests per minute per IP)
  - API key authentication (optional)
  - CORS support for web applications
  - Request logging and panic recovery
  - Graceful shutdown with connection draining
  - Health checks and metrics endpoints
  - OpenAPI 3.0 documentation

The API provides programmatic access to the starmap catalog with
comprehensive filtering, search, and real-time notification capabilities.`,
		Example: `  # Start on default port 8080
  starmap serve api

  # Start on custom port with authentication
  starmap serve api --port 3000 --auth

  # Enable CORS for specific origins
  starmap serve api --cors-origins "https://example.com,https://app.example.com"

  # Enable rate limiting
  starmap serve api --rate-limit 60

  # Full configuration
  starmap serve api --port 8080 --cors --auth --rate-limit 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAPI(cmd, args, app)
		},
	}

	// Server configuration flags
	cmd.Flags().IntP("port", "p", 8080, "Server port")
	cmd.Flags().String("host", "localhost", "Bind address")

	// CORS flags
	cmd.Flags().Bool("cors", false, "Enable CORS for all origins")
	cmd.Flags().StringSlice("cors-origins", []string{}, "Allowed CORS origins (comma-separated)")

	// Authentication flags
	cmd.Flags().Bool("auth", false, "Enable API key authentication")
	cmd.Flags().String("auth-header", "X-API-Key", "Authentication header name")

	// Performance flags
	cmd.Flags().Int("rate-limit", 100, "Requests per minute per IP (0 to disable)")
	cmd.Flags().Int("cache-ttl", 300, "Cache TTL in seconds")

	// Timeout flags
	cmd.Flags().Duration("read-timeout", 10*time.Second, "HTTP read timeout")
	cmd.Flags().Duration("write-timeout", 10*time.Second, "HTTP write timeout")
	cmd.Flags().Duration("idle-timeout", 120*time.Second, "HTTP idle timeout")

	// Features flags
	cmd.Flags().Bool("metrics", true, "Enable metrics endpoint")
	cmd.Flags().String("prefix", "/api/v1", "API path prefix")

	return cmd
}

// runAPI starts the enhanced API server.
func runAPI(cmd *cobra.Command, _ []string, app application.Application) error {
	// Parse flags
	port, _ := cmd.Flags().GetInt("port")
	host, _ := cmd.Flags().GetString("host")
	corsEnabled, _ := cmd.Flags().GetBool("cors")
	corsOrigins, _ := cmd.Flags().GetStringSlice("cors-origins")
	authEnabled, _ := cmd.Flags().GetBool("auth")
	authHeader, _ := cmd.Flags().GetString("auth-header")
	rateLimit, _ := cmd.Flags().GetInt("rate-limit")
	cacheTTL, _ := cmd.Flags().GetInt("cache-ttl")
	readTimeout, _ := cmd.Flags().GetDuration("read-timeout")
	writeTimeout, _ := cmd.Flags().GetDuration("write-timeout")
	idleTimeout, _ := cmd.Flags().GetDuration("idle-timeout")
	metricsEnabled, _ := cmd.Flags().GetBool("metrics")
	pathPrefix, _ := cmd.Flags().GetString("prefix")

	// Override with environment variables
	if envPort := os.Getenv("HTTP_PORT"); envPort != "" {
		if p, err := parsePort(envPort); err == nil {
			port = p
		}
	}
	if envHost := os.Getenv("HTTP_HOST"); envHost != "" {
		host = envHost
	}

	logger := app.Logger()
	logger.Info().
		Int("port", port).
		Str("host", host).
		Str("prefix", pathPrefix).
		Bool("cors", corsEnabled).
		Bool("auth", authEnabled).
		Int("rate_limit", rateLimit).
		Int("cache_ttl_seconds", cacheTTL).
		Msg("Starting API server")

	// Create API server
	apiServer, err := NewAPIServer(app)
	if err != nil {
		return fmt.Errorf("creating API server: %w", err)
	}

	// Start background services
	apiServer.Start()

	// Create HTTP server with middleware
	handler := buildHandler(apiServer, app, ServerConfig{
		PathPrefix:     pathPrefix,
		CORSEnabled:    corsEnabled,
		CORSOrigins:    corsOrigins,
		AuthEnabled:    authEnabled,
		AuthHeader:     authHeader,
		RateLimit:      rateLimit,
		MetricsEnabled: metricsEnabled,
	})

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// Start server with graceful shutdown
	return startServerWithGracefulShutdown(server, "API", logger)
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	PathPrefix     string
	CORSEnabled    bool
	CORSOrigins    []string
	AuthEnabled    bool
	AuthHeader     string
	RateLimit      int
	MetricsEnabled bool
}

// buildHandler creates the HTTP handler with middleware chain.
func buildHandler(apiServer *APIServer, app application.Application, config ServerConfig) http.Handler {
	mux := http.NewServeMux()
	logger := app.Logger()

	// Public health endpoints (no auth required)
	mux.HandleFunc("/health", apiServer.HandleHealth)
	mux.HandleFunc(config.PathPrefix+"/health", apiServer.HandleHealth)
	mux.HandleFunc(config.PathPrefix+"/ready", apiServer.HandleReady)

	// Models endpoints
	mux.HandleFunc(config.PathPrefix+"/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// POST /api/v1/models is treated as search
			if r.URL.Path == config.PathPrefix+"/models" || r.URL.Path == config.PathPrefix+"/models/" {
				apiServer.HandleSearchModels(w, r)
				return
			}
		}

		if r.Method == http.MethodGet {
			apiServer.HandleListModels(w, r)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc(config.PathPrefix+"/models/", func(w http.ResponseWriter, r *http.Request) {
		modelID := extractPathParam(r.URL.Path, config.PathPrefix+"/models/")
		if modelID != "" && r.Method == http.MethodGet {
			apiServer.HandleGetModel(w, r, modelID)
			return
		}
		http.Error(w, "Not found", http.StatusNotFound)
	})

	// Providers endpoints
	mux.HandleFunc(config.PathPrefix+"/providers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			apiServer.HandleListProviders(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc(config.PathPrefix+"/providers/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[len(config.PathPrefix+"/providers/"):]
		parts := splitPath(path)

		if len(parts) == 0 {
			http.Error(w, "Provider ID required", http.StatusBadRequest)
			return
		}

		providerID := parts[0]

		if len(parts) == 1 {
			// GET /providers/{id}
			if r.Method == http.MethodGet {
				apiServer.HandleGetProvider(w, r, providerID)
				return
			}
		} else if len(parts) == 2 && parts[1] == "models" {
			// GET /providers/{id}/models
			if r.Method == http.MethodGet {
				apiServer.HandleGetProviderModels(w, r, providerID)
				return
			}
		}

		http.Error(w, "Not found", http.StatusNotFound)
	})

	// Admin endpoints
	mux.HandleFunc(config.PathPrefix+"/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			apiServer.HandleUpdate(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc(config.PathPrefix+"/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			apiServer.HandleStats(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// Real-time endpoints
	mux.HandleFunc(config.PathPrefix+"/updates/ws", apiServer.HandleWebSocket)
	mux.HandleFunc(config.PathPrefix+"/updates/stream", apiServer.HandleSSE)

	// Metrics endpoint (optional)
	if config.MetricsEnabled {
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprintf(w, "# Starmap API Metrics\n")
			_, _ = fmt.Fprintf(w, "# TYPE starmap_api_info gauge\n")
			_, _ = fmt.Fprintf(w, "starmap_api_info{version=\"v1\"} 1\n")
		})
	}

	// Build middleware chain
	var handler http.Handler = mux

	// Rate limiting (if enabled)
	if config.RateLimit > 0 {
		rateLimiter := middleware.NewRateLimiter(config.RateLimit, logger)
		handler = middleware.RateLimit(rateLimiter)(handler)
	}

	// Authentication (if enabled)
	if config.AuthEnabled {
		authConfig := middleware.DefaultAuthConfig()
		authConfig.Enabled = true
		authConfig.HeaderName = config.AuthHeader
		handler = middleware.Auth(authConfig, logger)(handler)
	}

	// CORS (if enabled)
	if config.CORSEnabled {
		corsConfig := middleware.DefaultCORSConfig()
		if len(config.CORSOrigins) > 0 {
			corsConfig.AllowedOrigins = config.CORSOrigins
			corsConfig.AllowAll = false
		} else {
			corsConfig.AllowAll = true
		}
		handler = middleware.CORS(corsConfig)(handler)
	}

	// Logging and recovery (always enabled)
	handler = middleware.Logger(logger)(handler)
	handler = middleware.Recovery(logger)(handler)

	return handler
}

// startServerWithGracefulShutdown starts the server with graceful shutdown.
func startServerWithGracefulShutdown(server *http.Server, serviceName string, logger *zerolog.Logger) error {
	// Server errors channel
	serverErr := make(chan error, 1)

	// Start server in goroutine
	go func() {
		logger.Info().
			Str("addr", server.Addr).
			Str("service", serviceName).
			Msg("Server starting")

		fmt.Printf("🚀 Starting %s server on %s\n", serviceName, server.Addr)
		fmt.Println("   Press Ctrl+C to stop")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed: %w", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case sig := <-quit:
		logger.Info().
			Str("signal", sig.String()).
			Msg("Shutdown signal received")

		fmt.Printf("\n🛑 Shutting down %s server...\n", serviceName)

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Shutdown server
		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}

		logger.Info().Msg("Server stopped gracefully")
		fmt.Printf("✅ %s server stopped gracefully\n", serviceName)
		return nil
	}
}

// splitPath splits a URL path into parts, removing empty strings.
func splitPath(path string) []string {
	parts := []string{}
	for _, part := range strings.Split(path, "/") {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
