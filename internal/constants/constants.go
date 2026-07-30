// Package constants defines shared implementation limits and defaults.
package constants

import "time"

// Timeout constants define various timeout durations used in the application.
const (
	// DefaultHTTPTimeout is the standard timeout for HTTP requests to provider APIs.
	DefaultHTTPTimeout = 30 * time.Second

	// DefaultTimeout is the standard timeout for general operations.
	DefaultTimeout = 10 * time.Second

	// DefaultCatalogProjectionTimeout bounds full-catalog filesystem
	// projection and startup repair. The human-readable workspace intentionally
	// renders the complete capability surface and may contain hundreds of model
	// files, so slow or instrumented filesystems receive a bounded margin.
	DefaultCatalogProjectionTimeout = 3 * time.Minute

	// UpdateContextTimeout is the timeout for each catalog update operation.
	UpdateContextTimeout = 5 * time.Minute

	// ProviderFetchTimeout is the timeout for fetching models from a single provider.
	ProviderFetchTimeout = 2 * time.Minute

	// SyncCleanupTimeout is the timeout for source cleanup after sync operations.
	SyncCleanupTimeout = 30 * time.Second

	// CatalogStoreLockRetryDelay is the retry interval for a contended filesystem commit lock.
	CatalogStoreLockRetryDelay = 10 * time.Millisecond
)

// File permission constants define standard Unix file permissions.
const (
	// DirPermissions is the default permission for created directories (rwxr-xr-x).
	DirPermissions = 0755

	// FilePermissions is the default permission for created files (rw-r--r--).
	FilePermissions = 0644

	// ExecutablePermissions is for executable files (rwxr-xr-x).
	ExecutablePermissions = 0755

	// SecureFilePermissions is for sensitive files like API keys (rw-------).
	SecureFilePermissions = 0600
)

// Limit constants define various limits and capacities.
const (
	// MaxConcurrentProviders is the maximum number of providers to sync concurrently.
	MaxConcurrentProviders = 5

	// MaxCatalogModels is the maximum number of models in a catalog.
	MaxCatalogModels = 10000

	// MaxProviders is the maximum number of providers.
	MaxProviders = 100

	// MaxSourceProviders bounds provider records in one external source payload.
	// Source catalogs may include aliases and upstreams that do not become
	// canonical Starmap providers.
	MaxSourceProviders = 512

	// MaxSourcePayloadBytes bounds one provider or catalog source JSON payload.
	MaxSourcePayloadBytes = 16 << 20

	// MaxServerRequestBodyBytes bounds one JSON request accepted by the HTTP
	// server. Current request schemas are small query descriptions, not catalog
	// payloads.
	MaxServerRequestBodyBytes = 1 << 20
)

const (
	// CacheTTL is the default time-to-live for cached data.
	CacheTTL = 15 * time.Minute
)

// Path constants.
const (
	// DefaultCatalogPath is the default human-editable provider YAML workspace.
	DefaultCatalogPath = "~/.starmap/catalog"

	// DefaultCatalogStatePath is the CLI's machine-owned immutable generation
	// database. Go consumers normally provide their own catalogstore.Store.
	DefaultCatalogStatePath = "~/.starmap/state/catalog"

	// DefaultCachePath is the default path for cache files.
	DefaultCachePath = "~/.starmap/cache"

	// DefaultLogsPath is the default path for log files.
	DefaultLogsPath = "~/.starmap/logs"

	// DefaultSourcesPath is the default path for external source data.
	DefaultSourcesPath = "~/.starmap/sources"
)

const (
	// ModelsDevGit is the Git URL for the models.dev repository.
	ModelsDevGit = "https://github.com/neuralmagic/models.dev.git"
)
