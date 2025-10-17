// Package app provides the application context and dependency management
// for the starmap CLI. It follows idiomatic Go patterns for CLI applications
// by centralizing configuration, dependency injection, and lifecycle management.
package app

import (
	"context"
	"sync"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/cmd/application"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Ensure App implements application.Application at compile time.
var _ application.Application = (*App)(nil)

// App represents the starmap application with all its dependencies.
// It provides a centralized place for configuration, logging, and
// the starmap instance, following the dependency injection pattern.
type App struct {
	// Version information
	version string
	commit  string
	date    string
	builtBy string

	// Configuration
	config *Config

	// Logger
	logger *zerolog.Logger

	// Starmap instance (lazy-initialized, singleton)
	mu      sync.RWMutex
	starmap starmap.Client
}

// New creates a new App instance with the given version information.
// The app is initialized with default configuration that can be
// customized using functional options.
func New(version, commit, date, builtBy string, opts ...Option) (*App, error) {
	app := &App{
		version: version,
		commit:  commit,
		date:    date,
		builtBy: builtBy,
	}

	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		return nil, errors.WrapResource("load", "config", "", err)
	}
	app.config = config

	// Initialize logger
	logger := NewLogger(config)
	app.logger = &logger

	// Apply any custom options
	for _, opt := range opts {
		if err := opt(app); err != nil {
			return nil, err
		}
	}

	return app, nil
}

// Version returns the version information.
func (a *App) Version() string {
	return a.version
}

// Commit returns the git commit hash.
func (a *App) Commit() string {
	return a.commit
}

// Date returns the build date.
func (a *App) Date() string {
	return a.date
}

// BuiltBy returns the build system identifier.
func (a *App) BuiltBy() string {
	return a.builtBy
}

// Config returns the application configuration.
func (a *App) Config() *Config {
	return a.config
}

// Logger returns the application logger.
func (a *App) Logger() *zerolog.Logger {
	return a.logger
}

// OutputFormat returns the configured output format.
func (a *App) OutputFormat() string {
	return a.config.Output
}

// Starmap returns the starmap instance with optional configuration.
// When called without options, returns the default cached instance (lazy-initialized, thread-safe).
// When called with options, creates a new instance with custom configuration (no caching).
//
// This consolidates the previous Starmap() and StarmapWithOptions() methods into a single,
// more idiomatic Go interface following the variadic options pattern.
func (a *App) Starmap(opts ...starmap.Option) (starmap.Client, error) {
	// If options provided, create new instance (no caching)
	if len(opts) > 0 {
		sm, err := starmap.New(opts...)
		if err != nil {
			return nil, errors.WrapResource("create", "starmap", "with custom options", err)
		}
		return sm, nil
	}

	// No options: use cached default instance with double-checked locking
	a.mu.RLock()
	if a.starmap != nil {
		sm := a.starmap
		a.mu.RUnlock()
		return sm, nil
	}
	a.mu.RUnlock()

	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check after acquiring write lock
	if a.starmap != nil {
		return a.starmap, nil
	}

	// Create starmap instance with options from config
	configOpts := a.buildStarmapOptions()
	sm, err := starmap.New(configOpts...)
	if err != nil {
		return nil, errors.WrapResource("create", "starmap", "", err)
	}

	a.starmap = sm
	return sm, nil
}

// Catalog returns a deep copy of the current catalog from the starmap instance.
// This is a convenience method that handles the starmap initialization
// and catalog retrieval in one call.
//
// Thread Safety: This method is thread-safe because sm.Catalog() acquires
// a read lock and returns a deep copy (see starmap.go:71-76). Each caller
// receives an independent catalog instance with no shared mutable state.
//
// Performance: ~350-400ns per call with 9-10 allocations (single copy).
//
// Per docs/ARCHITECTURE.md § Thread Safety section, this ALWAYS returns a deep copy
// (provided by sm.Catalog()).
func (a *App) Catalog() (catalogs.Catalog, error) {
	sm, err := a.Starmap()
	if err != nil {
		return nil, err
	}

	// sm.Catalog() returns a deep copy with proper locking - no second copy needed
	catalog, err := sm.Catalog()
	if err != nil {
		return nil, errors.WrapResource("get", "catalog", "", err)
	}

	return catalog, nil
}

// Shutdown performs graceful shutdown of the application.
// It stops any running background tasks and cleans up resources.
func (a *App) Shutdown(_ context.Context) error {
	a.mu.RLock()
	sm := a.starmap
	a.mu.RUnlock()

	if sm != nil {
		// Stop auto-updates if running
		if err := sm.AutoUpdatesOff(); err != nil {
			a.logger.Error().Err(err).Msg("Failed to stop auto-updates during shutdown")
		}
	}

	return nil
}

// buildStarmapOptions constructs starmap options from the app configuration.
func (a *App) buildStarmapOptions() []starmap.Option {
	var opts []starmap.Option

	// Add local path if configured
	if a.config.LocalPath != "" {
		opts = append(opts, starmap.WithLocalPath(a.config.LocalPath))
	}

	// Add embedded catalog if configured
	if a.config.UseEmbeddedCatalog {
		opts = append(opts, starmap.WithEmbeddedCatalog())
	}

	// Add auto-update settings
	if !a.config.AutoUpdatesEnabled {
		opts = append(opts, starmap.WithAutoUpdatesDisabled())
	} else if a.config.AutoUpdateInterval > 0 {
		opts = append(opts, starmap.WithAutoUpdateInterval(a.config.AutoUpdateInterval))
	}

	// Add remote server if configured
	if a.config.RemoteServerURL != "" {
		var apiKey *string
		if a.config.RemoteServerAPIKey != "" {
			apiKey = &a.config.RemoteServerAPIKey
		}

		if a.config.RemoteServerOnly {
			opts = append(opts, starmap.WithRemoteServerOnly(a.config.RemoteServerURL))
		} else {
			opts = append(opts, starmap.WithRemoteServerURL(a.config.RemoteServerURL))
		}

		if apiKey != nil {
			opts = append(opts, starmap.WithRemoteServerAPIKey(*apiKey))
		}
	}

	return opts
}

// Option is a functional option for configuring the App.
type Option func(*App) error

// WithConfig sets a custom configuration.
func WithConfig(config *Config) Option {
	return func(a *App) error {
		a.config = config
		return nil
	}
}

// WithLogger sets a custom logger.
func WithLogger(logger *zerolog.Logger) Option {
	return func(a *App) error {
		a.logger = logger
		return nil
	}
}

// WithClient sets a custom starmap instance (useful for testing).
func WithClient(sm starmap.Client) Option {
	return func(a *App) error {
		a.starmap = sm
		return nil
	}
}
