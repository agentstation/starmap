package runtime

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
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
	if id := runtime.State().GenerationID; !strings.HasPrefix(id, "generation-2"+effectiveGenerationLocalSuffix) {
		t.Fatalf("effective generation = %q, want generation-2 with a local suffix", id)
	}
	if provider, found := runtime.Catalog().Providers().Get("deepinfra"); !found || provider.Models["linked-model"] == nil {
		t.Fatal("the effective catalog lost the linked offering after the source refresh")
	}
}

// countingStore counts every commit that reaches the catalog store. A test
// uses it to prove that an unchanged rebuild retains one generation.
type countingStore struct {
	storage.Store

	mu      sync.Mutex
	commits int
}

// Commit counts the call and then commits through the wrapped store.
func (s *countingStore) Commit(ctx context.Context, generation catalogs.Generation, expected string) error {
	s.mu.Lock()
	s.commits++
	s.mu.Unlock()
	return s.Store.Commit(ctx, generation, expected)
}

// commitCount returns how many commits the store answered.
func (s *countingStore) commitCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commits
}

// identityUpstreamGeneration is the upstream identity that every identity test
// reads from its source.
const identityUpstreamGeneration = "generation-upstream"

// openIdentityRuntime opens one runtime above a fixed upstream reply and a
// fixed provider observation. Two runtimes that use it compose the same
// effective catalog, so a test compares the identity that each one reports. A
// nil store selects in-memory publication.
func openIdentityRuntime(t *testing.T, store storage.Store, opts ...Option) *Runtime {
	t.Helper()
	observed := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	source := newStubSource("identity-source")
	source.replies = []SourceRead{
		testSourceRead(t, identityUpstreamGeneration,
			testCatalogPayload(t, "deepinfra", "linked-model", "Linked Model"), observed),
	}
	acquirer := &stubAcquirer{result: AcquisitionResult{
		Eligible: 1,
		Attempts: []sources.ProviderAttempt{
			testAttempt("deepinfra", sources.ProviderOutcomeSucceeded, ""),
		},
		Layers: []ProviderLayer{
			testObservationLayer(t, "deepinfra", observed, "linked-model"),
		},
	}}
	base := []Option{WithSource(source), WithAcquirer(acquirer)}
	if store != nil {
		base = append(base, WithClientOptions(starmap.WithCatalogStore(store)))
	}
	return openTestRuntime(t, append(base, opts...)...)
}

// refreshAndSync reads the upstream source and then observes the providers.
func refreshAndSync(t *testing.T, runtime *Runtime) {
	t.Helper()
	if _, err := runtime.RefreshSource(context.Background()); err != nil {
		t.Fatalf("RefreshSource: %v", err)
	}
	if _, err := runtime.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
}

// TestDurableRuntimeKeepsTheDerivedEffectiveIdentity proves that a catalog
// store does not replace the identity that the retained layers derive. A
// durable runtime and an in-memory runtime that hold the same layers report
// one identity, so a served generation ID names the served bytes.
func TestDurableRuntimeKeepsTheDerivedEffectiveIdentity(t *testing.T) {
	memory := openIdentityRuntime(t, nil)
	durable := openIdentityRuntime(t, storage.NewMemory())

	// An upstream generation without a local layer keeps the upstream identity.
	for _, runtime := range []*Runtime{memory, durable} {
		if _, err := runtime.RefreshSource(context.Background()); err != nil {
			t.Fatalf("RefreshSource: %v", err)
		}
	}
	if got := durable.State().GenerationID; got != identityUpstreamGeneration {
		t.Fatalf("durable generation = %q, want the upstream identity %q",
			got, identityUpstreamGeneration)
	}
	if got := memory.State().GenerationID; got != identityUpstreamGeneration {
		t.Fatalf("in-memory generation = %q, want the upstream identity %q",
			got, identityUpstreamGeneration)
	}

	// One local layer derives the identity, and the durable commit keeps it.
	for _, runtime := range []*Runtime{memory, durable} {
		if _, err := runtime.Sync(context.Background()); err != nil {
			t.Fatalf("Sync: %v", err)
		}
	}
	derived := memory.State().GenerationID
	if !strings.HasPrefix(derived, identityUpstreamGeneration+effectiveGenerationLocalSuffix) {
		t.Fatalf("in-memory generation = %q, want the upstream identity with a local suffix", derived)
	}
	if got := durable.State().GenerationID; got != derived {
		t.Fatalf("durable generation = %q, want the derived identity %q", got, derived)
	}
	if got := durable.Client().CurrentGenerationID(); got != derived {
		t.Fatalf("committed generation = %q, want the derived identity %q", got, derived)
	}
}

// TestUnchangedRebuildCommitsNoSecondGeneration proves the commit no-op. A
// rebuild that derives the committed identity serves the committed bytes, so
// the catalog store keeps one generation.
func TestUnchangedRebuildCommitsNoSecondGeneration(t *testing.T) {
	store := &countingStore{Store: storage.NewMemory()}
	runtime := openIdentityRuntime(t, store)
	refreshAndSync(t, runtime)

	committed := store.commitCount()
	derived := runtime.State().GenerationID
	if _, err := runtime.Sync(context.Background()); err != nil {
		t.Fatalf("second Sync: %v", err)
	}
	if got := store.commitCount(); got != committed {
		t.Fatalf("commits = %d, want the %d that the changed layers needed", got, committed)
	}
	if got := runtime.State().GenerationID; got != derived {
		t.Fatalf("generation = %q, want the unchanged identity %q", got, derived)
	}
	if got := runtime.Client().CurrentGenerationID(); got != derived {
		t.Fatalf("committed generation = %q, want the unchanged identity %q", got, derived)
	}
}

// TestRestartReportsTheCommittedEffectiveIdentity proves that a restart from
// the same durable state reports the identity that the previous run committed.
func TestRestartReportsTheCommittedEffectiveIdentity(t *testing.T) {
	stateDirectory := t.TempDir()
	storeDirectory := t.TempDir()
	store, err := storage.NewFilesystem(storeDirectory)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	first := openIdentityRuntime(t, store, WithStateDirectory(stateDirectory))
	refreshAndSync(t, first)
	derived := first.State().GenerationID
	if !strings.HasPrefix(derived, identityUpstreamGeneration+effectiveGenerationLocalSuffix) {
		t.Fatalf("generation = %q, want the upstream identity with a local suffix", derived)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	restarted, err := storage.NewFilesystem(storeDirectory)
	if err != nil {
		t.Fatalf("NewFilesystem after the restart: %v", err)
	}
	second := openIdentityRuntime(t, restarted, WithStateDirectory(stateDirectory))
	if got := second.State().GenerationID; got != derived {
		t.Fatalf("generation after the restart = %q, want %q", got, derived)
	}
	if got := second.Client().CurrentGenerationID(); got != derived {
		t.Fatalf("committed generation after the restart = %q, want %q", got, derived)
	}
}

// TestSourcelessDurableRestartKeepsOneDerivedIdentity proves that a durable
// runtime without an upstream layer derives one identity across restarts. The
// restart baseline is the generation that the previous run committed. A
// derivation from that identity would nest one more suffix per restart and
// commit one more generation per restart.
func TestSourcelessDurableRestartKeepsOneDerivedIdentity(t *testing.T) {
	stateDirectory := t.TempDir()
	storeDirectory := t.TempDir()
	observed := time.Date(2026, 9, 4, 9, 0, 0, 0, time.UTC)
	open := func() (*Runtime, *countingStore) {
		t.Helper()
		filesystem, err := storage.NewFilesystem(storeDirectory)
		if err != nil {
			t.Fatalf("NewFilesystem: %v", err)
		}
		store := &countingStore{Store: filesystem}
		acquirer := &stubAcquirer{result: AcquisitionResult{
			Eligible: 1,
			Attempts: []sources.ProviderAttempt{
				testAttempt("deepinfra", sources.ProviderOutcomeSucceeded, ""),
			},
			Layers: []ProviderLayer{
				testObservationLayer(t, "deepinfra", observed, "linked-model"),
			},
		}}
		runtime := openTestRuntime(t,
			WithSource(embeddedSource{}),
			WithAcquirer(acquirer),
			WithClientOptions(starmap.WithCatalogStore(store)),
			WithStateDirectory(stateDirectory))
		return runtime, store
	}

	first, firstStore := open()
	root := first.Client().CurrentGenerationID()
	if _, err := first.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	derived := first.State().GenerationID
	if !strings.HasPrefix(derived, root+effectiveGenerationLocalSuffix) {
		t.Fatalf("generation = %q, want %q with a local suffix", derived, root)
	}
	if got := firstStore.commitCount(); got != 1 {
		t.Fatalf("commits before the restart = %d, want 1", got)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, secondStore := open()
	if got := second.State().GenerationID; got != derived {
		t.Fatalf("generation after the restart = %q, want %q", got, derived)
	}
	if got := secondStore.commitCount(); got != 0 {
		t.Fatalf("commits at the restart = %d, want 0", got)
	}
	if _, err := second.Sync(context.Background()); err != nil {
		t.Fatalf("Sync after the restart: %v", err)
	}
	if got := second.State().GenerationID; got != derived {
		t.Fatalf("generation after the restart sync = %q, want %q", got, derived)
	}
	if got := secondStore.commitCount(); got != 0 {
		t.Fatalf("commits after the restart sync = %d, want 0", got)
	}
}
