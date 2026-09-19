package publication

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/sources"
	catalogruntime "github.com/agentstation/starmap/runtime"
)

func TestReplayAcquisitionNamesExplicitBaseline(t *testing.T) {
	baseline, _ := producerPipelineFixture(t)
	generation := publicationBaselineFixture(t, baseline)
	candidate, err := catalogruntime.ReplayAcquisition(t.Context(), generation, "publisher", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := candidate.Generation("replay", generation.Manifest.GeneratedAt)
	if err != nil {
		t.Fatal(err)
	}
	links := replayed.Manifest.SourceObservations
	if len(links) != 1 || links[0].Source != sources.LocalCatalogID || links[0].EvidenceChecksum != generation.Manifest.Payload.Checksum {
		t.Fatalf("explicit baseline receipt: %+v", links)
	}
}

func TestPreparePublicationCanResumeFromItsPromotedBaseline(t *testing.T) {
	profile, run := admissionFixture(t)
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewState(publicationBaselineFixture(t, empty), "publisher")
	if err != nil {
		t.Fatal(err)
	}
	first, err := preparePublication(t.Context(), state, profile, run, "first")
	if err != nil {
		t.Fatal(err)
	}
	// A later binary embeds the publisher's own accepted membership scopes.
	restarted, err := NewState(first.Next.Generation(), "publisher")
	if err != nil {
		t.Fatal(err)
	}
	next, err := preparePublication(t.Context(), restarted, profile, run, "second")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalogs.DecodeCatalogGeneration(next.Next.Generation()); err != nil {
		t.Fatal(err)
	}
	authority := first.Next.Generation()
	authority.Manifest.AuthorityHead.AuthorityID = "enterprise"
	if _, err := catalogruntime.ReplayAcquisition(t.Context(), authority, "publisher", nil, nil); err == nil {
		t.Fatal("public replay accepted enterprise authority")
	}
}

func TestPreparePublicationRetainsBaselineReviewEvidence(t *testing.T) {
	profile, run := admissionFixture(t)
	observation := run.Attempts[0].Observation
	review := evidence.ReviewCandidate{Code: evidence.ReviewCandidateUnresolvedModelReference,
		ProviderID: "provider", ProviderModelID: "unresolved", SourceID: observation.SourceID,
		SourceObservationID: observation.ID, SourceRevision: observation.Revision, EvidenceChecksum: observation.EvidenceChecksum,
		Reason: "requires a reviewed canonical model reference"}
	candidate, err := starmap.NewCandidate(observation.Catalog, starmap.CandidateEvidence{
		SourceObservations: []catalogs.SourceObservationLink{observation.Link()}, ReviewCandidates: []evidence.ReviewCandidate{review},
	}, starmap.WithCandidateGenerationID("baseline-with-review"))
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := candidate.Generation("baseline-run", run.StartedAt)
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewState(baseline, "publisher")
	if err != nil {
		t.Fatal(err)
	}
	next, err := preparePublication(t.Context(), state, profile, run, "next")
	if err != nil {
		t.Fatal(err)
	}
	if reviews := next.Next.Generation().Manifest.ReviewCandidates; len(reviews) != 1 || reviews[0] != review {
		t.Fatal("replay discarded an unresolved baseline review")
	}
}

func TestPreparePublicationKeepsOmittedModelVisibleWithScopedAbsence(t *testing.T) {
	baseline, profile := producerPipelineFixture(t)
	state, err := NewState(publicationBaselineFixture(t, baseline), "publisher")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	for _, omit := range []bool{false, true} {
		run := Run{StartedAt: at, CompletedAt: at}
		for _, policy := range profile.Scopes {
			attempt := Attempt{Scope: policy.Scope, Outcome: Disabled}
			if policy.Enabled {
				builder, err := catalogs.NewBuilderFrom(baseline)
				if err != nil {
					t.Fatal(err)
				}
				provider, err := builder.Provider("provider")
				if err != nil {
					t.Fatal(err)
				}
				id := policy.Scope.Binding.ID
				provider.Models = map[string]*catalogs.Model{id: provider.Models[id]}
				if omit && id == "two" {
					provider.Models = map[string]*catalogs.Model{}
				}
				if err := builder.SetProvider(provider); err != nil {
					t.Fatal(err)
				}
				catalog, err := builder.Build()
				if err != nil {
					t.Fatal(err)
				}
				observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
					ProviderBinding: policy.Scope.Binding, ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
					Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded})
				if err != nil {
					t.Fatal(err)
				}
				attempt.Outcome, attempt.Observation = Succeeded, &observation
			}
			run.Attempts = append(run.Attempts, attempt)
		}
		prepared, err := preparePublication(t.Context(), state, profile, run, at.Format(time.RFC3339))
		if err != nil {
			t.Fatal(err)
		}
		checkpoint, err := EncodeState(prepared.Next)
		if err != nil {
			t.Fatal(err)
		}
		state, err = RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
		if err != nil {
			t.Fatal(err)
		}
		at = at.Add(time.Minute)
	}
	catalog, err := catalogs.DecodeCatalogGeneration(state.Generation())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Offering("provider", "two"); err != nil {
		t.Fatal("complete omission removed the visible offering", err)
	}
	scopes := catalog.MembershipScopes()
	if len(scopes) != 2 {
		t.Fatal("lost account scope")
	}
	for _, scope := range scopes {
		present, known := scope.Membership(scope.BindingID)
		if !known || present != (scope.BindingID == "one") {
			t.Fatalf("incorrect account membership: %s present=%v known=%v", scope.BindingID, present, known)
		}
	}
}

func TestPreparePublicationPublishesNewReviewsWithoutChangingCatalogFacts(t *testing.T) {
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewState(publicationBaselineFixture(t, empty), "publisher")
	if err != nil {
		t.Fatal(err)
	}
	policy := ScopePolicy{Scope: Scope{Source: sources.ModelsDevHTTPID}, Required: true, Enabled: true, DisabledAction: Preserve}
	profile := Profile{Version: "reviews-v1", Scopes: []ScopePolicy{policy}}
	at := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	var previousChecksum string
	for _, id := range []string{"unresolved-one", "unresolved-two"} {
		builder := catalogs.NewEmpty()
		if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider", Models: map[string]*catalogs.Model{id: {ID: id, Name: id}}}); err != nil {
			t.Fatal(err)
		}
		catalog, err := catalogs.NewObservationCatalog(builder)
		if err != nil {
			t.Fatal(err)
		}
		observation, err := sources.NewObservation(policy.Scope.Source, catalog, sources.ObservationMetadata{
			ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
			Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded})
		if err != nil {
			t.Fatal(err)
		}
		run := Run{StartedAt: at, CompletedAt: at, Attempts: []Attempt{{Scope: policy.Scope, Outcome: Succeeded, Observation: &observation}}}
		prepared, err := preparePublication(t.Context(), state, profile, run, id)
		if err != nil {
			t.Fatal(err)
		}
		if prepared.ReusedArtifact != (previousChecksum != "") {
			t.Fatal("unchanged catalog facts did not reuse the immutable artifact")
		}
		generation := prepared.Next.Generation()
		receipt, err := artifact.DecodePublicationReceipt(prepared.Receipt.Data)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, review := range receipt.Reviews {
			found = found || review.ProviderModelID == id
		}
		if !found {
			t.Fatal("run receipt omitted the new unresolved offering")
		}
		checksum, err := generation.SemanticChecksum()
		if err != nil {
			t.Fatal(err)
		}
		if previousChecksum != "" && checksum != previousChecksum {
			t.Fatal("fixture changed catalog facts")
		}
		previousChecksum = checksum
		checkpoint, err := EncodeState(prepared.Next)
		if err != nil {
			t.Fatal(err)
		}
		state, err = RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
		if err != nil {
			t.Fatal(err)
		}
		at = at.Add(time.Minute)
	}
}
