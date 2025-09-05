package starmap

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// ============================================================================
// Starmap Options
// ============================================================================

// options holds the configuration for a Starmap instance.
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
		autoUpdatesEnabled: true,                            // Default to auto-updates enabled
		autoUpdateInterval: constants.DefaultUpdateInterval, // Default to hourly updates
		autoUpdateFunc:     nil,                             // Default to no auto-update function
		initialCatalog:     nil,                             // Default to no initial catalog
		remoteServerURL:    nil,                             // Default to no remote server
		remoteServerAPIKey: nil,                             // Default to no remote server API key
		remoteServerOnly:   false,                           // Default to not only use remote server
	}
}

// Option is a function that configures a Starmap instance.
type Option func(*options) error

// apply applies the given options to the options.
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

// WithRemoteServerOnly configures whether to only use the remote server and not hit provider APIs.
func WithRemoteServerOnly(enabled bool) Option {
	return func(o *options) error {
		o.remoteServerOnly = enabled
		return nil
	}
}

// WithAutoUpdates configures whether automatic updates are enabled.
func WithAutoUpdates(enabled bool) Option {
	return func(o *options) error {
		o.autoUpdatesEnabled = enabled
		return nil
	}
}

// WithAutoUpdateInterval configures how often to automatically update the catalog.
func WithAutoUpdateInterval(interval time.Duration) Option {
	return func(o *options) error {
		o.autoUpdateInterval = interval
		return nil
	}
}

// AutoUpdateFunc is a function that updates the catalog.
type AutoUpdateFunc func(catalogs.Catalog) (catalogs.Catalog, error)

// WithAutoUpdateFunc configures a custom function for updating the catalog.
func WithAutoUpdateFunc(fn AutoUpdateFunc) Option {
	return func(o *options) error {
		o.autoUpdateFunc = fn
		return nil
	}
}

// WithInitialCatalog configures the initial catalog to use.
func WithInitialCatalog(catalog catalogs.Catalog) Option {
	return func(o *options) error {
		o.initialCatalog = &catalog
		return nil
	}
}

// WithLocalPath configures the local source to use a specific catalog path.
func WithLocalPath(path string) Option {
	return func(o *options) error {
		o.localPath = path
		return nil
	}
}

// ============================================================================
// Sync Options
// ============================================================================

// SyncOptions controls the overall sync orchestration in Starmap.Sync().
type SyncOptions struct {
	// Orchestration control
	DryRun      bool          // Show changes without applying them
	AutoApprove bool          // Skip confirmation prompts
	FailFast    bool          // Stop on first source error instead of continuing
	Timeout     time.Duration // Timeout for the entire sync operation

	// Source selection
	Sources    []sources.Type       // Which sources to use (empty means all)
	ProviderID *catalogs.ProviderID // Filter for specific provider

	// Output control (used AFTER merging)
	OutputPath string // Where to save final catalog (empty means default location)

	// Source behavior control
	Fresh              bool // Delete existing models and fetch fresh from APIs (destructive)
	CleanModelsDevRepo bool // Remove temporary models.dev repository after update
	Reformat           bool // Reformat providers.yaml file even without changes
}

// Apply applies the given options to the sync options.
func (s *SyncOptions) apply(opts ...SyncOption) *SyncOptions {
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// defaultSyncOptions returns sync options with default values.
func defaultSyncOptions() *SyncOptions {
	return &SyncOptions{
		DryRun:             false,
		AutoApprove:        false,
		FailFast:           false,
		Timeout:            0,
		Sources:            nil,
		ProviderID:         nil,
		OutputPath:         "",
		Fresh:              false,
		CleanModelsDevRepo: false,
		Reformat:           false,
	}
}

// NewSyncOptions returns sync options with default values.
func NewSyncOptions(opts ...SyncOption) *SyncOptions {
	return defaultSyncOptions().apply(opts...)
}

// SyncOption is a function that configures SyncOptions.
type SyncOption func(*SyncOptions)

// Validate checks if the sync options are valid.
func (s *SyncOptions) Validate(providers *catalogs.Providers) error {
	// Validate timeout
	if s.Timeout < 0 {
		return &errors.ValidationError{
			Field:   "Timeout",
			Value:   s.Timeout,
			Message: "timeout must be non-negative",
		}
	}

	// Validate provider ID if specified
	if s.ProviderID != nil {
		_, found := providers.Get(*s.ProviderID)
		if !found {
			return &errors.ValidationError{
				Field:   "ProviderID",
				Value:   *s.ProviderID,
				Message: fmt.Sprintf("provider '%s' not found", *s.ProviderID),
			}
		}
	}

	// Validate output path if specified
	if s.OutputPath != "" {
		// Check if parent directory exists
		dir := filepath.Dir(s.OutputPath)
		if dir != "." && dir != "/" {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return &errors.ValidationError{
					Field:   "OutputPath",
					Value:   s.OutputPath,
					Message: fmt.Sprintf("output directory '%s' does not exist", dir),
				}
			}
		}
	}

	return nil
}

// SourceOptions converts sync options to properly typed source options.
func (s *SyncOptions) SourceOptions() []sources.Option {
	var sourceOpts []sources.Option

	if s.ProviderID != nil {
		sourceOpts = append(sourceOpts, sources.WithProviderFilter(*s.ProviderID))
	}
	if s.Fresh {
		sourceOpts = append(sourceOpts, sources.WithFresh(true))
	}
	if s.CleanModelsDevRepo {
		sourceOpts = append(sourceOpts, sources.WithCleanupRepo(true))
	}
	if s.Reformat {
		sourceOpts = append(sourceOpts, sources.WithReformat(true))
	}

	return sourceOpts
}

// WithDryRun configures dry run mode.
func WithDryRun(dryRun bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.DryRun = dryRun
	}
}

// WithAutoApprove configures auto approval.
func WithAutoApprove(autoApprove bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.AutoApprove = autoApprove
	}
}

// WithFailFast configures fail-fast behavior.
func WithFailFast(failFast bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.FailFast = failFast
	}
}

// WithTimeout configures the sync timeout.
func WithTimeout(timeout time.Duration) SyncOption {
	return func(opts *SyncOptions) {
		opts.Timeout = timeout
	}
}

// WithSources configures which sources to use.
func WithSources(types ...sources.Type) SyncOption {
	return func(opts *SyncOptions) {
		opts.Sources = types
	}
}

// WithProvider configures syncing for a specific provider only.
func WithProvider(providerID catalogs.ProviderID) SyncOption {
	return func(opts *SyncOptions) {
		opts.ProviderID = &providerID
	}
}

// WithOutputPath configures the output path for saving.
func WithOutputPath(path string) SyncOption {
	return func(opts *SyncOptions) {
		opts.OutputPath = path
	}
}

// WithFresh configures whether to delete existing models and fetch fresh from APIs.
func WithFresh(fresh bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.Fresh = fresh
	}
}

// WithCleanModelsDevRepo configures whether to remove temporary models.dev repository after update.
func WithCleanModelsDevRepo(cleanup bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.CleanModelsDevRepo = cleanup
	}
}

// WithReformat configures whether to reformat providers.yaml file even without changes.
func WithReformat(reformat bool) SyncOption {
	return func(opts *SyncOptions) {
		opts.Reformat = reformat
	}
}
