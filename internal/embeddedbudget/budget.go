// Package embeddedbudget measures and enforces checked-in catalog budgets.
package embeddedbudget

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/agentstation/starmap/pkg/catalogartifact"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// DefaultMaxAge is the checked-in catalog freshness budget.
	DefaultMaxAge = 30 * 24 * time.Hour
	// DefaultMaxUncompressedBytes is the canonical payload size budget.
	DefaultMaxUncompressedBytes int64 = 16 << 20
	// DefaultMaxCompressedBytes is the deterministic archive size budget.
	DefaultMaxCompressedBytes int64 = 8 << 20
	// DefaultMinProviders is the minimum embedded provider coverage.
	DefaultMinProviders = 5
	// DefaultMinModels is the minimum embedded canonical model coverage.
	DefaultMinModels = 100
)

// Limits are the reviewed age, size, and coverage thresholds.
type Limits struct {
	MaxAge               time.Duration `json:"-"`
	MaxUncompressedBytes int64         `json:"max_uncompressed_bytes"`
	MaxCompressedBytes   int64         `json:"max_compressed_bytes"`
	MinProviders         int           `json:"min_providers"`
	MinModels            int           `json:"min_models"`
}

// MarshalJSON emits the age limit in seconds so CI reports have an explicit,
// language-independent unit rather than time.Duration nanoseconds.
func (l Limits) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{
		"max_age_seconds":        int64(l.MaxAge / time.Second),
		"max_uncompressed_bytes": l.MaxUncompressedBytes,
		"max_compressed_bytes":   l.MaxCompressedBytes,
		"min_providers":          l.MinProviders,
		"min_models":             l.MinModels,
	})
}

// DefaultLimits returns the checked-in production budget policy.
func DefaultLimits() Limits {
	return Limits{
		MaxAge: DefaultMaxAge, MaxUncompressedBytes: DefaultMaxUncompressedBytes,
		MaxCompressedBytes: DefaultMaxCompressedBytes, MinProviders: DefaultMinProviders,
		MinModels: DefaultMinModels,
	}
}

// Validate verifies that every threshold is positive.
func (l Limits) Validate() error {
	if l.MaxAge <= 0 || l.MaxUncompressedBytes <= 0 || l.MaxCompressedBytes <= 0 || l.MinProviders <= 0 || l.MinModels <= 0 {
		return &errors.ValidationError{Field: "embedded_catalog_budget.limits", Value: l, Message: "every threshold must be positive"}
	}
	return nil
}

// Violation is one stable machine-readable budget failure.
type Violation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Report records exact embedded catalog measurements and applied limits.
type Report struct {
	GenerationID      string      `json:"generation_id"`
	GeneratedAt       time.Time   `json:"generated_at"`
	MeasuredAt        time.Time   `json:"measured_at"`
	AgeSeconds        int64       `json:"age_seconds"`
	PayloadChecksum   string      `json:"payload_checksum"`
	UncompressedBytes int64       `json:"uncompressed_bytes"`
	CompressedBytes   int64       `json:"compressed_bytes"`
	ProviderCount     int         `json:"provider_count"`
	ModelCount        int         `json:"model_count"`
	Limits            Limits      `json:"limits"`
	OverrideReason    string      `json:"override_reason,omitempty"`
	Passed            bool        `json:"passed"`
	Violations        []Violation `json:"violations,omitempty"`
}

// Check measures one validated generation and applies age, compressed and
// uncompressed size, provider, and model budgets.
func Check(generation catalogstore.Generation, measuredAt time.Time, limits Limits, overrideReason string) (Report, error) {
	if err := limits.Validate(); err != nil {
		return Report{}, err
	}
	if err := generation.Validate(); err != nil {
		return Report{}, errors.WrapResource("validate", "embedded catalog generation", generation.Manifest.GenerationID, err)
	}
	if measuredAt.IsZero() {
		return Report{}, &errors.ValidationError{Field: "embedded_catalog_budget.measured_at", Message: "is required"}
	}
	measuredAt = measuredAt.UTC()
	artifact, err := catalogartifact.Build(generation)
	if err != nil {
		return Report{}, err
	}
	catalog, err := catalogstore.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		return Report{}, err
	}
	age := measuredAt.Sub(generation.Manifest.GeneratedAt)
	report := Report{
		GenerationID: generation.Manifest.GenerationID, GeneratedAt: generation.Manifest.GeneratedAt,
		MeasuredAt: measuredAt, AgeSeconds: int64(age / time.Second),
		PayloadChecksum:   generation.Manifest.Payload.Checksum,
		UncompressedBytes: int64(len(generation.Payload)), CompressedBytes: int64(len(artifact.Data)),
		ProviderCount: len(catalog.Providers().List()), ModelCount: len(catalog.Definitions()),
		Limits: limits, OverrideReason: overrideReason,
	}
	if age < 0 {
		report.Violations = append(report.Violations, Violation{Code: "generation_future", Message: "embedded generation time is in the future"})
	} else if age > limits.MaxAge {
		report.Violations = append(report.Violations, Violation{Code: "generation_stale", Message: fmt.Sprintf("age %s exceeds %s", age.Round(time.Second), limits.MaxAge)})
	}
	if report.UncompressedBytes > limits.MaxUncompressedBytes {
		report.Violations = append(report.Violations, Violation{Code: "uncompressed_oversize", Message: fmt.Sprintf("%d bytes exceeds %d", report.UncompressedBytes, limits.MaxUncompressedBytes)})
	}
	if report.CompressedBytes > limits.MaxCompressedBytes {
		report.Violations = append(report.Violations, Violation{Code: "compressed_oversize", Message: fmt.Sprintf("%d bytes exceeds %d", report.CompressedBytes, limits.MaxCompressedBytes)})
	}
	if report.ProviderCount < limits.MinProviders {
		report.Violations = append(report.Violations, Violation{Code: "provider_coverage", Message: fmt.Sprintf("%d providers is below %d", report.ProviderCount, limits.MinProviders)})
	}
	if report.ModelCount < limits.MinModels {
		report.Violations = append(report.Violations, Violation{Code: "model_coverage", Message: fmt.Sprintf("%d models is below %d", report.ModelCount, limits.MinModels)})
	}
	report.Passed = len(report.Violations) == 0
	if !report.Passed {
		return report, &errors.ValidationError{
			Field: "embedded_catalog_budget", Value: report.Violations,
			Message: fmt.Sprintf("%d threshold(s) failed", len(report.Violations)),
		}
	}
	return report, nil
}
