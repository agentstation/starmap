package starmap

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCustomUpdateRetainsMembershipEvidence(t *testing.T) {
	for _, mode := range []string{"unchanged", "new evidence", "changed inventory", "removed scope", "changed receipt", "changed with unrelated evidence"} {
		t.Run(mode, func(t *testing.T) {
			store := storage.NewMemory()
			initial := membershipGeneration(t)
			if err := store.Commit(t.Context(), initial, ""); err != nil {
				t.Fatal(err)
			}
			client, err := NewContext(t.Context(), WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			before := client.CurrentCatalogState()
			publication, err := client.Update(t.Context(), func(_ context.Context, current *catalogs.Catalog) (*Candidate, error) {
				builder, err := catalogs.NewBuilderFrom(current)
				if err != nil {
					return nil, err
				}
				scopes := builder.MembershipScopes()
				switch mode {
				case "changed inventory", "changed with unrelated evidence":
					scopes[0].Inventory.ModelIDs = []string{"replacement"}
				case "removed scope":
					scopes = nil
				case "changed receipt":
					scopes[0].Inventory.ObservationID = "invented-receipt"
				}
				if err := builder.SetMembershipScopes(scopes); err != nil {
					return nil, err
				}
				if err := builder.SetProvider(catalogs.Provider{ID: "custom", Name: "Custom"}); err != nil {
					return nil, err
				}
				catalog, err := builder.Build()
				if err != nil {
					return nil, err
				}
				evidence := CandidateEvidence{}
				if mode == "new evidence" || mode == "changed with unrelated evidence" {
					link := initial.Manifest.SourceObservations[0]
					link.Source = sources.LocalCatalogID
					link.ObservationID = "unrelated-local-observation"
					evidence.SourceObservations = []catalogs.SourceObservationLink{link}
				}
				return NewCandidate(catalog, evidence)
			})
			if mode != "unchanged" && mode != "new evidence" {
				if err == nil || publication.Published {
					t.Fatalf("membership mutation without evidence succeeded: %+v", publication)
				}
				stored, getErr := store.Current(t.Context())
				if getErr != nil || stored.Manifest.GenerationID != initial.Manifest.GenerationID || client.CurrentCatalogState().GenerationID != before.GenerationID {
					t.Fatalf("rejection changed committed or active state: %v", getErr)
				}
				return
			}
			if err != nil || !publication.Published {
				t.Fatalf("custom update: %+v, %v", publication, err)
			}
			stored, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			links := stored.Manifest.SourceObservations
			if len(links) != 2 {
				t.Fatalf("source observations = %+v, want two", links)
			}
			originalIndex := 0
			if mode == "new evidence" {
				originalIndex = 1
			} else if links[1].Source != customUpdateSourceID || links[1].EvidenceChecksum != stored.Manifest.Payload.Checksum {
				t.Fatalf("custom update evidence lost: %+v", links)
			}
			if !reflect.DeepEqual(links[originalIndex], initial.Manifest.SourceObservations[0]) {
				t.Fatalf("original receipt changed: %+v", links)
			}
			restarted, err := NewContext(t.Context(), WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(restarted.Catalog().MembershipScopes(), before.Catalog.MembershipScopes()) {
				t.Fatal("restart changed provider membership")
			}
			if _, err := restarted.Catalog().Provider("custom"); err != nil {
				t.Fatal(err)
			}
			if mode == "new evidence" {
				return
			}
			if _, err := restarted.Update(t.Context(), func(_ context.Context, current *catalogs.Catalog) (*Candidate, error) {
				return NewCandidate(current, CandidateEvidence{})
			}); err != nil {
				t.Fatal(err)
			}
			repeated, err := store.Current(t.Context())
			if err != nil || len(repeated.Manifest.SourceObservations) != 2 {
				t.Fatalf("repeated custom updates accumulate receipts: %+v, %v", repeated.Manifest.SourceObservations, err)
			}
		})
	}
}

func membershipGeneration(t *testing.T) catalogs.Generation {
	t.Helper()
	at := time.Date(2026, time.September, 17, 0, 0, 0, 0, time.UTC)
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider"}); err != nil {
		t.Fatal(err)
	}
	observed, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, observed, sources.ObservationMetadata{
		ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.SetMembershipScopes([]catalogs.ProviderMembershipScope{{
		PublisherID: "publisher", BindingID: "binding", BindingRevision: "revision",
		ProviderID: "provider", Region: "global", APISurface: "models", Public: true,
		Authority: catalogs.MembershipProviderAuthority,
		Inventory: &catalogs.MembershipInventory{ObservationID: observation.ID, ObservedAt: at, ModelIDs: []string{"model"}},
	}}); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	generation, err := generationTestClient(at).newGeneration(catalog, CandidateEvidence{SourceObservations: []catalogs.SourceObservationLink{observation.Link()}}, "")
	if err != nil {
		t.Fatal(err)
	}
	return generation
}
