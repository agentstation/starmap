package embeddedbudget

import (
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
)

func TestEmbeddedBudgetRecordsAgeCompressedUncompressedAndCoverage(t *testing.T) {
	generation, err := bootstrap.Generation()
	if err != nil {
		t.Fatalf("bootstrap.Generation: %v", err)
	}
	report, err := Check(generation, generation.Manifest.GeneratedAt.Add(24*time.Hour), DefaultLimits(), "")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Passed || report.AgeSeconds != int64((24*time.Hour)/time.Second) ||
		report.UncompressedBytes != int64(len(generation.Payload)) || report.CompressedBytes <= 0 ||
		report.ProviderCount < DefaultMinProviders || report.ModelCount < DefaultMinModels ||
		report.PayloadChecksum != generation.Manifest.Payload.Checksum {
		t.Fatalf("report = %#v", report)
	}
}

func TestEmbeddedBudgetThresholdBreachesFailWithStableCodes(t *testing.T) {
	generation, err := bootstrap.Generation()
	if err != nil {
		t.Fatalf("bootstrap.Generation: %v", err)
	}
	baseline, err := Check(generation, generation.Manifest.GeneratedAt.Add(time.Hour), DefaultLimits(), "")
	if err != nil {
		t.Fatalf("baseline Check: %v", err)
	}
	tests := []struct {
		name   string
		now    time.Time
		limits Limits
		code   string
	}{
		{name: "future", now: generation.Manifest.GeneratedAt.Add(-time.Second), limits: DefaultLimits(), code: "generation_future"},
		{name: "stale", now: generation.Manifest.GeneratedAt.Add(2 * time.Hour), limits: func() Limits { value := DefaultLimits(); value.MaxAge = time.Hour; return value }(), code: "generation_stale"},
		{name: "uncompressed", now: generation.Manifest.GeneratedAt.Add(time.Hour), limits: func() Limits {
			value := DefaultLimits()
			value.MaxUncompressedBytes = baseline.UncompressedBytes - 1
			return value
		}(), code: "uncompressed_oversize"},
		{name: "compressed", now: generation.Manifest.GeneratedAt.Add(time.Hour), limits: func() Limits {
			value := DefaultLimits()
			value.MaxCompressedBytes = baseline.CompressedBytes - 1
			return value
		}(), code: "compressed_oversize"},
		{name: "providers", now: generation.Manifest.GeneratedAt.Add(time.Hour), limits: func() Limits { value := DefaultLimits(); value.MinProviders = baseline.ProviderCount + 1; return value }(), code: "provider_coverage"},
		{name: "models", now: generation.Manifest.GeneratedAt.Add(time.Hour), limits: func() Limits { value := DefaultLimits(); value.MinModels = baseline.ModelCount + 1; return value }(), code: "model_coverage"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report, err := Check(generation, test.now, test.limits, "reviewed test override")
			if err == nil || report.Passed {
				t.Fatalf("Check passed: %#v", report)
			}
			var validationError *starmaperrors.ValidationError
			if !stderrors.As(err, &validationError) {
				t.Fatalf("error = %T %v, want ValidationError", err, err)
			}
			found := false
			for _, violation := range report.Violations {
				found = found || violation.Code == test.code
			}
			if !found {
				t.Fatalf("violations = %#v, want %q", report.Violations, test.code)
			}
		})
	}
}
