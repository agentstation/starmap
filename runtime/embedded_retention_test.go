package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestOfflineStartupRetainsEmbeddedGeneration(t *testing.T) {
	for _, source := range []string{"embedded", "public"} {
		t.Run(source, func(t *testing.T) {
			store := &countingStore{Store: storage.NewMemory()}
			directory := privateRuntimeDirectory(t)
			workspace := filepath.Join(t.TempDir(), "unused-workspace")
			options := []Option{WithStateDirectory(directory), WithCatalogSource(source),
				WithCatalogNetworkMode("offline"), WithAcquisitionEnabled(false), WithSourcePollInterval(0),
				WithClientOptions(starmap.WithCatalogStore(store), starmap.WithCatalogPath(workspace))}
			first := openTestRuntime(t, options...)
			state := first.State()
			if got := store.commitCount(); got != 1 {
				t.Fatalf("cold baseline commits = %d, want 1", got)
			}
			generation, err := store.Current(t.Context())
			if err != nil {
				t.Fatalf("offline startup did not retain the embedded generation: %v", err)
			}
			if generation.Manifest.GenerationID != state.GenerationID || generation.Manifest.Payload.Checksum != state.PayloadChecksum {
				t.Fatal("retained baseline differs from the served generation")
			}
			if err := first.Close(); err != nil {
				t.Fatal(err)
			}
			second := openTestRuntime(t, options...)
			if got := store.commitCount(); got != 1 {
				t.Fatalf("restart recommitted the retained baseline: %d commits", got)
			}
			if _, err := os.Lstat(workspace); !os.IsNotExist(err) {
				t.Fatalf("baseline retention initialized an unused authoring workspace: %v", err)
			}
			if second.State().GenerationID != state.GenerationID || second.State().PayloadChecksum != state.PayloadChecksum {
				t.Fatal("offline restart changed the retained baseline")
			}
		})
	}
}

func TestOfflineBaselineStorageFailureRefusesStartup(t *testing.T) {
	store := failingBaselineStore{Store: storage.NewMemory()}
	connected, err := Open(t.Context(), WithStateDirectory(privateRuntimeDirectory(t)),
		WithCatalogSource("embedded"), WithCatalogNetworkMode("offline"), WithAcquisitionEnabled(false),
		WithClientOptions(starmap.WithCatalogStore(store)))
	if connected != nil {
		_ = connected.Close()
		t.Fatal("failed baseline persistence exposed a runtime")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("baseline storage failure = %v", err)
	}
}

type failingBaselineStore struct{ storage.Store }

func (failingBaselineStore) Commit(context.Context, catalogs.Generation, string) error {
	return context.Canceled
}
