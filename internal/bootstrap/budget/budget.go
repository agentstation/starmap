// Package budget measures the checked-in catalog and applies release policy.
package budget

import (
	"fmt"
	"time"

	"github.com/agentstation/starmap/pkg/catalogartifact"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// CurrentPolicyVersion identifies the machine-readable release policy shape.
	CurrentPolicyVersion uint64 = 1

	reviewMaxAge               = 30 * 24 * time.Hour
	reviewMaxUncompressedBytes = int64(16 << 20)
	reviewMaxCompressedBytes   = int64(8 << 20)
)

// Classification controls whether a policy finding blocks a release.
type Classification string

const (
	// ClassificationHardGate blocks a release when a correctness invariant fails.
	ClassificationHardGate Classification = "hard_gate"
	// ClassificationReviewThreshold records a review trigger without rejecting a release.
	ClassificationReviewThreshold Classification = "review_threshold"
)

const (
	ruleGenerationFuture     = "generation_future"
	ruleGenerationStale      = "generation_stale"
	ruleUncompressedOversize = "uncompressed_oversize"
	ruleCompressedOversize   = "compressed_oversize"
)

// Rule defines one classified release-policy decision.
type Rule struct {
	Code              string         `json:"code"`
	Classification    Classification `json:"classification"`
	Objective         string         `json:"objective"`
	MeasurementMethod string         `json:"measurement_method"`
	Unit              string         `json:"unit"`
	ApprovedLimit     string         `json:"approved_limit"`
	Consequence       string         `json:"consequence"`
	Owner             string         `json:"owner"`
	ExceptionPath     string         `json:"exception_path"`
	ReopenCondition   string         `json:"reopen_condition"`
}

// Policy is the versioned catalog release policy applied to one measurement.
type Policy struct {
	Version uint64 `json:"version"`
	Rules   []Rule `json:"rules"`
}

// DefaultPolicy returns the reviewed catalog release policy.
func DefaultPolicy() Policy {
	return Policy{
		Version: CurrentPolicyVersion,
		Rules: []Rule{
			{
				Code: ruleGenerationFuture, Classification: ClassificationHardGate,
				Objective:         "preserve valid generation chronology",
				MeasurementMethod: "compare the manifest generated_at value with the UTC measurement time",
				Unit:              "UTC timestamp", ApprovedLimit: "generated_at must not be after measured_at",
				Consequence: "block the release because future dating invalidates freshness evidence",
				Owner:       "Starmap release engineering", ExceptionPath: "correct and regenerate the catalog; no runtime bypass",
				ReopenCondition: "an approved release policy defines a bounded clock-skew tolerance",
			},
			{
				Code: ruleGenerationStale, Classification: ClassificationReviewThreshold,
				Objective:         "make catalog freshness drift visible before release",
				MeasurementMethod: "subtract manifest generated_at from the UTC measurement time",
				Unit:              "seconds", ApprovedLimit: fmt.Sprintf("review when age exceeds %d seconds", int64(reviewMaxAge/time.Second)),
				Consequence: "record a review finding; do not reject the release without a separate approved hard budget",
				Owner:       "Starmap catalog maintainers", ExceptionPath: "record the freshness disposition in release review",
				ReopenCondition: "an approved availability or freshness objective establishes a hard maximum age",
			},
			{
				Code: ruleUncompressedOversize, Classification: ClassificationReviewThreshold,
				Objective:         "make canonical payload growth visible before release",
				MeasurementMethod: "count bytes in the verified canonical catalog payload",
				Unit:              "bytes", ApprovedLimit: fmt.Sprintf("review when size exceeds %d bytes", reviewMaxUncompressedBytes),
				Consequence: "record a review finding; do not reject the release without a separate approved hard budget",
				Owner:       "Starmap release engineering", ExceptionPath: "record the payload-growth disposition in release review",
				ReopenCondition: "an approved startup, memory, or distribution objective establishes a hard payload limit",
			},
			{
				Code: ruleCompressedOversize, Classification: ClassificationReviewThreshold,
				Objective:         "make distribution artifact growth visible before release",
				MeasurementMethod: "count bytes in the deterministic compressed catalog artifact",
				Unit:              "bytes", ApprovedLimit: fmt.Sprintf("review when size exceeds %d bytes", reviewMaxCompressedBytes),
				Consequence: "record a review finding; do not reject the release without a separate approved hard budget",
				Owner:       "Starmap release engineering", ExceptionPath: "record the archive-growth disposition in release review",
				ReopenCondition: "an approved download, storage, or distribution objective establishes a hard archive limit",
			},
		},
	}
}

// Validate rejects an incomplete or unsupported policy before measurement.
func (p Policy) Validate() error {
	if p.Version != CurrentPolicyVersion {
		return &errors.ValidationError{Field: "embedded_catalog_policy.version", Value: p.Version, Message: "is not supported"}
	}
	expected := map[string]Classification{
		ruleGenerationFuture:     ClassificationHardGate,
		ruleGenerationStale:      ClassificationReviewThreshold,
		ruleUncompressedOversize: ClassificationReviewThreshold,
		ruleCompressedOversize:   ClassificationReviewThreshold,
	}
	if len(p.Rules) != len(expected) {
		return &errors.ValidationError{Field: "embedded_catalog_policy.rules", Value: len(p.Rules), Message: "must contain each supported rule exactly once"}
	}
	seen := make(map[string]struct{}, len(p.Rules))
	for index, rule := range p.Rules {
		classification, ok := expected[rule.Code]
		if !ok || rule.Classification != classification {
			return &errors.ValidationError{Field: fmt.Sprintf("embedded_catalog_policy.rules[%d].classification", index), Value: rule, Message: "does not match the supported rule classification"}
		}
		if _, duplicate := seen[rule.Code]; duplicate {
			return &errors.ValidationError{Field: fmt.Sprintf("embedded_catalog_policy.rules[%d].code", index), Value: rule.Code, Message: "is duplicated"}
		}
		seen[rule.Code] = struct{}{}
		if rule.Objective == "" || rule.MeasurementMethod == "" || rule.Unit == "" || rule.ApprovedLimit == "" ||
			rule.Consequence == "" || rule.Owner == "" || rule.ExceptionPath == "" || rule.ReopenCondition == "" {
			return &errors.ValidationError{Field: fmt.Sprintf("embedded_catalog_policy.rules[%d]", index), Value: rule, Message: "must define all policy fields"}
		}
	}
	return nil
}

// Finding records one measurement that crossed a classified policy boundary.
type Finding struct {
	Code           string         `json:"code"`
	Classification Classification `json:"classification"`
	Message        string         `json:"message"`
}

// Report records exact catalog measurements and the policy applied to them.
type Report struct {
	SchemaVersion     uint64    `json:"schema_version"`
	GenerationID      string    `json:"generation_id"`
	GeneratedAt       time.Time `json:"generated_at"`
	MeasuredAt        time.Time `json:"measured_at"`
	AgeSeconds        int64     `json:"age_seconds"`
	PayloadChecksum   string    `json:"payload_checksum"`
	UncompressedBytes int64     `json:"uncompressed_bytes"`
	CompressedBytes   int64     `json:"compressed_bytes"`
	ProviderCount     int       `json:"provider_count"`
	ModelCount        int       `json:"model_count"`
	Policy            Policy    `json:"policy"`
	Passed            bool      `json:"passed"`
	Findings          []Finding `json:"findings,omitempty"`
}

// Check measures one validated generation and applies the supplied policy.
func Check(generation catalogstore.Generation, measuredAt time.Time, policy Policy) (Report, error) {
	if err := policy.Validate(); err != nil {
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
		SchemaVersion: CurrentPolicyVersion, GenerationID: generation.Manifest.GenerationID,
		GeneratedAt: generation.Manifest.GeneratedAt, MeasuredAt: measuredAt,
		AgeSeconds: int64(age / time.Second), PayloadChecksum: generation.Manifest.Payload.Checksum,
		UncompressedBytes: int64(len(generation.Payload)), CompressedBytes: int64(len(artifact.Data)),
		ProviderCount: len(catalog.Providers().List()), ModelCount: len(catalog.Definitions()), Policy: policy,
	}
	if age < 0 {
		report.Findings = append(report.Findings, finding(policy, ruleGenerationFuture, "embedded generation time is in the future"))
	} else if age > reviewMaxAge {
		report.Findings = append(report.Findings, finding(policy, ruleGenerationStale, fmt.Sprintf("age %s exceeds review threshold %s", age.Round(time.Second), reviewMaxAge)))
	}
	if report.UncompressedBytes > reviewMaxUncompressedBytes {
		report.Findings = append(report.Findings, finding(policy, ruleUncompressedOversize, fmt.Sprintf("%d bytes exceeds review threshold %d", report.UncompressedBytes, reviewMaxUncompressedBytes)))
	}
	if report.CompressedBytes > reviewMaxCompressedBytes {
		report.Findings = append(report.Findings, finding(policy, ruleCompressedOversize, fmt.Sprintf("%d bytes exceeds review threshold %d", report.CompressedBytes, reviewMaxCompressedBytes)))
	}
	report.Passed = true
	for _, assessment := range report.Findings {
		if assessment.Classification == ClassificationHardGate {
			report.Passed = false
			return report, &errors.ValidationError{
				Field: "embedded_catalog_budget", Value: report.Findings,
				Message: "hard catalog release gate failed",
			}
		}
	}
	return report, nil
}

func finding(policy Policy, code, message string) Finding {
	for _, rule := range policy.Rules {
		if rule.Code == code {
			return Finding{Code: code, Classification: rule.Classification, Message: message}
		}
	}
	panic("validated catalog policy is missing rule " + code)
}
