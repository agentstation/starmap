package runtime

import (
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestBaselineReceiptPreservesVerifiedCatalogBoundary(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "embedded alias policy", true: "unchanged remote"}[remote], func(t *testing.T) {
			var aliases []catalogs.CanonicalAlias
			if !remote {
				aliases = append(aliases, activeAlias("author/old"))
			}
			generation := aliasGeneration(t, "selected-baseline", aliases...)
			catalog, err := catalogs.DecodeCatalogGeneration(generation)
			if err != nil {
				t.Fatal(err)
			}
			baseline := starmap.CatalogState{Catalog: catalog, GenerationID: generation.Manifest.GenerationID,
				PayloadChecksum: generation.Manifest.Payload.Checksum, GeneratedAt: generation.Manifest.GeneratedAt}
			layers := layerSet{embedded: baseline, embeddedManifest: &generation.Manifest}
			if remote {
				layers.source = &sourceLayer{Manifest: &generation.Manifest, Identity: "remote", GenerationID: baseline.GenerationID,
					Checksum: baseline.PayloadChecksum, Payload: generation.Payload}
			} else {
				layers.providerBindings = &providerBindingPolicy{}
			}
			state, err := layers.build(t.Context(), baseline)
			if err != nil {
				t.Fatal(err)
			}
			if state.PayloadChecksum != baseline.PayloadChecksum || !reflect.DeepEqual(state.Catalog.CanonicalAliasRecords(), catalog.CanonicalAliasRecords()) {
				t.Fatal("baseline rebuild changed verified catalog content")
			}
			if remote {
				if state.GenerationID != baseline.GenerationID || len(layers.buildEvidence.SourceObservations) != 0 {
					t.Fatal("unchanged remote catalog acquired local evidence or identity")
				}
			} else {
				links := layers.buildEvidence.SourceObservations
				if len(links) != 1 || links[0].Source != sources.EmbeddedCatalogID || links[0].EvidenceChecksum != baseline.PayloadChecksum {
					t.Fatalf("compiled baseline receipt: %+v", links)
				}
				if err := links[0].Validate(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestEmbeddedBaselineStartupPreservesGenerationEvidence(t *testing.T) {
	baseline, err := starmap.EmbeddedGeneration()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := catalogs.DecodeCatalogGeneration(baseline)
	if err != nil {
		t.Fatal(err)
	}
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "no local policy", true: "empty local bindings"}[explicit], func(t *testing.T) {
			store := storage.NewMemory()
			options := []Option{WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store))}
			if explicit {
				options = append(options, WithProviderBindings())
			}
			var first starmap.CatalogState
			for attempt := range 2 {
				connected := openTestRuntime(t, options...)
				state := connected.State()
				if !reflect.DeepEqual(state.Catalog.MembershipScopes(), catalog.MembershipScopes()) {
					t.Fatal("local binding policy changed compiled publisher scopes")
				}
				accepted, err := connected.Client().CurrentGeneration(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if _, err := catalogs.DecodeCatalogGeneration(accepted); err != nil {
					t.Fatalf("startup lost original membership evidence: %v", err)
				}
				if attempt == 0 {
					first = state
				} else if state.GenerationID != first.GenerationID || state.PayloadChecksum != first.PayloadChecksum {
					t.Fatal("restart changed baseline identity or bytes")
				}
				if err := connected.Close(); err != nil {
					t.Fatal(err)
				}
				if attempt == 0 && !explicit {
					if err := store.Commit(t.Context(), accepted, ""); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestCompiledScopeEvidenceSurvivesLayerRebuild(t *testing.T) {
	generation, observation, expected, _ := upstreamScopeFixture(t)
	catalog, err := catalogs.DecodeCatalogGeneration(generation)
	if err != nil {
		t.Fatal(err)
	}
	baseline := starmap.CatalogState{
		Catalog: catalog, GenerationID: generation.Manifest.GenerationID,
		PayloadChecksum: generation.Manifest.Payload.Checksum, GeneratedAt: generation.Manifest.GeneratedAt,
	}
	layers := layerSet{embedded: baseline, embeddedManifest: &generation.Manifest}
	state, err := layers.build(t.Context(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state.Catalog.MembershipScopes(), expected) {
		t.Fatal("rebuild changed compiled membership scopes")
	}
	if !reflect.DeepEqual(layers.buildEvidence.SourceObservations, []catalogs.SourceObservationLink{observation.Link()}) {
		t.Fatal("rebuild lost the original provider observation")
	}
	layers.embeddedManifest = nil
	if _, err := layers.build(t.Context(), baseline); err == nil {
		t.Fatal("rebuild accepted scopes without original evidence")
	}
}

func TestStoredScopedStateMustMatchExactCompiledBaseline(t *testing.T) {
	store, _ := acceptedScopedStartupStore(t)
	client, err := starmap.NewContext(t.Context(), starmap.WithCatalogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	baseline := client.CurrentCatalogState()
	for name, mutate := range map[string]func(*starmap.CatalogState){
		"exact compiled state": func(*starmap.CatalogState) {},
		"other identity":       func(s *starmap.CatalogState) { s.GenerationID += "-other" },
		"other payload":        func(s *starmap.CatalogState) { s.PayloadChecksum = "other" },
		"other time":           func(s *starmap.CatalogState) { s.GeneratedAt = s.GeneratedAt.Add(time.Second) },
		"other authority":      func(s *starmap.CatalogState) { s.AuthorityHead.AuthorityID = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			state := baseline
			mutate(&state)
			if required := storedProviderPolicyRequired(state, baseline); required != (name != "exact compiled state") {
				t.Fatalf("binding policy required=%v", required)
			}
		})
	}
	if !storedProviderPolicyRequired(baseline, starmap.CatalogState{}) {
		t.Fatal("unknown baseline accepted stored provider scopes")
	}
}

func TestOriginBootstrapRetainsEmbeddedEvidence(t *testing.T) {
	baseline, err := starmap.EmbeddedGeneration()
	if err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithAuthorityOrigin(storage.NewMemory(), originTestConfig()))
	accepted, err := r.Client().CurrentGeneration(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(accepted.Manifest.SourceObservations, baseline.Manifest.SourceObservations) ||
		!reflect.DeepEqual(accepted.Manifest.ReviewCandidates, baseline.Manifest.ReviewCandidates) {
		t.Fatal("origin bootstrap changed the original catalog evidence")
	}
	if _, err := catalogs.DecodeCatalogGeneration(accepted); err != nil {
		t.Fatalf("origin bootstrap lost membership evidence: %v", err)
	}
}
