// Package app provides the application context and dependency management
// for the starmap CLI. It follows idiomatic Go patterns for CLI applications
// by centralizing configuration, dependency injection, and lifecycle management.
package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/application"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogscheduler"
	"github.com/agentstation/starmap/pkg/catalogstore"
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
	mu         sync.RWMutex
	starmap    *starmap.Client
	operations *catalogscheduler.Operations
}

// New creates a new App instance with the given version information.
// The app is initialized with default configuration that can be
// customized using functional options.
func New(version, commit, date, builtBy string, opts ...Option) (*App, error) {
	operations, err := catalogscheduler.NewOperations()
	if err != nil {
		return nil, err
	}
	app := &App{
		version:    version,
		commit:     commit,
		date:       date,
		builtBy:    builtBy,
		operations: operations,
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
func (a *App) Starmap(opts ...starmap.Option) (*starmap.Client, error) {
	storeOption, err := a.catalogStoreOption()
	if err != nil {
		return nil, err
	}

	// If options provided, create new instance (no caching)
	if len(opts) > 0 {
		configured := make([]starmap.Option, 0, len(opts)+1)
		configured = append(configured, storeOption)
		configured = append(configured, opts...)
		sm, err := starmap.New(configured...)
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
	o, err := a.buildStarmapOptions(storeOption)
	if err != nil {
		return nil, err
	}
	sm, err := starmap.New(o...)
	if err != nil {
		return nil, errors.WrapResource("create", "starmap", "", err)
	}

	a.starmap = sm
	return sm, nil
}

// Catalog returns the current structurally read-only snapshot from the starmap instance.
// This is a convenience method that handles the starmap initialization
// and catalog retrieval in one call.
//
// Thread Safety: sm.Catalog atomically loads an immutable generation. Collection
// reads return caller-owned copies behind interfaces that expose no mutation
// methods.
func (a *App) Catalog() (*catalogs.Catalog, error) {
	sm, err := a.Starmap()
	if err != nil {
		return nil, err
	}

	return sm.Catalog(), nil
}

// CatalogState atomically returns the current catalog and generation identity.
func (a *App) CatalogState() (starmap.CatalogState, error) {
	sm, err := a.Starmap()
	if err != nil {
		return starmap.CatalogState{}, err
	}
	return sm.CurrentCatalogState(), nil
}

// Readiness reports catalog availability and active embedded-bootstrap budgets.
func (a *App) Readiness() (starmap.CatalogReadiness, error) {
	sm, err := a.Starmap()
	if err != nil {
		return starmap.CatalogReadiness{}, err
	}
	return sm.Readiness(), nil
}

// OperationalState composes the current atomic catalog identity with all
// deployment-owned scheduler telemetry configured at the composition root.
func (a *App) OperationalState(ctx context.Context) (catalogscheduler.OperationalState, error) {
	sm, err := a.Starmap()
	if err != nil {
		return catalogscheduler.OperationalState{}, err
	}
	state := sm.CurrentCatalogState()
	return a.operations.State(ctx, catalogscheduler.CatalogIdentity{
		GenerationID: state.GenerationID,
		Sequence:     state.Sequence,
	})
}

// Shutdown performs graceful shutdown of the application.
// It stops any running background tasks and cleans up resources.
// The context controls the shutdown timeout - shutdown will abort if context is cancelled.
func (a *App) Shutdown(ctx context.Context) error {
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return err
	}

	return nil
}

// buildStarmapOptions constructs starmap options from the app configuration.
func (a *App) buildStarmapOptions(storeOption starmap.Option) ([]starmap.Option, error) {
	opts := []starmap.Option{storeOption}

	// Add the editable YAML catalog only when explicitly configured.
	exportPath, err := a.catalogExportPath()
	if err != nil {
		return nil, err
	}
	if exportPath != "" {
		opts = append(opts, starmap.WithCatalogExportPath(exportPath))
	}

	// Add embedded catalog if configured
	if a.config.UseEmbeddedCatalog {
		opts = append(opts, starmap.WithEmbeddedCatalog())
	}
	if a.config.EmbeddedBootstrapMaxAge > 0 {
		opts = append(opts, starmap.WithEmbeddedBootstrapMaxAge(a.config.EmbeddedBootstrapMaxAge))
	}
	if a.config.EmbeddedBootstrapMaxSizeBytes > 0 {
		opts = append(opts, starmap.WithEmbeddedBootstrapMaxSizeBytes(a.config.EmbeddedBootstrapMaxSizeBytes))
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

	return opts, nil
}

func (a *App) catalogStoreOption() (starmap.Option, error) {
	path, err := a.catalogDatabasePath()
	if err != nil {
		return nil, err
	}
	store, err := catalogstore.NewFilesystem(path)
	if err != nil {
		return nil, errors.WrapResource("create", "catalog store", path, err)
	}
	return starmap.WithCatalogStore(store), nil
}

func expandHomePath(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.WrapResource("resolve", "home directory", path, err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
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
func WithClient(sm *starmap.Client) Option {
	return func(a *App) error {
		a.starmap = sm
		return nil
	}
}

// WithOperations sets the deployment-owned operational-state composer.
func WithOperations(operations *catalogscheduler.Operations) Option {
	return func(a *App) error {
		if operations == nil {
			return &errors.ValidationError{Field: "application.operations", Message: "is required"}
		}
		a.operations = operations
		return nil
	}
}
