// Package constants provides shared constants used throughout the starmap codebase.
// This includes timeouts, limits, file permissions, and other configuration values
// that should be consistent across the application.
package constants

import "time"

// Timeout constants define various timeout durations used in the application
const (
	// DefaultHTTPTimeout is the standard timeout for HTTP requests to provider APIs
	DefaultHTTPTimeout = 30 * time.Second

	// DefaultTimeout is the standard timeout for general operations
	DefaultTimeout = 10 * time.Second

	// UpdateContextTimeout is the timeout for each catalog update operation
	UpdateContextTimeout = 5 * time.Minute

	// DefaultUpdateInterval is the default interval between automatic catalog updates
	DefaultUpdateInterval = 1 * time.Hour

	// ProviderFetchTimeout is the timeout for fetching models from a single provider
	ProviderFetchTimeout = 2 * time.Minute

	// CommandTimeout is the default timeout for CLI commands
	CommandTimeout = 10 * time.Minute

	// LongRunningTimeout is for operations that may take extended time
	LongRunningTimeout = 30 * time.Minute

	// SyncTimeout is the timeout for sync operations
	SyncTimeout = 30 * time.Minute

	// RetryBackoff is the base backoff duration for retries
	RetryBackoff = 1 * time.Second

	// MaxRetryBackoff is the maximum backoff duration for retries
	MaxRetryBackoff = 30 * time.Second
)

// File permission constants define standard Unix file permissions
const (
	// DirPermissions is the default permission for created directories (rwxr-xr-x)
	DirPermissions = 0755

	// FilePermissions is the default permission for created files (rw-r--r--)
	FilePermissions = 0644

	// ExecutablePermissions is for executable files (rwxr-xr-x)
	ExecutablePermissions = 0755

	// SecureFilePermissions is for sensitive files like API keys (rw-------)
	SecureFilePermissions = 0600
)

// Limit constants define various limits and capacities
const (
	// MaxRetries is the maximum number of retry attempts for failed operations
	MaxRetries = 3

	// MaxConcurrentAPIs is the maximum number of concurrent API calls
	MaxConcurrentAPIs = 10

	// MaxConcurrentProviders is the maximum number of providers to sync concurrently
	MaxConcurrentProviders = 5

	// MaxConcurrentRequests is the maximum number of concurrent requests
	MaxConcurrentRequests = 10

	// OutputBufferSize is the maximum size of output buffers in bytes
	OutputBufferSize = 30000

	// MaxModelNameLength is the maximum allowed length for model names
	MaxModelNameLength = 256

	// MaxDescriptionLength is the maximum allowed length for descriptions
	MaxDescriptionLength = 4096

	// DefaultPageSize is the default number of items per page for paginated results
	DefaultPageSize = 100

	// MaxPageSize is the maximum allowed page size for paginated results
	MaxPageSize = 1000

	// ChannelBufferSize is the default buffer size for channels
	ChannelBufferSize = 100

	// WriteBufferSize is the default buffer size for write operations
	WriteBufferSize = 4096

	// MaxCatalogModels is the maximum number of models in a catalog
	MaxCatalogModels = 10000

	// MaxProviders is the maximum number of providers
	MaxProviders = 100
)

// Rate limiting constants
const (
	// DefaultRateLimit is the default requests per minute for providers without specific limits
	DefaultRateLimit = 60

	// BurstSize is the token bucket burst size for rate limiting
	BurstSize = 10

	// RateLimitRetryDelay is the delay before retrying after hitting a rate limit
	RateLimitRetryDelay = 1 * time.Second

	// MaxRateLimitRetries is the maximum number of retries for rate-limited requests
	MaxRateLimitRetries = 5
)

// Cache constants
const (
	// CacheTTL is the default time-to-live for cached data
	CacheTTL = 15 * time.Minute

	// CacheCleanupInterval is how often to clean expired cache entries
	CacheCleanupInterval = 5 * time.Minute

	// MaxCacheSize is the maximum number of items in the cache
	MaxCacheSize = 1000
)

// Logging constants
const (
	// LogRotationSize is the maximum size of a log file before rotation (10 MB)
	LogRotationSize = 10 * 1024 * 1024

	// LogRotationAge is the maximum age of log files before deletion
	LogRotationAge = 7 * 24 * time.Hour

	// LogRotationBackups is the maximum number of old log files to retain
	LogRotationBackups = 5
)

// Network constants
const (
	// DialTimeout is the timeout for establishing network connections
	DialTimeout = 10 * time.Second

	// KeepAliveInterval is the interval between keep-alive probes
	KeepAliveInterval = 30 * time.Second

	// MaxIdleConnections is the maximum number of idle connections in the pool
	MaxIdleConnections = 100

	// MaxConnectionsPerHost is the maximum number of connections per host
	MaxConnectionsPerHost = 10
)

// Default values
const (
	// DefaultProviderID is the default provider when none is specified
	DefaultProviderID = "openai"

	// DefaultModelID is the default model when none is specified
	DefaultModelID = "gpt-4"

	// DefaultRegion is the default region for providers that support multiple regions
	DefaultRegion = "us-central1"

	// DefaultEnvironment is the default environment (development, staging, production)
	DefaultEnvironment = "production"
)

// Path constants
const (
	// DefaultCatalogPath is the default path for the local catalog
	DefaultCatalogPath = "~/.starmap"

	// DefaultConfigPath is the default path for configuration files
	DefaultConfigPath = "~/.starmap/config.yaml"

	// DefaultCachePath is the default path for cache files
	DefaultCachePath = "~/.starmap/cache"

	// DefaultLogsPath is the default path for log files
	DefaultLogsPath = "~/.starmap/logs"
)

// Format constants
const (
	// TimeFormatISO8601 is the ISO 8601 time format
	TimeFormatISO8601 = time.RFC3339

	// TimeFormatHuman is a human-readable time format
	TimeFormatHuman = "Jan 2, 2006 at 3:04pm MST"

	// TimeFormatLog is the format used in log files
	TimeFormatLog = "2006-01-02 15:04:05.000"

	// TimeFormatFilename is the format used in generated filenames
	TimeFormatFilename = "20060102-150405"
)

// GitHub and external resource constants
const (
	// ModelsDevURL is the URL for the models.dev website
	ModelsDevURL = "https://models.dev"

	// ModelsDevGit is the Git URL for the models.dev repository
	ModelsDevGit = "https://github.com/neuralmagic/models.dev.git"
)

// Error messages
const (
	// ErrMsgProviderNotFound is the standard error message for missing providers
	ErrMsgProviderNotFound = "provider not found"

	// ErrMsgModelNotFound is the standard error message for missing models
	ErrMsgModelNotFound = "model not found"

	// ErrMsgInvalidAPIKey is the standard error message for invalid API keys
	ErrMsgInvalidAPIKey = "invalid or missing API key"

	// ErrMsgRateLimited is the standard error message for rate limiting
	ErrMsgRateLimited = "rate limit exceeded, please try again later"

	// ErrMsgTimeout is the standard error message for timeouts
	ErrMsgTimeout = "operation timed out"

	// ErrMsgNetworkError is the standard error message for network failures
	ErrMsgNetworkError = "network error occurred"
)