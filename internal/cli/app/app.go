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
	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/internal/catalog/settings"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

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
	config       *Config
	commandFlags commandFlags

	// Logger
	logger *zerolog.Logger

	// Starmap instance (lazy-initialized, singleton)
	mu      sync.RWMutex
	starmap *starmap.Client

	// Canonical catalog settings and the connected runtime they compose.
	catalogSettings settings.Config
	runtimeMu       sync.Mutex
	runtime         *starmap.Runtime

	credentialMu       sync.Mutex
	credentialResolver sources.ProviderCredentialResolver
}

// New creates an App with the given version information. It applies functional
// options to the default configuration.
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

	// The canonical catalog settings load after the options, so a test
	// configuration selects the same names that a deployment uses.
	catalogSettings, err := loadCatalogSettings(app.config)
	if err != nil {
		return nil, err
	}
	app.catalogSettings = catalogSettings

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

// CredentialResolver returns the process-owned catalog-acquisition resolver.
// One resolver owns source caching and single-flight refresh for the process.
func (a *App) CredentialResolver() (sources.ProviderCredentialResolver, error) {
	a.credentialMu.Lock()
	defer a.credentialMu.Unlock()
	if a.credentialResolver != nil {
		return a.credentialResolver, nil
	}
	catalog, err := a.Catalog()
	if err != nil {
		return nil, err
	}
	policies := make(map[auth.CredentialFieldKey]auth.ReferencePolicy)
	for providerValue, fields := range a.config.CredentialSources {
		providerID := catalogs.ProviderID(providerValue)
		provider, providerErr := catalog.Provider(providerID)
		if providerErr != nil || provider.ID != providerID {
			return nil, &errors.ValidationError{
				Field: "credential_sources.provider", Value: providerValue,
				Message: "must identify one canonical catalog provider",
			}
		}
		declaredFields := make(map[catalogs.ProviderCredentialFieldID]struct{})
		if provider.Credentials != nil {
			for _, field := range provider.Credentials.Fields {
				declaredFields[field.ID] = struct{}{}
			}
		}
		for fieldValue, config := range fields {
			fieldID := catalogs.ProviderCredentialFieldID(fieldValue)
			if _, exists := declaredFields[fieldID]; !exists {
				return nil, &errors.ValidationError{
					Field: "credential_sources.field", Value: fieldValue,
					Message: "must identify one catalog-declared provider field",
				}
			}
			reference, parseErr := auth.ParseReference(config.Reference)
			if parseErr != nil {
				return nil, errors.WrapResource(
					"parse", "credential source", providerValue+"/"+fieldValue, parseErr,
				)
			}
			policies[auth.CredentialFieldKey{ProviderID: providerID, FieldID: fieldID}] = auth.ReferencePolicy{
				Reference: reference, FallbackAmbient: config.FallbackAmbient,
			}
		}
	}
	a.credentialResolver = auth.NewResolver(auth.WithReferencePolicies(policies))
	return a.credentialResolver, nil
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

// Catalog returns the current immutable catalog from the starmap instance.
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

// Shutdown gracefully stops background tasks and releases resources.
// The context sets the timeout and can abort the shutdown.
func (a *App) Shutdown(ctx context.Context) error {
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return err
	}

	return a.closeRuntime()
}

// buildStarmapOptions constructs starmap options from the app configuration.
func (a *App) buildStarmapOptions(storeOption starmap.Option) ([]starmap.Option, error) {
	catalogOptions, err := a.catalogClientOptions()
	if err != nil {
		return nil, err
	}
	return append([]starmap.Option{storeOption}, catalogOptions...), nil
}

// catalogClientOptions returns the catalog options that every composition
// shares. The connected runtime and the read-only client both use them.
func (a *App) catalogClientOptions() ([]starmap.Option, error) {
	// The CLI always composes one human provider-YAML workspace. Merely
	// constructing the client reads it when present and never creates it.
	catalogPath, err := a.CatalogPath()
	if err != nil {
		return nil, err
	}
	opts := []starmap.Option{starmap.WithCatalogPath(catalogPath)}

	if a.config.EmbeddedBootstrapMaxAge > 0 {
		opts = append(opts, starmap.WithEmbeddedBootstrapMaxAge(a.config.EmbeddedBootstrapMaxAge))
	}
	if a.config.EmbeddedBootstrapMaxSizeBytes > 0 {
		opts = append(opts, starmap.WithEmbeddedBootstrapMaxSizeBytes(a.config.EmbeddedBootstrapMaxSizeBytes))
	}
	return opts, nil
}

func (a *App) catalogStoreOption() (starmap.Option, error) {
	path, err := a.catalogStatePath()
	if err != nil {
		return nil, err
	}
	store, err := storage.NewFilesystem(path)
	if err != nil {
		return nil, errors.WrapResource("create", "catalog store", path, err)
	}
	return starmap.WithCatalogStore(store), nil
}

// MigrateCatalogWorkspace explicitly relocates the pre-plan catalog store
// from the configured human catalog workspace path into the CLI's machine state root.
func (a *App) MigrateCatalogWorkspace(ctx context.Context) (workspace.LegacyLayoutMigrationResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.starmap != nil {
		return workspace.LegacyLayoutMigrationResult{}, &errors.ConflictError{
			Resource: "catalog workspace migration",
			Message:  "cannot migrate after the application catalog has been initialized",
		}
	}
	catalogPath, err := a.CatalogPath()
	if err != nil {
		return workspace.LegacyLayoutMigrationResult{}, err
	}
	statePath, err := a.catalogStatePath()
	if err != nil {
		return workspace.LegacyLayoutMigrationResult{}, err
	}
	return workspace.MigrateLegacyLayout(ctx, catalogPath, statePath)
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
