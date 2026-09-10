package starmap

import (
	"context"
	"github.com/agentstation/starmap/pkg/sources"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestLocalPublicationRequiresOriginalMembershipEvidence(t *testing.T) {
	for _, supplied := range []bool{false, true} {
		name := "missing receipt"
		if supplied {
			name = "original receipt"
		}
		t.Run(name, func(t *testing.T) {
			sourceBuilder := catalogs.NewEmpty()
			if err := sourceBuilder.SetProvider(catalogs.Provider{ID: "openai", Name: "OpenAI"}); err != nil {
				t.Fatal(err)
			}
			sourceCatalog, err := sourceBuilder.Build()
			if err != nil {
				t.Fatal(err)
			}
			observation, err := sources.NewObservation(sources.ProvidersID, sourceCatalog, sources.ObservationMetadata{
				ObservedAt:   time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC),
				Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
				Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
			})
			if err != nil {
				t.Fatal(err)
			}
			link := observation.Link()
			scope := catalogs.ProviderMembershipScope{
				PublisherID: "operator", BindingID: "account", BindingRevision: "1",
				ProviderID: "openai", AccountID: "account", Region: "global", APISurface: "models.list",
				Authority: catalogs.MembershipScopeAuthority,
				Inventory: &catalogs.MembershipInventory{ObservationID: link.ObservationID, ObservedAt: link.ObservedAt, ModelIDs: []string{}},
			}
			builder := catalogs.NewEmpty()
			if err := builder.SetMembershipScopes([]catalogs.ProviderMembershipScope{scope}); err != nil {
				t.Fatal(err)
			}
			catalog, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			evidence := CandidateEvidence{}
			if supplied {
				evidence.SourceObservations = []catalogs.SourceObservationLink{link}
			}
			store := storage.NewMemory()
			client, err := New(WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			before := client.CurrentGenerationID()
			publication, err := client.Update(t.Context(), func(context.Context, *catalogs.Catalog) (*Candidate, error) {
				return NewCandidate(catalog, evidence)
			})
			if supplied {
				if err != nil {
					t.Fatal(err)
				}
				if !publication.Published {
					t.Fatal("valid scoped catalog was not published")
				}
				generation, err := client.CurrentGeneration(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if _, err := catalogs.DecodeCatalogGeneration(generation); err != nil {
					t.Fatal(err)
				}
				restarted, err := New(WithCatalogStore(store))
				if err != nil {
					t.Fatal(err)
				}
				if len(restarted.Catalog().MembershipScopes()) != 1 {
					t.Fatal("restart lost scope evidence")
				}
			} else {
				if err == nil || publication.Published {
					t.Fatal("local publication accepted missing scope evidence")
				}
				if client.CurrentGenerationID() != before {
					t.Fatal("rejected publication changed active generation")
				}
				restarted, err := New(WithCatalogStore(store))
				if err != nil {
					t.Fatal(err)
				}
				if restarted.CurrentGenerationID() != before {
					t.Fatal("rejected publication changed durable generation")
				}
			}
		})
	}
}
