package catalogs

// MergeOption configures how catalogs are merged.
type MergeOption func(*MergeOptions)

// MergeOptions holds merge configuration.
type MergeOptions struct {
	Strategy MergeStrategy // nil means use source catalog's suggestion
}

// WithStrategy overrides the merge strategy.
func WithStrategy(s MergeStrategy) MergeOption {
	return func(c *MergeOptions) {
		c.Strategy = s
	}
}

// ParseMergeOptions processes merge options and returns the configuration.
func ParseMergeOptions(opts ...MergeOption) *MergeOptions {
	cfg := &MergeOptions{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}
