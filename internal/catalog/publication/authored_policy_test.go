package publication

import (
	"bytes"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPublicationAuthoredPolicyRetainsOnlyRequiredCatalogIdentity(t *testing.T) {
	for _, change := range []string{"alias-addition", "operator-removal"} {
		t.Run(change, func(t *testing.T) {
			empty, err := catalogs.NewEmpty().Build()
			if err != nil {
				t.Fatal(err)
			}
			state, err := NewState(publicationBaselineFixture(t, empty), "publisher")
			if err != nil {
				t.Fatal(err)
			}
			observed, _ := producerPipelineFixture(t)
			at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
			policy := ScopePolicy{Scope: Scope{Source: sources.ModelsDevHTTPID}, Required: true, Enabled: true, DisabledAction: Preserve}
			profile := Profile{Version: "authored-policy", Scopes: []ScopePolicy{policy}}
			observation, err := sources.NewObservation(policy.Scope.Source, observed, sources.ObservationMetadata{
				ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
				Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
			})
			if err != nil {
				t.Fatal(err)
			}
			first, err := preparePublication(t.Context(), state, profile, Run{StartedAt: at, CompletedAt: at,
				Attempts: []Attempt{{Scope: policy.Scope, Outcome: Succeeded, Observation: &observation}}}, "policy-source")
			if err != nil {
				t.Fatal(err)
			}
			accepted, err := EncodeState(first.Next)
			if err != nil {
				t.Fatal(err)
			}
			promoted, err := catalogs.DecodeCatalogGeneration(first.Next.Generation())
			if err != nil {
				t.Fatal(err)
			}
			builder, err := catalogs.NewBuilderFrom(promoted)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "alias-addition":
				if err := builder.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{
					{ID: "author/old", TargetID: "author/one", PublisherID: "publisher", State: catalogs.CanonicalAliasActive},
					{ID: "author/older", TargetID: "author/old", PublisherID: "publisher", State: catalogs.CanonicalAliasActive},
				}); err != nil {
					t.Fatal(err)
				}
			case "operator-removal":
				if err := builder.SetRemovalPolicies([]catalogs.CatalogRemovalPolicy{{PublisherID: "publisher", Targets: []catalogs.CatalogRemovalTarget{
					{Kind: catalogs.CatalogRemovalCanonical, DefinitionID: "author/one"},
				}}}); err != nil {
					t.Fatal(err)
				}
			}
			edited, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			replacement := first.Next.Generation()
			replacement.Payload, err = catalogs.EncodeCatalogPayload(edited)
			if err != nil {
				t.Fatal(err)
			}
			replacement.Manifest.Payload = catalogs.DescribeCatalogPayload(replacement.Payload)
			replacement.Manifest.SchemaVersion = catalogs.CatalogPayloadSchemaVersion(edited)
			replacement.Manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{
				MinSchemaVersion: replacement.Manifest.SchemaVersion, MaxSchemaVersion: replacement.Manifest.SchemaVersion,
			}
			profile.Scopes[0].Enabled = false
			profile.Scopes[0].DisabledAction = Remove
			producer, err := NewProducer(profile, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			next, err := producer.PrepareWithBaseline(t.Context(), first.Next, replacement, change)
			if err != nil {
				t.Fatal(err)
			}
			checkpoint, err := EncodeState(next.Next)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
			if err != nil {
				t.Fatal(err)
			}
			after, err := catalogs.DecodeCatalogGeneration(restored.Generation())
			if err != nil {
				t.Fatal(err)
			}
			if len(after.Providers().List()) != 0 || len(restored.history) != 0 || len(after.MembershipScopes()) != 0 {
				t.Error("authored policy retained acquired serving records or history")
			}
			switch change {
			case "alias-addition":
				target, state, found := after.CanonicalAliases().Lookup("author/older")
				if !found || target != "author/one" || state != catalogs.CanonicalAliasActive || len(after.AuthoredModels()) != 1 {
					t.Error("alias lost its required chain or retained unrelated definitions")
				}
			case "operator-removal":
				policies := after.RemovalPolicies()
				if len(policies) != 1 || len(policies[0].Targets) != 1 || policies[0].Targets[0].DefinitionID != "author/one" || len(after.AuthoredModels()) != 0 {
					t.Error("operator policy lost its target or retained acquired definitions")
				}
			}
			unchanged, err := EncodeState(first.Next)
			if err != nil || !bytes.Equal(accepted.Data, unchanged.Data) {
				t.Fatal("authored policy preparation changed the accepted checkpoint", err)
			}
		})
	}
}
