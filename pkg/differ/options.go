package differ

// Option is a functional option for configuring Differ.
type Option func(*differ)

// WithIgnoredFields sets fields to ignore during comparison.
func WithIgnoredFields(fields ...string) Option {
	return func(d *differ) {
		for _, field := range fields {
			d.ignoreFields[field] = true
		}
	}
}

// WithDeepComparison enables/disables deep structural comparison.
func WithDeepComparison(enabled bool) Option {
	return func(d *differ) {
		d.deepComparison = enabled
	}
}

// WithTracking enables provenance tracking in diffs.
func WithTracking(enabled bool) Option {
	return func(d *differ) {
		d.tracking = enabled
	}
}
