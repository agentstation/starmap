package publication

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPublicPublicationRetainsStaleMetadataAlongsideFreshProvider(t *testing.T) {
	profile := publicPublicationProfile(t)
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	run := Run{StartedAt: now, CompletedAt: now}
	for _, policy := range profile.Scopes {
		attempt := Attempt{Scope: policy.Scope, Outcome: MissingCredentials}
		if policy.Scope.Source == sources.ModelsDevHTTPID {
			attempt.Outcome = Failed
			run.Retained = append(run.Retained, admissionObservation(t, policy.Scope, now.Add(-7*24*time.Hour)))
		} else if len(run.Attempts) == 1 {
			observation := admissionObservation(t, policy.Scope, now)
			attempt.Outcome, attempt.Observation = Succeeded, &observation
		}
		run.Attempts = append(run.Attempts, attempt)
	}
	decision, err := Admit(profile, run)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || !decision.FreshAcquisition || len(decision.Inputs) != 2 || len(decision.Removals) != 0 {
		t.Fatalf("stale metadata blocked fresh provider evidence: %+v", decision)
	}
	old := run.Retained[0]
	row := decision.Scopes[0]
	if row.EvidenceKind != "stale_retained" || row.Evidence.ObservationID != old.ID || !row.Evidence.ObservedAt.Equal(old.ObservedAt) {
		t.Fatalf("stale evidence lost its status or original receipt: %+v", row)
	}
	_, bundle, semantic := receiptBindingFixture(t, old.Catalog)
	record, err := BindReceipt(t.Context(), profile, run, "stale-metadata", semantic, bundle.Data, bundle.Attestation)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := artifact.DecodePublicationReceipt(record.Data)
	if err != nil || receipt.Sources[0].EvidenceKind != "stale_retained" || receipt.CompletedAt.Sub(receipt.Sources[0].Observation.ObservedAt) != 7*24*time.Hour {
		t.Fatalf("receipt lost stale status or age: %+v, %v", receipt, err)
	}
	fresh := admissionObservation(t, profile.Scopes[0].Scope, now)
	run.Attempts[0].Outcome, run.Attempts[0].Observation = Succeeded, &fresh
	recovered, err := Admit(profile, run)
	if err != nil || !recovered.Allowed || recovered.Scopes[0].EvidenceKind != FreshEvidence || recovered.Scopes[0].Evidence.ObservationID != fresh.ID {
		t.Fatalf("source recovery retained a stale status: %+v, %v", recovered, err)
	}
	if receipt.Sources[0].EvidenceKind != "stale_retained" || !receipt.Sources[0].Observation.ObservedAt.Equal(old.ObservedAt) {
		t.Fatal("source recovery changed the historical receipt")
	}
}
