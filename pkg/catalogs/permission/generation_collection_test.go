package permission

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestPublisherGenerationCollectionChecksAuthority(t *testing.T) {
	for _, mode := range []string{"matching", "different authority", "unsupported permissions", "stale expectation"} {
		t.Run(mode, func(t *testing.T) {
			store := storage.NewMemory()
			old, current := issuerGeneration(t, "old", 1), issuerGeneration(t, "current", 2)
			if mode == "different authority" {
				current.Manifest.AuthorityHead.AuthorityID = "other"
			}
			if mode == "unsupported permissions" {
				current.Manifest.AuthorityHead.PermissionSchemaVersion = 99
			}
			if err := store.Commit(t.Context(), old, ""); err != nil {
				t.Fatal(err)
			}
			if err := store.Commit(t.Context(), current, old.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
			collector, ok := storage.GenerationCollectorFor(newTestPublisher(t, store))
			if !ok {
				t.Fatal("publisher hides collection capability")
			}
			request := storage.RetentionRequest{ExpectedGenerationID: current.Manifest.GenerationID, MaxGenerations: 1, MaxBytes: 1}
			if mode == "stale expectation" {
				request.ExpectedGenerationID = "old"
			}
			report, err := collector.Collect(t.Context(), request)
			if mode == "matching" {
				if err != nil || len(report.Removed) != 1 || report.Removed[0] != "old" {
					t.Fatalf("collection failed: %+v, %v", report, err)
				}
			} else {
				if err == nil {
					t.Fatal("unsafe authority collection succeeded")
				}
				if _, err := store.Get(t.Context(), "old"); err != nil {
					t.Fatal("refused collection removed data", err)
				}
			}
		})
	}
}

func TestPublisherGenerationCollectionPreservesUnsupportedStores(t *testing.T) {
	publisher := newTestPublisher(t, generationLeaseHiddenStore{Store: storage.NewMemory()})
	if collector, ok := storage.GenerationCollectorFor(publisher); ok || collector != nil {
		t.Fatal("publisher invented collection support")
	}
}
