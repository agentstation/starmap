package sources

import (
	"github.com/agentstation/starmap/internal/utils/ptr"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// Options is the configuration for sources.
type Options struct {
	// Provider filtering (needed by provider source)
	ProviderID *catalogs.ProviderID

	// Behavior flags (needed by various sources)
	Fresh    bool // Fresh sync (delete existing before adding)
	SafeMode bool // Don't delete models, only add/update

	// Typed source-specific options
	CleanupRepo bool // For models.dev git source - remove repo after fetch
	Reformat    bool // For file-based sources - reformat output files
}

// Defaults returns source options with default values.
func Defaults() *Options {
	return &Options{
		ProviderID:  nil,
		Fresh:       false,
		SafeMode:    false,
		CleanupRepo: false,
		Reformat:    false,
	}
}

// Option is a function that configures options.
type Option func(*Options)

// Apply applies a set of options to create configured sourceOptions
// This is a helper for sources to use internally.
func (o *Options) Apply(opts ...Option) *Options {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithProviderFilter configures filtering for a specific provider.
func WithProviderFilter(providerID catalogs.ProviderID) Option {
	return func(opts *Options) {
		opts.ProviderID = ptr.To(providerID)
	}
}

// WithFresh configures fresh sync mode for sources.
func WithFresh(fresh bool) Option {
	return func(opts *Options) {
		opts.Fresh = fresh
	}
}

// WithSafeMode configures safe mode for sources.
func WithSafeMode(safeMode bool) Option {
	return func(opts *Options) {
		opts.SafeMode = safeMode
	}
}

// WithCleanupRepo configures whether to clean up temporary repositories after fetch.
func WithCleanupRepo(cleanup bool) Option {
	return func(opts *Options) {
		opts.CleanupRepo = cleanup
	}
}

// WithReformat configures whether to reformat output files.
func WithReformat(reformat bool) Option {
	return func(opts *Options) {
		opts.Reformat = reformat
	}
}
