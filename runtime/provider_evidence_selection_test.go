package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestProviderEvidenceSelectionKeepsUnresolvedReceipts(t *testing.T) {
	at := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	prior := manualProviderObservation(t, 200, at)
	for _, scenario := range []struct {
		name    string
		review  bool
		partial bool
		when    time.Time
	}{
		{name: "review", review: true, when: at.Add(time.Minute)},
		{name: "partial", partial: true, when: at.Add(time.Minute)},
		{name: "same-time", when: at},
		{name: "older", when: at.Add(-time.Minute)},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			current := manualProviderObservation(t, 300, scenario.when)
			if scenario.partial {
				var err error
				current, err = sources.NewObservation(sources.ProvidersID, current.Catalog, sources.ObservationMetadata{
					ObservedAt: scenario.when, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
					Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
					Issues: []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/invalid", Message: "A source record is invalid."}},
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			collected := starmap.CandidateEvidence{SourceObservations: []catalogs.SourceObservationLink{prior.Link(), current.Link()}}
			if scenario.review {
				collected.ReviewCandidates = []evidence.ReviewCandidate{{SourceObservationID: prior.ID}}
			}
			if err := selectCurrentProviderEvidence(t.Context(), catalogs.NewEmpty(), []sources.Observation{prior, current}, &observationResetSelection{}, &collected); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, link := range collected.SourceObservations {
				found = found || link.ObservationID == prior.ID
			}
			if !found {
				t.Fatal("unresolved prior receipt was removed")
			}
		})
	}
}

func TestProviderEvidenceSelectionHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := selectCurrentProviderEvidence(ctx, catalogs.NewEmpty(), nil, &observationResetSelection{}, &starmap.CandidateEvidence{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled selection = %v", err)
	}
}
