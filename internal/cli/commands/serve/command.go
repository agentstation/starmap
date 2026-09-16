// Package serve provides HTTP server commands for the Starmap CLI.
package serve

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/internal/cli/emoji"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime"
	"github.com/agentstation/starmap/server"
	"github.com/agentstation/starmap/server/administration"
)

type application interface {
	Runtime(context.Context, ...runtime.Option) (*runtime.Runtime, error)
	Logger() *zerolog.Logger
	CatalogAcquisition(*starmap.Client) (*acquisition.Syncer, error)
	AdministrationConfig() (administration.Config, error)
	ConfigurationReports() (*administration.Reports, error)
	CatalogSettings() catalogconfig.Config
}

// NewCommand creates the serve command using app context.
func NewCommand(app application) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "serve",
		GroupID: "server",
		Short:   "Start the REST API server",
		Long: `Start a production-ready REST API server for the starmap catalog.

Features:
  - RESTful endpoints for models, providers, and catalog management
  - Heartbeat-enabled Server-Sent Events for catalog publications (/api/v1/updates/stream)
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
	cmd.Flags().Int("port", 8080, "Server port")
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
	serverDefaults := server.DefaultConfig()
	cmd.Flags().Duration(
		"sse-heartbeat-interval", serverDefaults.SSEHeartbeatInterval,
		"SSE comment heartbeat interval",
	)
	cmd.Flags().Duration(
		"sse-write-timeout", serverDefaults.SSEWriteTimeout,
		"per-frame SSE write and flush timeout",
	)

	// Features flags
	cmd.Flags().Bool("metrics", true, "Enable metrics endpoint")
	cmd.Flags().String("prefix", "/api/v1", "API path prefix")

	return cmd
}

// runServer starts the API server.
func runServer(cmd *cobra.Command, _ []string, app application) error {
	// Parse flags into configuration
	cfg, err := parseConfig(cmd)
	if err != nil {
		return err
	}
	administrationConfig, err := app.AdministrationConfig()
	if err != nil {
		return err
	}
	var manager *administration.Manager
	if _, err := os.Lstat(filepath.Join(administrationConfig.StateDirectory, "admin")); err == nil {
		manager, err = administration.Open(cmd.Context(), administrationConfig)
		if err != nil {
			return err
		}
		defer func() { _ = manager.Close() }()
	} else if !os.IsNotExist(err) {
		return err
	}
	settings := app.CatalogSettings()
	if manager == nil && (settings.AuthorityOrigin.Enabled || settings.SourceKind == runtime.SourceStarmap) {
		return &errors.ConfigError{Component: "server administration", Message: "initialize private subscriber and administrator identities with starmap admin init before serving an internal catalog"}
	}
	if manager != nil {
		cfg.AuthEnabled = true
	}
	reports, err := app.ConfigurationReports()
	if err != nil {
		return err
	}
	serverOptions := []server.Option{server.WithConfigurationReports(reports)}
	if manager != nil {
		serverOptions = append(serverOptions, server.WithAdministration(manager, administrationConfig.Audience))
	}
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

	// The connected runtime opens first. It serves the verified embedded
	// catalog at once and pulls the configured source in the background, so the
	// listener never waits for the network. The listen address separates the
	// schedule phase of two replicas on one host.
	logger.Debug().Msg("Opening the connected catalog runtime")
	connected, err := app.Runtime(
		cmd.Context(),
		runtime.WithListenAddress(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)),
	)
	if err != nil {
		return fmt.Errorf("opening the catalog runtime: %w", err)
	}
	syncer, err := app.CatalogAcquisition(connected.Client())
	if err != nil {
		return fmt.Errorf("composing catalog acquisition: %w", err)
	}

	logger.Debug().Msg("Creating server instance")
	srv, err := server.New(
		connected.Client(),
		cfg,
		append(serverOptions, server.WithLogger(logger), server.WithRuntime(connected), server.WithSyncer(syncer))...,
	)
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}
	logger.Debug().Msg("Server instance created")

	// Activate server-owned services. SSE connections remain request-owned.
	logger.Debug().Msg("Activating server services")
	if err := srv.Start(); err != nil {
		return fmt.Errorf("starting server services: %w", err)
	}
	logger.Debug().Msg("Server services active")

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
	// Pass cmd.Context() which has signal handling from main.go
	logger.Debug().Msg("Starting HTTP server listener with graceful shutdown handling")
	return startWithGracefulShutdown(cmd.Context(), httpServer, srv, logger)
}

// parseConfig parses command flags into server configuration.
func parseConfig(cmd *cobra.Command) (server.Config, error) {
	port := mustGetInt(cmd, "port")
	host := mustGetString(cmd, "host")
	corsEnabled := mustGetBool(cmd, "cors")
	corsOrigins := mustGetStringSlice(cmd, "cors-origins")
	if len(corsOrigins) > 0 {
		corsEnabled = true
	}
	authEnabled := mustGetBool(cmd, "auth")
	authHeader := mustGetString(cmd, "auth-header")
	rateLimit := mustGetInt(cmd, "rate-limit")
	cacheTTL := mustGetInt(cmd, "cache-ttl")
	readTimeout := mustGetDuration(cmd, "read-timeout")
	writeTimeout := mustGetDuration(cmd, "write-timeout")
	idleTimeout := mustGetDuration(cmd, "idle-timeout")
	sseHeartbeatInterval := mustGetDuration(cmd, "sse-heartbeat-interval")
	sseWriteTimeout := mustGetDuration(cmd, "sse-write-timeout")
	metricsEnabled := mustGetBool(cmd, "metrics")
	pathPrefix := mustGetString(cmd, "prefix")

	var err error
	host, err = listenerEnvironment(cmd, "host", "STARMAP_SERVER_HOST", "HTTP_HOST", host)
	if err != nil {
		return server.Config{}, err
	}
	portValue, err := listenerEnvironment(cmd, "port", "STARMAP_SERVER_PORT", "HTTP_PORT", strconv.Itoa(port))
	if err != nil {
		return server.Config{}, err
	}
	port, err = parsePort(portValue)
	if err != nil {
		return server.Config{}, &errors.ValidationError{Field: "server.port", Message: "requires a valid TCP port"}
	}
	if host == "" {
		return server.Config{}, &errors.ValidationError{Field: "server.host", Message: "requires an explicit bind address"}
	}

	return server.Config{
		Host:                 host,
		Port:                 port,
		PathPrefix:           pathPrefix,
		CORSEnabled:          corsEnabled,
		CORSOrigins:          corsOrigins,
		AuthEnabled:          authEnabled,
		AuthHeader:           authHeader,
		RateLimit:            rateLimit,
		CacheTTL:             time.Duration(cacheTTL) * time.Second,
		ReadTimeout:          readTimeout,
		WriteTimeout:         writeTimeout,
		IdleTimeout:          idleTimeout,
		SSEHeartbeatInterval: sseHeartbeatInterval,
		SSEWriteTimeout:      sseWriteTimeout,
		MetricsEnabled:       metricsEnabled,
	}, nil
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
// Cancel the context to stop the server gracefully.
func startWithGracefulShutdown(ctx context.Context, httpServer *http.Server, srv *server.Server, logger *zerolog.Logger) error {
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

	// Wait for server error or context cancellation (e.g., SIGINT/SIGTERM from main.go)
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info().Msg("Shutdown signal received via context")

		fmt.Printf("\n%s Shutting down API server...\n", emoji.Stop)

		// Create fresh shutdown context with timeout for cleanup operations
		// Use Background() since the parent context is already cancelled
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Shutdown HTTP server
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}

		// Shutdown background services
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Warn().Err(err).Msg("Background services shutdown had issues")
		}

		logger.Info().Msg("Server stopped gracefully")
		fmt.Printf("%s API server stopped gracefully\n", emoji.Success)
		return nil
	}
}

// mustGetInt retrieves an integer flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetInt(cmd *cobra.Command, name string) int {
	val, err := cmd.Flags().GetInt(name)
	if err != nil {
		panic(fmt.Sprintf("programming error: failed to get flag %q: %v", name, err))
	}
	return val
}

// mustGetString retrieves a string flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetString(cmd *cobra.Command, name string) string {
	val, err := cmd.Flags().GetString(name)
	if err != nil {
		panic(fmt.Sprintf("programming error: failed to get flag %q: %v", name, err))
	}
	return val
}

// mustGetBool retrieves a boolean flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetBool(cmd *cobra.Command, name string) bool {
	val, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(fmt.Sprintf("programming error: failed to get flag %q: %v", name, err))
	}
	return val
}

// mustGetStringSlice retrieves a string slice flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetStringSlice(cmd *cobra.Command, name string) []string {
	val, err := cmd.Flags().GetStringSlice(name)
	if err != nil {
		panic(fmt.Sprintf("programming error: failed to get flag %q: %v", name, err))
	}
	return val
}

// mustGetDuration retrieves a duration flag value or panics if the flag does not exist.
// Use it only for flags that this package defines.
func mustGetDuration(cmd *cobra.Command, name string) time.Duration {
	val, err := cmd.Flags().GetDuration(name)
	if err != nil {
		panic(fmt.Sprintf("programming error: failed to get flag %q: %v", name, err))
	}
	return val
}

// listenerEnvironment preserves explicit flags and diagnoses conflicting listener variables.
func listenerEnvironment(cmd *cobra.Command, flag, primary, legacy, fallback string) (string, error) {
	selected, primaryPresent := os.LookupEnv(primary)
	old, legacyPresent := os.LookupEnv(legacy)
	if legacyPresent {
		cmd.PrintErrln(legacy + " is deprecated; use " + primary + " or --" + flag)
	}
	if cmd.Flags().Changed(flag) {
		return fallback, nil
	}
	if primaryPresent && legacyPresent && selected != old {
		return "", &errors.ValidationError{Field: primary, Message: "conflicts with " + legacy + "; remove the legacy variable or set the explicit flag"}
	}
	if primaryPresent {
		return selected, nil
	}
	if legacyPresent {
		return old, nil
	}
	return fallback, nil
}
