package publication

import (
	"bytes"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPublicationQuarantinePreservesRejectedRecordsAndClearsAfterRecovery(t *testing.T) {
	baseline, _ := producerPipelineFixture(t)
	state, err := NewState(publicationBaselineFixture(t, baseline), "quarantine-test")
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{Source: sources.ModelsDevHTTPID}
	profile := Profile{Version: "quarantine-v1", Scopes: []ScopePolicy{{Scope: scope, Required: true, Enabled: true,
		AllowRecordQuarantine: true, MaxRetainedAge: 24 * time.Hour, DisabledAction: Preserve}}}
	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	var quarantinedID string
	var firstReceipt []byte
	for step := range 4 {
		at = at.Add(time.Hour)
		builder, err := catalogs.NewBuilderFrom(baseline)
		if err != nil {
			t.Fatal(err)
		}
		provider, err := builder.Provider("provider")
		if err != nil {
			t.Fatal(err)
		}
		provider.Models["one"].Limits = &catalogs.ModelLimits{ContextWindow: int64(100 + step)}
		if step == 1 {
			delete(provider.Models, "two")
		}
		if err := builder.SetProvider(provider); err != nil {
			t.Fatal(err)
		}
		catalog, err := builder.Build()
		if err != nil {
			t.Fatal(err)
		}
		metadata := sources.ObservationMetadata{ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
			Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
			Records: sources.ObservationRecordCounts{Accepted: 3}}
		outcome := Succeeded
		if step == 1 {
			outcome = Partial
			metadata.Completeness, metadata.Status = sources.ObservationCompletenessPartial, sources.ObservationStatusDegraded
			metadata.Records = sources.ObservationRecordCounts{Accepted: 2, Rejected: 1}
			metadata.Issues = []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord,
				Subject: "provider/two", Message: "PRIVATE_DIAGNOSTIC_MUST_NOT_BE_PUBLISHED"}}
		}
		observation, err := sources.NewObservation(scope.Source, catalog, metadata)
		if err != nil {
			t.Fatal(err)
		}
		attempt := Attempt{Scope: scope, Outcome: outcome, Observation: &observation}
		if step == 2 {
			attempt.Outcome, attempt.Observation = Failed, nil
		}
		retained, err := retainedForProfile(profile, state.history)
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := preparePublication(t.Context(), state, profile, Run{StartedAt: at, CompletedAt: at,
			Attempts: []Attempt{attempt}, Retained: retained}, at.Format(time.RFC3339))
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		receipt, err := artifact.DecodePublicationReceipt(prepared.Receipt.Data)
		if err != nil {
			t.Fatal(err)
		}
		report := receipt.Sources[0]
		if step == 1 || step == 2 {
			if report.Quarantine == nil || report.Quarantine.Records.Rejected != 1 || report.Quarantine.Issues[0].Subject != "provider/two" ||
				report.Observation.Status != sources.ObservationStatusDegraded || report.Observation.Completeness != sources.ObservationCompletenessPartial {
				t.Fatal("quarantine diagnostics or degraded source status were lost")
			}
			if step == 1 {
				quarantinedID = report.Observation.ObservationID
				firstReceipt = bytes.Clone(prepared.Receipt.Data)
			} else if report.Observation.ObservationID != quarantinedID || report.EvidenceKind != "retained" || receipt.FreshAcquisition || !prepared.ReusedArtifact {
				t.Fatal("outage invented fresh evidence or replaced the accepted artifact")
			}
		} else if report.Quarantine != nil || report.Observation.Status != sources.ObservationStatusSucceeded {
			t.Fatal("a complete update retained the active quarantine flag")
		}
		checkpoint, err := EncodeState(prepared.Next)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(checkpoint.Data, []byte("PRIVATE_DIAGNOSTIC")) || bytes.Contains(prepared.Receipt.Data, []byte("PRIVATE_DIAGNOSTIC")) {
			t.Fatal("private diagnostic message entered a public object")
		}
		state, err = RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
		if err != nil {
			t.Fatal(err)
		}
		accepted, err := catalogs.DecodeCatalogGeneration(state.Generation())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := accepted.Offering("provider", "two"); err != nil {
			t.Fatal("quarantine removed the last accepted record", err)
		}
		acceptedProvider, err := accepted.Provider("provider")
		if err != nil {
			t.Fatal(err)
		}
		want := int64(100 + step)
		if step == 2 {
			want = 101
		}
		if acceptedProvider.Models["one"].Limits.ContextWindow != want {
			t.Fatal("quarantine discarded a valid sibling update")
		}
	}
	historical, err := artifact.DecodePublicationReceipt(firstReceipt)
	if err != nil || historical.Sources[0].Quarantine == nil {
		t.Fatal("recovery erased the immutable quarantine record", err)
	}
}

func TestPublicationQuarantineRequiresExplicitPolicyAndOnlyRecordFailures(t *testing.T) {
	profile, run := admissionFixture(t)
	original := *run.Attempts[0].Observation
	for _, test := range []struct {
		name     string
		allow    bool
		code     sources.ObservationIssueCode
		accepted int
		want     bool
	}{
		{"disabled", false, sources.ObservationIssueCodeInvalidRecord, 1, false},
		{"isolated record", true, sources.ObservationIssueCodeInvalidRecord, 1, true},
		{"no valid data", true, sources.ObservationIssueCodeInvalidRecord, 0, false},
		{"fetch failure", true, sources.ObservationIssueCodeFetchFailed, 1, false},
		{"truncation", true, sources.ObservationIssueCodePayloadLimit, 1, false},
		{"stale fallback", true, sources.ObservationIssueCodeStaleFallback, 1, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile.Scopes[0].AllowRecordQuarantine = test.allow
			observation, err := sources.NewObservation(original.SourceID, original.Catalog, sources.ObservationMetadata{
				ProviderBinding: original.ProviderBinding, ObservedAt: original.ObservedAt, Revision: original.Revision,
				Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
				Records: sources.ObservationRecordCounts{Accepted: test.accepted, Rejected: 1},
				Issues:  []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: test.code, Subject: "provider/model", Message: "fixture"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			run.Attempts[0].Outcome, run.Attempts[0].Observation = Partial, &observation
			decision, err := Admit(profile, run)
			if err != nil || decision.Allowed != test.want {
				t.Fatalf("allowed=%t want=%t error=%v", decision.Allowed, test.want, err)
			}
		})
	}
}

func TestPublicationRepeatedQuarantineRetainsBoundedOriginalEvidence(t *testing.T) {
	baseline, _ := producerPipelineFixture(t)
	state, err := NewState(publicationBaselineFixture(t, baseline), "repeated-quarantine")
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{Source: sources.ModelsDevHTTPID}
	profile := Profile{Version: "quarantine-v1", Scopes: []ScopePolicy{{Scope: scope, Required: true, Enabled: true,
		AllowRecordQuarantine: true, MaxRetainedAge: 24 * time.Hour, DisabledAction: Preserve}}}
	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	for step := range 7 {
		at = at.Add(time.Hour)
		observation, err := sources.NewObservation(scope.Source, baseline, sources.ObservationMetadata{
			ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
			Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
			Records: sources.ObservationRecordCounts{Accepted: 3, Rejected: 1},
			Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord,
				Subject: "provider/unpublished", Message: "invalid record"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := preparePublication(t.Context(), state, profile, Run{StartedAt: at, CompletedAt: at,
			Attempts: []Attempt{{Scope: scope, Outcome: Partial, Observation: &observation}}}, at.Format(time.RFC3339))
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		checkpoint, err := EncodeState(prepared.Next)
		if err != nil {
			t.Fatal(err)
		}
		state, err = RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
		if err != nil {
			t.Fatal(err)
		}
		if len(state.history) > 2 || state.history[len(state.history)-1].ID != observation.ID {
			t.Fatalf("step %d retained %d observations or lost current diagnostics", step, len(state.history))
		}
	}
}
