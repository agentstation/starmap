package runtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestOperatorUpstreamPolicySurvivesReconciliationAndRejectsLegacyReplacement(t *testing.T) {
	upstream, _, _, at := upstreamScopeFixture(t)
	original, err := catalogs.DecodeCatalogGeneration(upstream)
	if err != nil {
		t.Fatal(err)
	}
	target, err := catalogs.NewCanonicalRemovalTarget(original.Definitions()[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	builder, err := catalogs.NewBuilderFrom(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.SetRemovalPolicies([]catalogs.CatalogRemovalPolicy{{PublisherID: "upstream-operator", Targets: []catalogs.CatalogRemovalTarget{target}}}); err != nil {
		t.Fatal(err)
	}
	withPolicy := upstream.Copy()
	withPolicy.Payload, err = catalogs.EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	withPolicy.Manifest.GenerationID += "-operator"
	withPolicy.Manifest.Payload = catalogs.DescribeCatalogPayload(withPolicy.Payload)
	if _, err := catalogs.DecodeCatalogGeneration(withPolicy); err != nil {
		t.Fatal(err)
	}
	source := newStubSource("trusted-policy-source")
	source.replies = []SourceRead{{Changed: true, Generation: withPolicy, PublishedAt: at, Health: HealthOK}}
	store := storage.NewMemory()
	options := []Option{WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(store))}
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !connected.Catalog().Removals().ContainsCanonical(target.DefinitionID) {
		t.Fatal("selected upstream policy was lost")
	}
	if _, err := connected.PublishObservations(t.Context(), manualProviderObservation(t, 300, at.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	if !connected.Catalog().Removals().ContainsCanonical(target.DefinitionID) {
		t.Fatal("local reconciliation lost upstream policy")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestRuntime(t, options...)
	if !restarted.Catalog().Removals().ContainsCanonical(target.DefinitionID) {
		t.Fatal("restart lost upstream policy")
	}
	before := restarted.State()
	legacy := upstream.Copy()
	var payload map[string]any
	if err := json.Unmarshal(legacy.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload["schema_version"] = 7
	legacy.Payload, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	legacy.Manifest.SchemaVersion = 7
	legacy.Manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{MinSchemaVersion: 7, MaxSchemaVersion: 7}
	legacy.Manifest.GenerationID += "-legacy-replacement"
	legacy.Manifest.Payload = catalogs.DescribeCatalogPayload(legacy.Payload)
	if _, err := catalogs.DecodeCatalogGeneration(legacy); err != nil {
		t.Fatal(err)
	}
	source.replies = []SourceRead{{Changed: true, Generation: legacy, PublishedAt: at.Add(2 * time.Hour), Health: HealthOK}}
	if _, err := restarted.RefreshSource(t.Context()); !errors.IsConflict(err) {
		t.Fatalf("legacy replacement = %v, want policy-format conflict", err)
	}
	if restarted.State().GenerationID != before.GenerationID || !restarted.Catalog().Removals().ContainsCanonical(target.DefinitionID) {
		t.Fatal("legacy replacement restored an upstream removal")
	}
}

func TestOperatorPolicyRequiresOriginalManifestAndDistinctPublisher(t *testing.T) {
	for _, publisher := range []string{"upstream-operator", "local-runtime", "local-alias"} {
		t.Run(publisher, func(t *testing.T) {
			builder := catalogs.NewEmpty()
			target, err := catalogs.NewCanonicalRemovalTarget("author/retired")
			if err != nil {
				t.Fatal(err)
			}
			if err := builder.SetRemovalPolicies([]catalogs.CatalogRemovalPolicy{{PublisherID: publisher, Targets: []catalogs.CatalogRemovalTarget{target}}}); err != nil {
				t.Fatal(err)
			}
			payload, err := catalogs.EncodeCatalogPayload(builder)
			if err != nil {
				t.Fatal(err)
			}
			layer := sourceLayer{Payload: payload}
			if _, err := layer.decodeCatalog(); !errors.IsValidationError(err) {
				t.Fatalf("policy without original manifest = %v, want validation error", err)
			}
			catalog, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			layers := layerSet{publisherID: "local-runtime", publisherAliases: []string{"local-alias"}}
			err = layers.validateUpstreamScopePublishers(catalog)
			if publisher == "upstream-operator" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.IsConflict(err) {
				t.Fatalf("local publisher claim = %v, want conflict", err)
			}
		})
	}
}
