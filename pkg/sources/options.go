package sources

import "github.com/agentstation/starmap/pkg/catalogs"

// options is the internal configuration for sources
type options struct {
	// Provider filtering (needed by provider source)
	ProviderID *catalogs.ProviderID

	// Behavior flags (needed by various sources)
	Fresh    bool // Fresh sync (delete existing before adding)
	SafeMode bool // Don't delete models, only add/update

	// Typed source-specific options
	CleanupRepo bool // For models.dev git source - remove repo after fetch
	Reformat    bool // For file-based sources - reformat output files
}

// defaultOptions returns source options with default values
func defaultOptions() *options {
	return &options{
		ProviderID:  nil,
		Fresh:       false,
		SafeMode:    false,
		CleanupRepo: false,
		Reformat:    false,
	}
}

// Option is a function that configures options
type Option func(*options)

// WithProviderFilter configures filtering for a specific provider
func WithProviderFilter(providerID catalogs.ProviderID) Option {
	return func(opts *options) {
		opts.ProviderID = &providerID
	}
}

// WithFresh configures fresh sync mode for sources
func WithFresh(fresh bool) Option {
	return func(opts *options) {
		opts.Fresh = fresh
	}
}

// WithSafeMode configures safe mode for sources
func WithSafeMode(safeMode bool) Option {
	return func(opts *options) {
		opts.SafeMode = safeMode
	}
}

// WithCleanupRepo configures whether to clean up temporary repositories after fetch
func WithCleanupRepo(cleanup bool) Option {
	return func(opts *options) {
		opts.CleanupRepo = cleanup
	}
}

// WithReformat configures whether to reformat output files
func WithReformat(reformat bool) Option {
	return func(opts *options) {
		opts.Reformat = reformat
	}
}

// ApplyOptions applies a set of options to create configured sourceOptions
// This is a helper for sources to use internally
func ApplyOptions(opts ...Option) *options {
	options := defaultOptions()
	for _, opt := range opts {
		opt(options)
	}
	return options
}
