package starmap

import (
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ============================================================================
// Starmap Options
// ============================================================================

// options holds the configuration for a Starmap instance
type options struct {
	// Remote server configuration
	remoteServerURL    *string
	remoteServerAPIKey *string
	remoteServerOnly   bool // If true (enabled), don't use any other sources for catalog updates including provider APIs

	// Update configuration
	autoUpdatesEnabled bool
	autoUpdateInterval time.Duration
	autoUpdateFunc     AutoUpdateFunc

	// Initial catalog
	initialCatalog *catalogs.Catalog

	// Local source configuration
	localPath string // Path for local source catalog
}

func defaultOptions() *options {
	return &options{
		autoUpdatesEnabled: true,          // Default to auto-updates enabled
		autoUpdateInterval: constants.DefaultUpdateInterval, // Default to hourly updates
		autoUpdateFunc:     nil,           // Default to no auto-update function
		initialCatalog:     nil,           // Default to no initial catalog
		remoteServerURL:    nil,           // Default to no remote server
		remoteServerAPIKey: nil,           // Default to no remote server API key
		remoteServerOnly:   false,         // Default to not only use remote server
	}
}

// Option is a function that configures a Starmap instance
type Option func(*options) error

// apply applies the given options to the options
func (s *starmap) apply(opts ...Option) error {
	for _, opt := range opts {
		if err := opt(s.options); err != nil {
			return errors.WrapResource("apply", "option", "", err)
		}
	}
	return nil
}

// WithRemoteServer configures the remote server for catalog updates.
// A url is required, an api key can be provided for authentication,
// otherwise use nil to skip Bearer token authentication.
func WithRemoteServer(url string, apiKey *string) Option {
	return func(o *options) error {
		o.remoteServerURL = &url
		o.remoteServerAPIKey = apiKey
		return nil
	}
}

// WithRemoteServerOnly configures whether to only use the remote server and not hit provider APIs
func WithRemoteServerOnly(enabled bool) Option {
	return func(o *options) error {
		o.remoteServerOnly = enabled
		return nil
	}
}

// WithAutoUpdates configures whether automatic updates are enabled
func WithAutoUpdates(enabled bool) Option {
	return func(o *options) error {
		o.autoUpdatesEnabled = enabled
		return nil
	}
}

// WithAutoUpdateInterval configures how often to automatically update the catalog
func WithAutoUpdateInterval(interval time.Duration) Option {
	return func(o *options) error {
		o.autoUpdateInterval = interval
		return nil
	}
}

// AutoUpdateFunc is a function that updates the catalog
type AutoUpdateFunc func(catalogs.Catalog) (catalogs.Catalog, error)

// WithAutoUpdateFunc configures a custom function for updating the catalog
func WithAutoUpdateFunc(fn AutoUpdateFunc) Option {
	return func(o *options) error {
		o.autoUpdateFunc = fn
		return nil
	}
}

// WithInitialCatalog configures the initial catalog to use
func WithInitialCatalog(catalog catalogs.Catalog) Option {
	return func(o *options) error {
		o.initialCatalog = &catalog
		return nil
	}
}

// WithLocalPath configures the local source to use a specific catalog path
func WithLocalPath(path string) Option {
	return func(o *options) error {
		o.localPath = path
		return nil
	}
}

// ============================================================================
// Sync Options
// ============================================================================

// SyncOptions controls the overall sync orchestration in Starmap.Sync()
type SyncOptions struct {
	// Orchestration control
	DryRun      bool          // Show changes without applying them
	AutoApprove bool          // Skip confirmation prompts
	FailFast    bool          // Stop on first source error instead of continuing
	Timeout     time.Duration // Timeout for the entire sync operation

	// Source selection
	Sources    []sources.SourceName // Which sources to use (empty means all)
	ProviderID *catalogs.ProviderID // Filter for specific provider

	// Output control (used AFTER merging)
	OutputPath string // Where to save final catalog (empty means default location)

	// Context for passing to sources
	Context map[string]any // For passing data to sources
}

// Apply applies the given options to the sync options
func (s *SyncOptions) apply(opts ...SyncOption) *SyncOptions {
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// defaultSyncOptions returns sync options with default values
func defaultSyncOptions() *SyncOptions {
	return &SyncOptions{
		DryRun:      false,
		AutoApprove: false,
		FailFast:    false,
		Timeout:     0,
		Sources:     nil,
		ProviderID:  nil,
		OutputPath:  "",
		Context:     make(map[string]any),
	}
}

// NewSyncOptions returns sync options with default values
func NewSyncOptions(opts ...SyncOption) *SyncOptions {
	return defaultSyncOptions().apply(opts...)
}

// SyncOption is a function that configures SyncOptions
type SyncOption func(*SyncOptions)

// WithDryRun configures dry run mode
func WithDryRun(dryRun bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.DryRun = dryRun
	}
}

// WithAutoApprove configures auto approval
func WithAutoApprove(autoApprove bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.AutoApprove = autoApprove
	}
}

// WithFailFast configures fail-fast behavior
func WithFailFast(failFast bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.FailFast = failFast
	}
}

// WithTimeout configures the sync timeout
func WithTimeout(timeout time.Duration) SyncOption {
	return func(opts *SyncOptions) {
		opts.Timeout = timeout
	}
}

// WithSources configures which sources to use
func WithSources(sourceNames ...sources.SourceName) SyncOption {
	return func(opts *SyncOptions) {
		opts.Sources = sourceNames
	}
}

// WithProvider configures syncing for a specific provider only
func WithProvider(providerID catalogs.ProviderID) SyncOption {
	return func(opts *SyncOptions) {
		opts.ProviderID = &providerID
	}
}

// WithOutputPath configures the output path for saving
func WithOutputPath(path string) SyncOption {
	return func(opts *SyncOptions) {
		opts.OutputPath = path
	}
}

// WithContext adds context data for sources
func WithContext(key string, value any) SyncOption {
	return func(opts *SyncOptions) {
		if opts.Context == nil {
			opts.Context = make(map[string]any)
		}
		opts.Context[key] = value
	}
}
