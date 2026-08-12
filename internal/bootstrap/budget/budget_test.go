package budget

import (
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
)

func TestEmbeddedBudgetRecordsMeasurementsAndPolicy(t *testing.T) {
	generation, err := bootstrap.Generation()
	if err != nil {
		t.Fatalf("bootstrap.Generation: %v", err)
	}
	report, err := Check(generation, generation.Manifest.GeneratedAt.Add(24*time.Hour), DefaultPolicy())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.Passed || report.SchemaVersion != CurrentPolicyVersion ||
		report.AgeSeconds != int64((24*time.Hour)/time.Second) ||
		report.UncompressedBytes != int64(len(generation.Payload)) || report.CompressedBytes <= 0 ||
		report.ProviderCount <= 0 || report.ModelCount <= 0 ||
		report.PayloadChecksum != generation.Manifest.Payload.Checksum || len(report.Findings) != 0 {
		t.Fatalf("report = %#v", report)
	}
}

func TestCatalogBudgetPolicyClassification(t *testing.T) {
	policy := DefaultPolicy()
	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	classifications := make(map[string]Classification, len(policy.Rules))
	for _, rule := range policy.Rules {
		classifications[rule.Code] = rule.Classification
	}
	want := map[string]Classification{
		ruleGenerationFuture:     ClassificationHardGate,
		ruleGenerationStale:      ClassificationReviewThreshold,
		ruleUncompressedOversize: ClassificationReviewThreshold,
		ruleCompressedOversize:   ClassificationReviewThreshold,
	}
	if len(classifications) != len(want) {
		t.Fatalf("classifications = %#v, want %#v", classifications, want)
	}
	for code, classification := range want {
		if classifications[code] != classification {
			t.Errorf("classification[%q] = %q, want %q", code, classifications[code], classification)
		}
	}
	for _, removed := range []string{"provider_coverage", "model_coverage"} {
		if _, ok := classifications[removed]; ok {
			t.Errorf("unapproved coverage gate %q remains in policy", removed)
		}
	}
}

func TestCatalogBudgetReviewThresholdDoesNotReject(t *testing.T) {
	generation, err := bootstrap.Generation()
	if err != nil {
		t.Fatalf("bootstrap.Generation: %v", err)
	}
	report, err := Check(generation, generation.Manifest.GeneratedAt.Add(reviewMaxAge+time.Second), DefaultPolicy())
	if err != nil {
		t.Fatalf("review threshold rejected release: %v", err)
	}
	if !report.Passed || len(report.Findings) != 1 {
		t.Fatalf("report = %#v, want one non-blocking finding", report)
	}
	finding := report.Findings[0]
	if finding.Code != ruleGenerationStale || finding.Classification != ClassificationReviewThreshold {
		t.Fatalf("finding = %#v", finding)
	}
}

func TestCatalogBudgetHardGateRequiresPolicy(t *testing.T) {
	fields := []struct {
		name  string
		clear func(*Rule)
	}{
		{name: "objective", clear: func(rule *Rule) { rule.Objective = "" }},
		{name: "measurement method", clear: func(rule *Rule) { rule.MeasurementMethod = "" }},
		{name: "unit", clear: func(rule *Rule) { rule.Unit = "" }},
		{name: "approved limit", clear: func(rule *Rule) { rule.ApprovedLimit = "" }},
		{name: "consequence", clear: func(rule *Rule) { rule.Consequence = "" }},
		{name: "owner", clear: func(rule *Rule) { rule.Owner = "" }},
		{name: "exception path", clear: func(rule *Rule) { rule.ExceptionPath = "" }},
		{name: "reopen condition", clear: func(rule *Rule) { rule.ReopenCondition = "" }},
	}
	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			policy := DefaultPolicy()
			for index := range policy.Rules {
				if policy.Rules[index].Classification == ClassificationHardGate {
					field.clear(&policy.Rules[index])
					break
				}
			}
			if err := policy.Validate(); err == nil {
				t.Fatalf("policy accepted a hard gate without %s", field.name)
			}
		})
	}

	generation, err := bootstrap.Generation()
	if err != nil {
		t.Fatalf("bootstrap.Generation: %v", err)
	}
	report, err := Check(generation, generation.Manifest.GeneratedAt.Add(-time.Second), DefaultPolicy())
	if err == nil || report.Passed || len(report.Findings) != 1 {
		t.Fatalf("future generation result = %#v, %v", report, err)
	}
	if finding := report.Findings[0]; finding.Code != ruleGenerationFuture || finding.Classification != ClassificationHardGate {
		t.Fatalf("finding = %#v", finding)
	}
	var validationError *starmaperrors.ValidationError
	if !stderrors.As(err, &validationError) {
		t.Fatalf("error = %T %v, want ValidationError", err, err)
	}
}
