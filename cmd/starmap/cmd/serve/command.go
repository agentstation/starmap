// Package serve provides HTTP server commands for the Starmap CLI.
package serve

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/emoji"
	"github.com/agentstation/starmap/internal/server"
)

// NewCommand creates the serve command using app context.
func NewCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "serve",
		Aliases: []string{"server"},
		Short:   "Start the REST API server with WebSocket and SSE support",
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
  - OpenAPI 3.1 documentation (/api/v1/openapi.json)

The API provides programmatic access to the starmap catalog with
comprehensive filtering, search, and real-time notification capabilities.`,
		Example: `  # Start on default port 8080
  starmap serve

  # Start on custom port with authentication
  starmap serve --port 3000 --auth

  # Enable CORS for specific origins
  starmap serve --cors-origins "https://example.com,https://app.example.com"

  # Enable rate limiting
  starmap serve --rate-limit 60

  # Full configuration
  starmap serve --port 8080 --cors --auth --rate-limit 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(cmd, args, app)
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

// runServer starts the API server.
func runServer(cmd *cobra.Command, _ []string, app application.Application) error {
	// Parse flags into configuration
	cfg := parseConfig(cmd)
	logger := app.Logger()

	logger.Debug().Msg("Parsed server configuration")

	logger.Info().
		Int("port", cfg.Port).
		Str("host", cfg.Host).
		Str("prefix", cfg.PathPrefix).
		Bool("cors", cfg.CORSEnabled).
		Bool("auth", cfg.AuthEnabled).
		Int("rate_limit", cfg.RateLimit).
		Dur("cache_ttl", cfg.CacheTTL).
		Msg("Starting API server")

	// Create server
	logger.Debug().Msg("Creating server instance")
	srv, err := server.New(app, cfg)
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}
	logger.Debug().Msg("Server instance created")

	// Start background services (WebSocket hub, SSE broadcaster, event broker)
	logger.Debug().Msg("Starting background services")
	srv.Start()
	logger.Debug().Msg("Background services started")

	// Log that server is starting (after background services initialize)
	logger.Info().
		Str("addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)).
		Str("service", "API").
		Msg("Server starting")

	// Create HTTP server
	logger.Debug().
		Str("addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)).
		Dur("read_timeout", cfg.ReadTimeout).
		Dur("write_timeout", cfg.WriteTimeout).
		Dur("idle_timeout", cfg.IdleTimeout).
		Msg("Creating HTTP server")

	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      srv.Handler(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Start HTTP server with graceful shutdown
	logger.Debug().Msg("Starting HTTP server listener with graceful shutdown handling")
	return startWithGracefulShutdown(httpServer, srv, logger)
}

// parseConfig parses command flags into server configuration.
func parseConfig(cmd *cobra.Command) server.Config {
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

	return server.Config{
		Host:           host,
		Port:           port,
		PathPrefix:     pathPrefix,
		CORSEnabled:    corsEnabled,
		CORSOrigins:    corsOrigins,
		AuthEnabled:    authEnabled,
		AuthHeader:     authHeader,
		RateLimit:      rateLimit,
		CacheTTL:       time.Duration(cacheTTL) * time.Second,
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		IdleTimeout:    idleTimeout,
		MetricsEnabled: metricsEnabled,
	}
}

// parsePort safely parses a port string to integer.
func parsePort(portStr string) (int, error) {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %s", portStr)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port out of range: %d", port)
	}
	return port, nil
}

// startWithGracefulShutdown starts the HTTP server with graceful shutdown.
func startWithGracefulShutdown(httpServer *http.Server, srv *server.Server, logger *zerolog.Logger) error {
	// Server errors channel
	serverErr := make(chan error, 1)

	// Start server in goroutine
	go func() {
		logger.Info().
			Str("addr", httpServer.Addr).
			Str("service", "API").
			Msg("HTTP server listening")

		fmt.Printf("🚀 API server listening on %s\n", httpServer.Addr)
		fmt.Println("   Press Ctrl+C to stop")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

		fmt.Printf("\n%s Shutting down API server...\n", emoji.Stop)

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Shutdown HTTP server
		if err := httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}

		// Shutdown background services
		if err := srv.Shutdown(ctx); err != nil {
			logger.Warn().Err(err).Msg("Background services shutdown had issues")
		}

		logger.Info().Msg("Server stopped gracefully")
		fmt.Printf("%s API server stopped gracefully\n", emoji.Success)
		return nil
	}
}
