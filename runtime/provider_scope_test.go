package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func scopedProviderLayer(t *testing.T, bindingID, revision string, at time.Time) ProviderLayer {
	t.Helper()
	binding := sources.ProviderAcquisitionBinding{SchemaVersion: 1, ID: bindingID, Revision: revision, ProviderID: "provider", AccountID: bindingID, Region: "global", APISurface: "models", CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: "catalog-key"}
	return providerLayerWithBinding(t, binding, at)
}

func providerLayerWithBinding(t *testing.T, binding sources.ProviderAcquisitionBinding, at time.Time) ProviderLayer {
	t.Helper()
	base := testProviderLayer(t, binding.ProviderID, "model", "Model", at)
	catalog, err := catalogs.DecodeSourceObservationPayload(base.Payload)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{ProviderBinding: &binding, ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest}, Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded})
	if err != nil {
		t.Fatal(err)
	}
	layer, err := NewProviderLayer(binding.ProviderID, observation)
	if err != nil {
		t.Fatal(err)
	}
	return layer
}

func TestProviderScopesRetainIndependentObservations(t *testing.T) {
	for _, durable := range []bool{false, true} {
		t.Run(map[bool]string{false: "memory", true: "disk"}[durable], func(t *testing.T) {
			r := providerOrderRuntime(t, durable)
			at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
			first := scopedProviderLayer(t, "account-a", "1", at)
			second := scopedProviderLayer(t, "account-b", "1", at)
			if err := r.retainProviders(t.Context(), []ProviderLayer{first, second}); err != nil {
				t.Fatal("independent scopes conflict:", err)
			}
			if len(r.layers.providers) != 2 {
				t.Fatal("one scope replaced another")
			}
			newer := scopedProviderLayer(t, "account-a", "1", at.Add(time.Minute))
			if err := r.retainProviders(t.Context(), []ProviderLayer{newer}); err != nil {
				t.Fatal(err)
			}
			for _, layer := range r.layers.providers {
				if layer.Receipt.ProviderBinding.ID == "account-b" && layer.Receipt.Link.ObservationID != second.Receipt.Link.ObservationID {
					t.Fatal("scope refresh changed its peer")
				}
			}
			if durable {
				loaded, err := r.store.loadProviders()
				if err != nil {
					t.Fatal(err)
				}
				if len(loaded) != 2 {
					t.Fatal("restart lost an independent scope")
				}
			}
		})
	}
}

func TestProviderScopeChangeRequiresNewRevision(t *testing.T) {
	tests := []struct {
		name   string
		change func(*sources.ProviderAcquisitionBinding)
	}{
		{"account", func(b *sources.ProviderAcquisitionBinding) { b.AccountID = "different-account" }},
		{"provider", func(b *sources.ProviderAcquisitionBinding) { b.ProviderID = "another-provider" }},
		{"region", func(b *sources.ProviderAcquisitionBinding) { b.Region = "another-region" }},
		{"surface", func(b *sources.ProviderAcquisitionBinding) { b.APISurface = "another-surface" }},
		{"profile", func(b *sources.ProviderAcquisitionBinding) { b.CredentialProfileID = "another-profile" }},
		{"public", func(b *sources.ProviderAcquisitionBinding) { b.AccountID = ""; b.Public = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := providerOrderRuntime(t, true)
			at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
			first := scopedProviderLayer(t, "binding", "1", at)
			if err := r.retainProviders(t.Context(), []ProviderLayer{first}); err != nil {
				t.Fatal(err)
			}
			changed := *first.Receipt.ProviderBinding
			test.change(&changed)
			next := providerLayerWithBinding(t, changed, at.Add(time.Minute))
			if err := r.retainProviders(t.Context(), []ProviderLayer{next}); err == nil {
				t.Fatal("changed scope reused the retained binding revision")
			}
			loaded, err := r.store.loadProviders()
			if err != nil {
				t.Fatal(err)
			}
			for _, layer := range loaded {
				if layer.Receipt.Link.ObservationID != first.Receipt.Link.ObservationID {
					t.Fatal("scope conflict changed retained evidence")
				}
			}
		})
	}
}

func TestProviderScopeRevisionsAndUnscopedEvidenceStaySeparate(t *testing.T) {
	r := providerOrderRuntime(t, true)
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	layers := []ProviderLayer{
		testProviderLayer(t, "provider", "model", "Model", at),
		scopedProviderLayer(t, "binding", "1", at),
		scopedProviderLayer(t, "binding", "2", at),
	}
	if err := r.retainProviders(t.Context(), layers); err != nil {
		t.Fatal(err)
	}
	loaded, err := r.store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 3 {
		t.Fatal("revision or unscoped evidence replaced a peer")
	}
	for _, layer := range layers {
		if loaded[layer.evidenceKey()].Receipt.Link.ObservationID != layer.Receipt.Link.ObservationID {
			t.Fatal("wrong evidence after restart")
		}
	}
}

func TestProviderScopeSelectorsNeverNameFiles(t *testing.T) {
	r := providerOrderRuntime(t, true)
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	layer := scopedProviderLayer(t, "../../outside:秘密", "revision/with/slashes", at)
	if err := r.retainProviders(t.Context(), []ProviderLayer{layer}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(r.store.root, "providers", "bindings"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != ".record-publications" || !entries[0].IsDir() {
		t.Fatalf("unexpected publication metadata: %+v", entries)
	}
	entries = entries[1:]
	if len(entries) != 1 || len(entries[0].Name()) != 69 || !strings.HasSuffix(entries[0].Name(), ".json") {
		t.Fatalf("unexpected scoped filenames: %+v", entries)
	}
	for _, character := range strings.TrimSuffix(entries[0].Name(), ".json") {
		if !strings.ContainsRune("0123456789abcdef", character) {
			t.Fatal("selector entered filename")
		}
	}
	loaded, err := r.store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	if loaded[layer.evidenceKey()].Receipt.ProviderBinding.ID != "../../outside:秘密" {
		t.Fatal("stored selector did not round trip")
	}
}

func TestProviderScopeFilenameSeparatesFieldBoundaries(t *testing.T) {
	first := providerEvidenceKey{providerID: "provider", bindingID: "a:b", revision: "c"}
	second := providerEvidenceKey{providerID: "provider", bindingID: "a", revision: "b:c"}
	if first.filename() == second.filename() {
		t.Fatal("ambiguous identity fields share a filename")
	}
}

func TestProviderScopeDirectoryAndFilenameMustMatchReceipt(t *testing.T) {
	for _, location := range []string{"legacy-directory", "different-filename"} {
		t.Run(location, func(t *testing.T) {
			r := providerOrderRuntime(t, true)
			layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
			directory, name := r.store.bindings, "other.json"
			if location == "legacy-directory" {
				directory, name = r.store.providers, "provider.json"
			}
			if err := r.store.writeContext(t.Context(), directory, name, layer); err != nil {
				t.Fatal(err)
			}
			if _, err := r.store.loadProviders(); err == nil {
				t.Fatal("accepted receipt in a different storage identity")
			}
		})
	}
}

func TestProviderScopeConflictRejectsWholeBatch(t *testing.T) {
	r := providerOrderRuntime(t, true)
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	first := scopedProviderLayer(t, "binding", "1", at)
	changed := *first.Receipt.ProviderBinding
	changed.Region = "another-region"
	second := providerLayerWithBinding(t, changed, at.Add(time.Minute))
	peer := scopedProviderLayer(t, "peer", "1", at)
	if err := r.retainProviders(t.Context(), []ProviderLayer{peer, first, second}); err == nil {
		t.Fatal("batch reused a binding revision for another scope")
	}
	if len(r.layers.providers) != 0 {
		t.Fatal("conflicting batch changed memory")
	}
	loaded, err := r.store.loadProviders()
	if err != nil || len(loaded) != 0 {
		t.Fatalf("conflicting batch changed files: %v", err)
	}
}

func TestProviderScopesConcurrentRetention(t *testing.T) {
	r := providerOrderRuntime(t, true)
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	layers := make([]ProviderLayer, 12)
	for index := range layers {
		layers[index] = scopedProviderLayer(t, fmt.Sprintf("scope-%d", index), "1", at)
	}
	var work sync.WaitGroup
	failures := make(chan error, len(layers))
	for _, layer := range layers {
		work.Go(func() { failures <- r.retainProviders(t.Context(), []ProviderLayer{layer}) })
	}
	work.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(r.layers.providers) != len(layers) {
		t.Fatal("concurrent retention lost scopes")
	}
	loaded, err := r.store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != len(layers) {
		t.Fatal("restart lost concurrent scopes")
	}
}

func TestProviderScopeRestartRejectsReusedDeclaration(t *testing.T) {
	r := providerOrderRuntime(t, true)
	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	first := scopedProviderLayer(t, "binding", "1", at)
	changed := *first.Receipt.ProviderBinding
	changed.ProviderID = "different-provider"
	second := providerLayerWithBinding(t, changed, at)
	for _, layer := range []ProviderLayer{first, second} {
		if err := r.store.writeContext(t.Context(), r.store.bindings, layer.evidenceKey().filename(), layer); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.store.loadProviders(); err == nil {
		t.Fatal("restart accepted different declarations for one binding revision")
	}
}
