package runtime

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestRetainedProviderFactsUseCanonicalAuthorityAndReceipts(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	basePayload := testCatalogPayload(t, "provider", "model", "Baseline")
	base, err := catalogs.DecodeCatalogPayload(basePayload)
	if err != nil {
		t.Fatal(err)
	}
	builder, err := catalogs.NewBuilderFrom(base)
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := builder.Provider("provider")
	provider.Models["model"].Limits = &catalogs.ModelLimits{ContextWindow: 100}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	base, err = builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	observedBuilder, err := catalogs.NewBuilderFrom(base)
	if err != nil {
		t.Fatal(err)
	}
	provider.Models["model"].Limits.ContextWindow = 200
	if err := observedBuilder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	observed, err := observedBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(observed)
	if err != nil {
		t.Fatal(err)
	}
	layer := testProviderLayerFromPayload(t, "provider", payload, at)
	layers := layerSet{}
	layers.setProvider(layer)
	baseline := starmap.CatalogState{Catalog: base, GenerationID: "baseline", GeneratedAt: at.Add(-time.Hour)}
	first, err := layers.build(t.Context(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := first.Catalog.Provider("provider")
	if got.Models["model"].Limits.ContextWindow != 200 {
		t.Fatal("retained provider facts did not replace lower-authority baseline limits")
	}
	entries := first.Catalog.Provenance().FindModel("provider", "model")["limits.context_window"]
	if len(entries) == 0 || entries[len(entries)-1].ObservationID != layer.Receipt.Link.ObservationID || entries[len(entries)-1].Source != sources.ProvidersID {
		t.Fatal("effective catalog lost the provider receipt")
	}
	second, err := layers.build(t.Context(), baseline)
	if err != nil {
		t.Fatal(err)
	}
	if first.PayloadChecksum != second.PayloadChecksum || first.GenerationID != second.GenerationID {
		t.Fatal("unchanged retained evidence changed effective generation bytes")
	}
}

func TestRuntimePublicationRetainsEachProviderReceiptAcrossRestart(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "bound"}[scoped], func(t *testing.T) {
			at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
			layers := []ProviderLayer{
				testProviderLayer(t, "provider-one", "model-one", "One", at),
				testProviderLayer(t, "provider-two", "model-two", "Two", at.Add(time.Minute)),
			}
			store := storage.NewMemory()
			opts := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithSource(testReviewedDefinitionsSource(t, layers)), WithClientOptions(starmap.WithCatalogStore(store))}
			if scoped {
				var bindings []sources.ProviderAcquisitionBinding
				for i := range layers {
					binding := sources.ProviderAcquisitionBinding{SchemaVersion: 1, ID: string(layers[i].ProviderID), Revision: "1", ProviderID: layers[i].ProviderID,
						AccountID: "fixture", Region: "global", APISurface: "models.list", CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "unauthenticated"}
					observed, err := catalogs.DecodeSourceObservationPayload(layers[i].Payload)
					if err != nil {
						t.Fatal(err)
					}
					observation, err := sources.NewObservation(sources.ProvidersID, observed, sources.ObservationMetadata{
						ProviderBinding: &binding, ObservedAt: layers[i].ObservedAt, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
						Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded, Records: sources.ObservationRecordCounts{Accepted: 1},
					})
					if err != nil {
						t.Fatal(err)
					}
					layers[i], err = NewProviderLayer(layers[i].ProviderID, observation)
					if err != nil {
						t.Fatal(err)
					}
					bindings = append(bindings, binding)
				}
				opts = append(opts, WithProviderBindings(bindings...))
			}
			first := openTestRuntime(t, opts...)
			if _, err := first.RefreshSource(t.Context()); err != nil {
				t.Fatal(err)
			}
			baselineCount := first.Catalog().Providers().Len()
			if _, err := first.publishProviders(t.Context(), layers, first.lease.epoch()); err != nil {
				t.Fatal(err)
			}
			generation, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(generation.Manifest.SourceObservations) != len(layers) {
				t.Fatalf("published links = %d", len(generation.Manifest.SourceObservations))
			}
			for _, layer := range layers {
				found := false
				for _, link := range generation.Manifest.SourceObservations {
					found = found || reflect.DeepEqual(link, layer.Receipt.Link)
				}
				if !found {
					t.Fatal("published manifest lost an original provider link")
				}
				provider, err := first.Catalog().Provider(layer.ProviderID)
				if err != nil {
					t.Fatal("published catalog lost a selected provider")
				}
				if len(provider.Models) != 1 {
					t.Fatal("provider lost its reviewed model")
				}
				for id := range provider.Models {
					entries := first.Catalog().Provenance().FindModelField(layer.ProviderID, id, "Name")
					if len(entries) != 1 || entries[0].ObservationID != layer.Receipt.Link.ObservationID {
						t.Fatalf("model %s inherited a peer receipt: %#v", id, entries)
					}
				}
			}
			if first.Catalog().Providers().Len() != baselineCount+2 {
				t.Fatal("provider filter removed unobserved baseline providers")
			}
			if err := first.Close(); err != nil {
				t.Fatal(err)
			}
			second := openTestRuntime(t, opts...)
			if second.State().GenerationID != generation.Manifest.GenerationID || second.State().PayloadChecksum != generation.Manifest.Payload.Checksum {
				t.Fatal("restart changed the retained generation")
			}
			if _, err := second.rebuild(t.Context(), second.lease.epoch()); err != nil {
				t.Fatal(err)
			}
			repeated, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(repeated.Manifest, generation.Manifest) {
				t.Fatal("unchanged rebuild rewrote immutable receipt evidence")
			}
		})
	}
}

func TestCanceledRuntimeBuildDoesNotReplaceAcceptedGeneration(t *testing.T) {
	store := storage.NewMemory()
	connected := openTestRuntime(t, WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)))
	before := connected.State()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := connected.rebuild(ctx, connected.lease.epoch()); !errors.Is(err, context.Canceled) {
		t.Fatalf("rebuild error = %v", err)
	}
	if connected.State().GenerationID != before.GenerationID || connected.State().PayloadChecksum != before.PayloadChecksum {
		t.Fatal("canceled rebuild replaced the accepted catalog")
	}
}

func TestRuntimeQuarantinePublishesOriginalReviewEvidence(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	layers := []ProviderLayer{
		testObservationLayer(t, "unlinked-one", at, "unknown-one"),
		testObservationLayer(t, "unlinked-two", at.Add(time.Minute), "unknown-two"),
		testProviderLayer(t, "provider-authored", "unreviewed", "Unreviewed", at),
	}
	store := storage.NewMemory()
	connected := openTestRuntime(t, WithCatalogSource("embedded"), WithClientOptions(starmap.WithCatalogStore(store)))
	if _, err := connected.publishProviders(t.Context(), layers, connected.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	generation, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := generation.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(generation.Manifest.ReviewCandidates) != len(layers) {
		t.Fatal("publication lost quarantined offerings")
	}
	for _, candidate := range generation.Manifest.ReviewCandidates {
		matched := false
		for _, layer := range layers {
			if candidate.ProviderID == string(layer.ProviderID) {
				matched = candidate.SourceObservationID == layer.Receipt.Link.ObservationID && candidate.EvidenceChecksum == layer.Receipt.Link.EvidenceChecksum
			}
		}
		if !matched {
			t.Fatal("quarantined offering inherited a peer receipt")
		}
		if provider, err := connected.Catalog().Provider(catalogs.ProviderID(candidate.ProviderID)); err == nil && provider.Models[candidate.ProviderModelID] != nil {
			t.Fatal("quarantined offering reached the effective catalog")
		}
	}
}

// testReviewedDefinitionsSource keeps reviewed definitions outside provider evidence.
func testReviewedDefinitionsSource(t *testing.T, layers []ProviderLayer) Source {
	t.Helper()
	client, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	return testReviewedDefinitionsFromBaseline(t, client.EmbeddedCatalogState().Catalog, layers)
}

func testReviewedDefinitionsFromBaseline(t *testing.T, baseline *catalogs.Catalog, layers []ProviderLayer) Source {
	t.Helper()
	builder := catalogs.NewEmpty()
	if baseline != nil {
		var err error
		builder, err = catalogs.NewBuilderFrom(baseline)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, layer := range layers {
		fixture, err := catalogs.DecodeCatalogPayload(layer.Payload)
		if err != nil {
			t.Fatal(err)
		}
		for _, author := range fixture.Authors().List() {
			if err := builder.SetAuthor(author); err != nil {
				t.Fatal(err)
			}
		}
		for _, record := range fixture.AuthoredModels() {
			if err := builder.SetAuthorModel(record.AuthorID, record.Model); err != nil {
				t.Fatal(err)
			}
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	source := newStubSource("reviewed-fixture")
	source.replies = []SourceRead{testSourceRead(t, "reviewed-fixture", payload, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))}
	return source
}
