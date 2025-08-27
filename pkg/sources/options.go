package sources

import "github.com/agentstation/starmap/pkg/catalogs"

// sourceOptions is the internal configuration for sources
type sourceOptions struct {
	// Provider filtering (needed by provider source)
	ProviderID *catalogs.ProviderID

	// Behavior flags (needed by various sources)
	Fresh    bool // Fresh sync (delete existing before adding)
	SafeMode bool // Don't delete models, only add/update

	// Source-specific data passed as context
	Context map[string]any // For source-specific options
}

// defaultSourceOptions returns source options with default values
func defaultSourceOptions() *sourceOptions {
	return &sourceOptions{
		ProviderID: nil,
		Fresh:      false,
		SafeMode:   false,
		Context:    make(map[string]any),
	}
}

// SourceOption is a function that configures sourceOptions
type SourceOption func(*sourceOptions)

// WithProviderFilter configures filtering for a specific provider
func WithProviderFilter(providerID catalogs.ProviderID) SourceOption {
	return func(opts *sourceOptions) {
		opts.ProviderID = &providerID
	}
}

// WithFresh configures fresh sync mode for sources
func WithFresh(fresh bool) SourceOption {
	return func(opts *sourceOptions) {
		opts.Fresh = fresh
	}
}

// WithSafeMode configures safe mode for sources
func WithSafeMode(safeMode bool) SourceOption {
	return func(opts *sourceOptions) {
		opts.SafeMode = safeMode
	}
}

// WithSourceContext adds source-specific context data
func WithSourceContext(key string, value any) SourceOption {
	return func(opts *sourceOptions) {
		if opts.Context == nil {
			opts.Context = make(map[string]any)
		}
		opts.Context[key] = value
	}
}

// ApplyOptions applies a set of options to create configured sourceOptions
// This is a helper for sources to use internally
func ApplyOptions(opts ...SourceOption) *sourceOptions {
	options := defaultSourceOptions()
	for _, opt := range opts {
		opt(options)
	}
	return options
}