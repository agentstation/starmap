package runtime

import (
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestRestoreRejectsDifferentEvidenceUnderSelectedIdentity(t *testing.T) {
	layer := scopedProviderLayer(t, "binding", "1", time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	store := storage.NewMemory()
	common := []Option{WithStateDirectory(privateRuntimeDirectory(t)), WithSource(testReviewedDefinitionsSource(t, []ProviderLayer{layer})), WithClientOptions(starmap.WithCatalogStore(store))}
	first := openTestRuntime(t, append(common, WithProviderBindings(*layer.Receipt.ProviderBinding))...)
	if _, err := first.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := first.publishProviders(t.Context(), []ProviderLayer{layer}, first.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	original, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	inactive := openTestRuntime(t, append(common, WithProviderBindings())...)
	if err := inactive.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	altered := original.Copy()
	unexpected := altered.Manifest.SourceObservations[0]
	unexpected.ObservationID += ":unexpected"
	altered.Manifest.SourceObservations = append(altered.Manifest.SourceObservations, unexpected)
	if err := altered.Validate(); err != nil {
		t.Fatalf("altered fixture is not a valid generation: %v", err)
	}
	supplied := &policyMismatchedReadStore{Memory: store, generation: altered}
	selected := append(common, WithProviderBindings(*layer.Receipt.ProviderBinding), WithClientOptions(starmap.WithCatalogStore(supplied)), WithAcquisitionEnabled(false), WithSourcePollInterval(0))
	restored, err := Open(t.Context(), selected...)
	if err == nil {
		if err := restored.Close(); err != nil {
			t.Error(err)
		}
		t.Error("restore accepted different evidence under the selected identity")
	}
	if !errors.IsConflict(err) {
		t.Fatalf("restore error = %v, want immutable generation conflict", err)
	}
	current, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if current.Manifest.GenerationID != before.Manifest.GenerationID {
		t.Error("mismatched evidence changed the accepted generation")
	}
}
