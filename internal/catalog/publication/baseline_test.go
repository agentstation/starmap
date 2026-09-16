package publication

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestPublicationReplacementBaselineSurvivesCheckpointRestore(t *testing.T) {
	profile, run := admissionFixture(t)
	baseline := authoredPublicationBaseline(t, "original", false, true)
	state, err := NewState(baseline, "publisher")
	if err != nil {
		t.Fatal(err)
	}
	first, err := preparePublication(t.Context(), state, profile, run, "first")
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := EncodeState(first.Next)
	if err != nil {
		t.Fatal(err)
	}
	state, err = RestoreState(t.Context(), accepted.Data, accepted.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	profile.Scopes[0].Enabled = false
	producer, err := NewProducer(profile, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	replacement := authoredPublicationBaseline(t, "replacement", true, true)
	next, err := producer.PrepareWithBaseline(t.Context(), state, replacement, "replacement-run")
	if err != nil {
		t.Fatal(err)
	}
	if next.ReusedArtifact {
		t.Fatal("authored changes reused the old artifact")
	}
	checkpoint, err := EncodeState(next.Next)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := catalogs.DecodeCatalogGeneration(restored.Generation())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.AuthorModel("author", "gone"); err == nil {
		t.Fatal("checkpoint restore resurrected an explicitly removed baseline model")
	}
	if _, err := catalog.AuthorModel("author", "added"); err != nil {
		t.Fatal("checkpoint restore lost an added baseline model", err)
	}
	aliases := catalog.CanonicalAliasRecords()
	if len(aliases) != 1 || aliases[0].State != catalogs.CanonicalAliasRemoved {
		t.Fatal("checkpoint restore lost the explicit alias removal")
	}
	if len(restored.history) != 1 || restored.history[0].ID != run.Attempts[0].Observation.ID {
		t.Fatal("baseline change rewrote original provider evidence")
	}
	unchanged, err := EncodeState(state)
	if err != nil || !bytes.Equal(accepted.Data, unchanged.Data) {
		t.Fatal("preparation changed the accepted checkpoint", err)
	}
}

func TestPublicationReplacementBaselineRejectsInvalidSuccessor(t *testing.T) {
	baseline := authoredPublicationBaseline(t, "original", false, true)
	state, err := NewState(baseline, "publisher")
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := admissionFixture(t)
	profile.Scopes[0].Enabled = false
	producer, err := NewProducer(profile, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := EncodeState(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alias-history-loss", "enterprise-authority", "payload", "canceled"} {
		t.Run(name, func(t *testing.T) {
			replacement := authoredPublicationBaseline(t, "replacement", true, name != "alias-history-loss")
			ctx := t.Context()
			switch name {
			case "enterprise-authority":
				replacement.Manifest.AuthorityHead.AuthorityID = "enterprise"
			case "payload":
				replacement.Payload = []byte("{}")
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			next, err := producer.PrepareWithBaseline(ctx, state, replacement, "rejected")
			if err == nil || next.Next != nil {
				t.Fatal("invalid baseline produced an accepted successor")
			}
			unchanged, err := EncodeState(state)
			if err != nil || !bytes.Equal(accepted.Data, unchanged.Data) {
				t.Fatal("rejected baseline changed accepted state", err)
			}
		})
	}
}

func TestPublicationOwnPromotedBaselineDoesNotMakeLocalFactsPermanent(t *testing.T) {
	catalog, profile := producerPipelineFixture(t)
	baseline := publicationBaselineFixture(t, catalog)
	state, err := NewState(baseline, "publisher")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	run := Run{StartedAt: at, CompletedAt: at}
	for _, policy := range profile.Scopes {
		attempt := Attempt{Scope: policy.Scope, Outcome: Disabled}
		if policy.Enabled {
			observation := publicationInventoryAt(t, catalog, policy.Scope.Binding, 6, at)
			attempt.Outcome, attempt.Observation = Succeeded, &observation
		}
		run.Attempts = append(run.Attempts, attempt)
	}
	first, err := preparePublication(t.Context(), state, profile, run, "provider-update")
	if err != nil {
		t.Fatal(err)
	}
	for index := range profile.Scopes {
		profile.Scopes[index].Enabled = false
		profile.Scopes[index].DisabledAction = Remove
	}
	producer, err := NewProducer(profile, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, err := producer.PrepareWithBaseline(t.Context(), first.Next, first.Next.Generation(), "explicit-removal")
	if err != nil {
		t.Fatal(err)
	}
	after, err := catalogs.DecodeCatalogGeneration(next.Next.Generation())
	if err != nil {
		t.Fatal(err)
	}
	provider, err := after.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	if limits := provider.Models["one"].Limits; limits != nil && limits.ContextWindow == 200 {
		t.Fatal("promoted local facts survived explicit scope removal")
	}
	if len(next.Next.history) != 0 || len(after.MembershipScopes()) != 0 {
		t.Fatal("promoted source scopes survived explicit removal")
	}
}

func authoredPublicationBaseline(t *testing.T, id string, changed, includeAlias bool) catalogs.Generation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(catalogs.Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	second := "gone"
	if changed {
		second = "added"
	}
	for _, model := range []string{"current", second} {
		if err := builder.SetAuthorModel("author", catalogs.Model{ID: model, Name: model, Authors: []catalogs.Author{{ID: "author", Name: "Author"}}}); err != nil {
			t.Fatal(err)
		}
	}
	if includeAlias {
		state := catalogs.CanonicalAliasActive
		if changed {
			state = catalogs.CanonicalAliasRemoved
		}
		if err := builder.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{{ID: "author/old", TargetID: "author/current", PublisherID: "baseline-publisher", State: state}}); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	generation := publicationBaselineFixture(t, catalog)
	generation.Manifest.GenerationID = id
	return generation
}
