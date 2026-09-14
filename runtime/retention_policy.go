package runtime

import (
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// RetentionPolicy bounds automatic collection independently of catalog update permissions.
// Required generations can exceed the limits. Collection never removes them to satisfy a limit.
type RetentionPolicy struct {
	Enabled        bool
	Interval       time.Duration
	MaxGenerations int
	MaxBytes       int64
	ScanEntries    int
	InputMaxBytes  int64
}

// DefaultRetentionPolicy selects hourly collection with bounded scans and retained bytes.
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{Enabled: true, Interval: time.Hour, MaxGenerations: 32,
		MaxBytes: 512 << 20, ScanEntries: storage.DefaultRetentionScanEntries, InputMaxBytes: 256 << 20}
}

// Validate rejects unbounded or invalid retention settings, including when operators disable scheduling.
func (p RetentionPolicy) Validate() error {
	if p.Interval <= 0 || p.MaxGenerations < 1 || p.MaxBytes < 1 || p.InputMaxBytes < 1 ||
		p.ScanEntries < 1 || p.ScanEntries > storage.MaxRetentionScanEntries {
		return &errors.ValidationError{Field: "catalog_retention", Message: "requires positive duration, generation and byte limits, and a bounded entry limit"}
	}
	return nil
}

// WithRetentionPolicy replaces the complete runtime collection policy.
func WithRetentionPolicy(policy RetentionPolicy) Option {
	return func(config *options) error {
		if err := policy.Validate(); err != nil {
			return err
		}
		config.retention = policy
		return nil
	}
}

// WithRetentionEnabled selects automatic collection. Explicit collection remains available.
func WithRetentionEnabled(enabled bool) Option {
	return func(config *options) error { config.retention.Enabled = enabled; return nil }
}

// WithRetentionInterval sets the positive delay between scheduled collection passes.
func WithRetentionInterval(interval time.Duration) Option {
	return func(config *options) error { config.retention.Interval = interval; return nil }
}

// WithRetentionMaxGenerations sets the retained generation count target.
func WithRetentionMaxGenerations(count int) Option {
	return func(config *options) error { config.retention.MaxGenerations = count; return nil }
}

// WithRetentionMaxBytes sets the retained generation byte target.
func WithRetentionMaxBytes(bytes int64) Option {
	return func(config *options) error { config.retention.MaxBytes = bytes; return nil }
}

// WithRetentionScanEntries bounds the entries inspected during one collection pass.
func WithRetentionScanEntries(entries int) Option {
	return func(config *options) error { config.retention.ScanEntries = entries; return nil }
}

// WithRetentionInputMaxBytes bounds raw retained input bytes captured during one scan.
func WithRetentionInputMaxBytes(bytes int64) Option {
	return func(config *options) error { config.retention.InputMaxBytes = bytes; return nil }
}
