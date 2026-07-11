package sources

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

type directObservationSource struct {
	catalog *catalogs.Catalog
}

func TestNewObservationProvidesTypedAuditMetadata(t *testing.T) {
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	observedAt := time.Date(2026, time.July, 9, 12, 34, 56, 0, time.UTC)
	observation, err := NewObservation("direct", catalog, ObservationMetadata{
		ObservedAt:   observedAt,
		Revision:     Revision{Kind: RevisionKindContentDigest},
		Completeness: ObservationCompletenessComplete,
		Status:       ObservationStatusSucceeded,
		Records:      ObservationRecordCounts{Accepted: 2},
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	if observation.ID == "" || observation.SourceID != "direct" {
		t.Fatalf("observation identity = (%q, %q)", observation.ID, observation.SourceID)
	}
	if !observation.ObservedAt.Equal(observedAt) || observation.ObservedAt.Location() != time.UTC {
		t.Fatalf("observed at = %v, want %v UTC", observation.ObservedAt, observedAt)
	}
	if observation.Revision.Kind != RevisionKindContentDigest || observation.Revision.Value == "" {
		t.Fatalf("revision = %#v", observation.Revision)
	}
	if observation.EvidenceChecksum == "" || observation.Revision.Value != observation.EvidenceChecksum {
		t.Fatalf("evidence/revision = (%q, %#v)", observation.EvidenceChecksum, observation.Revision)
	}
	if observation.Completeness != ObservationCompletenessComplete || observation.Status != ObservationStatusSucceeded {
		t.Fatalf("state = (%q, %q)", observation.Completeness, observation.Status)
	}
	if observation.Records.Accepted != 2 || observation.Records.Rejected != 0 {
		t.Fatalf("record counts = %#v", observation.Records)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	repeated, err := NewObservation("direct", catalog, ObservationMetadata{
		ObservedAt:   observedAt.Add(time.Second),
		Revision:     Revision{Kind: RevisionKindContentDigest},
		Completeness: ObservationCompletenessComplete,
		Status:       ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("repeated NewObservation: %v", err)
	}
	if repeated.EvidenceChecksum != observation.EvidenceChecksum {
		t.Fatalf("stable catalog checksums differ: %q != %q", repeated.EvidenceChecksum, observation.EvidenceChecksum)
	}
	if repeated.ID == observation.ID {
		t.Fatal("observations at distinct times reused an event identity")
	}

	tamperedCounts := observation
	tamperedCounts.Records.Accepted++
	if err := tamperedCounts.Validate(); err == nil {
		t.Fatal("Validate accepted record-count evidence not bound to observation identity")
	}
	rejectedSuccess := observation
	rejectedSuccess.Records.Rejected = 1
	rejectedSuccess.ID = observationID(rejectedSuccess)
	if err := rejectedSuccess.Validate(); err == nil {
		t.Fatal("Validate accepted rejected records on a complete successful observation")
	}

	tampered := observation
	tampered.EvidenceChecksum = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := tampered.Validate(); err == nil {
		t.Fatal("Validate accepted an evidence checksum that does not describe the catalog")
	}

	partialSuccess := observation
	partialSuccess.Completeness = ObservationCompletenessPartial
	partialSuccess.ID = observationID(partialSuccess)
	if err := partialSuccess.Validate(); err == nil {
		t.Fatal("Validate accepted a partial observation with succeeded status")
	}
}

func TestObservationIssuesDistinguishFailureScopes(t *testing.T) {
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	issues := []ObservationIssue{
		{Scope: ObservationIssueScopeRecord, Code: ObservationIssueCodeInvalidRecord, Subject: "model-a", Message: "invalid record"},
		{Scope: ObservationIssueScopeProvider, Code: ObservationIssueCodeSchemaDrift, Subject: "provider-a", Message: "models changed type"},
		{Scope: ObservationIssueScopeProvider, Code: ObservationIssueCodeFetchFailed, Subject: "provider-a", Message: "provider failed"},
		{Scope: ObservationIssueScopeSource, Code: ObservationIssueCodeFetchFailed, Message: "source failed"},
		{Scope: ObservationIssueScopeStaleFallback, Code: ObservationIssueCodeStaleFallback, Message: "stale cache used"},
	}
	observation, err := NewObservation("direct", catalog, ObservationMetadata{
		ObservedAt:   time.Date(2026, time.July, 9, 12, 34, 56, 0, time.UTC),
		Revision:     Revision{Kind: RevisionKindContentDigest},
		Completeness: ObservationCompletenessPartial,
		Status:       ObservationStatusDegraded,
		Issues:       issues,
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	if !reflect.DeepEqual(observation.Issues, issues) {
		t.Fatalf("issues = %#v, want %#v", observation.Issues, issues)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestObservationRetainsPinnedGitAndLockfileRevision(t *testing.T) {
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	want := Revision{
		Kind: RevisionKindGitCommit, Value: strings.Repeat("a", 40),
		InputName: "bun.lock", InputChecksum: "sha256:" + strings.Repeat("b", 64),
	}
	observation, err := NewObservation(ModelsDevGitID, catalog, ObservationMetadata{
		ObservedAt: time.Date(2026, time.July, 10, 20, 0, 0, 0, time.UTC), Revision: want,
		Completeness: ObservationCompletenessComplete, Status: ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	if observation.Revision != want {
		t.Fatalf("revision = %#v, want %#v", observation.Revision, want)
	}
	if err := observation.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestPartialSourcePolicyMatrix(t *testing.T) {
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	tests := []struct {
		name         string
		issue        ObservationIssue
		completeness ObservationCompleteness
	}{
		{
			name: "record quarantine", completeness: ObservationCompletenessPartial,
			issue: ObservationIssue{Scope: ObservationIssueScopeRecord, Code: ObservationIssueCodeInvalidRecord, Subject: "record-a", Message: "record quarantined"},
		},
		{
			name: "provider unavailable", completeness: ObservationCompletenessPartial,
			issue: ObservationIssue{Scope: ObservationIssueScopeProvider, Code: ObservationIssueCodeFetchFailed, Subject: "provider-a", Message: "provider unavailable"},
		},
		{
			name: "source partial", completeness: ObservationCompletenessPartial,
			issue: ObservationIssue{Scope: ObservationIssueScopeSource, Code: ObservationIssueCodeFetchFailed, Message: "source returned usable partial data"},
		},
		{
			name: "stale fallback remains structurally complete", completeness: ObservationCompletenessComplete,
			issue: ObservationIssue{Scope: ObservationIssueScopeStaleFallback, Code: ObservationIssueCodeStaleFallback, Message: "stale last-known-good data"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation, err := NewObservation("matrix", catalog, ObservationMetadata{
				ObservedAt:   time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC),
				Revision:     Revision{Kind: RevisionKindContentDigest},
				Completeness: test.completeness,
				Status:       ObservationStatusDegraded,
				Issues:       []ObservationIssue{test.issue},
			})
			if err != nil {
				t.Fatalf("NewObservation: %v", err)
			}
			if err := observation.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

func (s directObservationSource) ID() ID                     { return "direct" }
func (s directObservationSource) Name() string               { return "Direct" }
func (s directObservationSource) Cleanup() error             { return nil }
func (s directObservationSource) Dependencies() []Dependency { return nil }
func (s directObservationSource) IsOptional() bool           { return false }
func (s directObservationSource) Observe(context.Context, ...Option) (Observation, error) {
	return NewObservation(s.ID(), s.catalog, ObservationMetadata{
		ObservedAt:   time.Date(2026, time.July, 9, 12, 0, 0, 0, time.UTC),
		Revision:     Revision{Kind: RevisionKindContentDigest},
		Completeness: ObservationCompletenessComplete,
		Status:       ObservationStatusSucceeded,
	})
}

func TestSourceUsesDirectObservationContract(t *testing.T) {
	interfaceType := reflect.TypeFor[Source]()
	if _, found := interfaceType.MethodByName("Observe"); !found {
		t.Fatal("Source has no direct Observe method")
	}
	for _, obsolete := range []string{"Fetch", "Catalog"} {
		if _, found := interfaceType.MethodByName(obsolete); found {
			t.Fatalf("Source still exposes stateful %s ordering", obsolete)
		}
	}

	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var source Source = directObservationSource{catalog: catalog}
	first, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("first Observe: %v", err)
	}
	second, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("second Observe: %v", err)
	}
	if first.SourceID != source.ID() || first.Catalog == nil || second.Catalog == nil {
		t.Fatalf("observations = (%#v, %#v)", first, second)
	}
}
