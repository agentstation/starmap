package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// testObservationPayload returns one provider observation whose records carry
// no canonical model reference. A live provider reply looks like this for
// every offering that the baseline does not link.
func testObservationPayload(t testing.TB, providerID catalogs.ProviderID, modelIDs ...string) []byte {
	t.Helper()
	builder := catalogs.NewEmpty()
	models := make(map[string]*catalogs.Model, len(modelIDs))
	for _, modelID := range modelIDs {
		// A live reply carries serving facts, and the enrich merge adds a new
		// offering only when it carries pricing or limits.
		models[modelID] = &catalogs.Model{
			ID:     modelID,
			Name:   modelID,
			Limits: &catalogs.ModelLimits{ContextWindow: 8192, OutputTokens: 1024},
		}
	}
	if err := builder.SetProvider(catalogs.Provider{
		ID:     providerID,
		Name:   string(providerID),
		Models: models,
	}); err != nil {
		t.Fatalf("SetProvider(%s): %v", providerID, err)
	}
	observed, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatalf("NewObservationCatalog: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(observed)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	return payload
}

// testObservationLayer returns one retained provider layer built from a live
// style observation.
func testObservationLayer(t testing.TB, providerID catalogs.ProviderID, observed time.Time, modelIDs ...string) ProviderLayer {
	t.Helper()
	payload := testObservationPayload(t, providerID, modelIDs...)
	return ProviderLayer{
		ProviderID: providerID,
		Payload:    payload,
		Digest:     catalogs.DescribeCatalogPayload(payload).Checksum,
		ObservedAt: observed,
	}
}

// TestBuildKeepsUnlinkedOfferingsOutOfTheEffectiveCatalog proves that a
// provider observation with offerings that name no authored model still
// publishes. The linked offering keeps the baseline reference, and the
// unlinked offering stays out.
func TestBuildKeepsUnlinkedOfferingsOutOfTheEffectiveCatalog(t *testing.T) {
	observed := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	baselinePayload := testCatalogPayload(t, "deepinfra", "linked-model", "Linked Model")
	baselineCatalog, err := catalogs.DecodeCatalogPayload(baselinePayload)
	if err != nil {
		t.Fatalf("DecodeCatalogPayload: %v", err)
	}
	baseline := starmap.CatalogState{
		GenerationID: "baseline",
		Catalog:      baselineCatalog,
	}

	layers := layerSet{}
	layers.setProvider(testObservationLayer(t, "deepinfra", observed, "linked-model", "unlinked-model"))

	state, err := layers.build(baseline)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	provider, found := state.Catalog.Providers().Get("deepinfra")
	if !found {
		t.Fatal("the effective catalog lost the observed provider")
	}
	linked := provider.Models["linked-model"]
	if linked == nil || linked.ModelRef == "" {
		t.Fatalf("linked offering = %+v, want the baseline model reference", linked)
	}
	if _, present := provider.Models["unlinked-model"]; present {
		t.Fatal("an offering without a canonical model reference reached the effective catalog")
	}
}

// TestUnlinkedProviderOfferingsNeverBlockTheSourceLayer proves the runtime
// contract end to end. Acquisition retains a layer with unlinked offerings, the
// runtime publishes, and a later upstream generation still publishes above the
// retained layer.
func TestUnlinkedProviderOfferingsNeverBlockTheSourceLayer(t *testing.T) {
	published := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	source := newStubSource("unlinked-source")
	source.replies = []SourceRead{
		testSourceRead(t, "generation-1", testCatalogPayload(t, "deepinfra", "linked-model", "Linked Model"), published),
		testSourceRead(t, "generation-2", testCatalogPayload(t, "deepinfra", "linked-model", "Linked Model v2"), published.Add(time.Hour)),
	}
	acquirer := &stubAcquirer{result: AcquisitionResult{
		Eligible: 1,
		Attempts: []sources.ProviderAttempt{
			testAttempt("deepinfra", sources.ProviderOutcomeSucceeded, ""),
		},
		Layers: []ProviderLayer{
			testObservationLayer(t, "deepinfra", published, "linked-model", "unlinked-model"),
		},
	}}
	runtime := openTestRuntime(t, WithSource(source), WithAcquirer(acquirer))

	if _, err := runtime.RefreshSource(context.Background()); err != nil {
		t.Fatalf("RefreshSource: %v", err)
	}
	report, err := runtime.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if !report.Published {
		t.Fatal("Sync published nothing above the retained layer")
	}
	provider, found := runtime.Catalog().Providers().Get("deepinfra")
	if !found {
		t.Fatal("the effective catalog lost the observed provider")
	}
	if _, present := provider.Models["unlinked-model"]; present {
		t.Fatal("an offering without a canonical model reference reached the effective catalog")
	}

	sourceReport, err := runtime.RefreshSource(context.Background())
	if err != nil {
		t.Fatalf("RefreshSource above the retained layer: %v", err)
	}
	if sourceReport.Reason != "" {
		t.Fatalf("source reason = %q, want none", sourceReport.Reason)
	}
	if id := runtime.State().GenerationID; !strings.HasPrefix(id, "generation-2+local.") {
		t.Fatalf("effective generation = %q, want generation-2 with a local suffix", id)
	}
	if provider, found := runtime.Catalog().Providers().Get("deepinfra"); !found || provider.Models["linked-model"] == nil {
		t.Fatal("the effective catalog lost the linked offering after the source refresh")
	}
}
