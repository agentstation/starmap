package permission

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestOriginPreparationReservesNoPublicationSequence(t *testing.T) {
	store := storage.NewMemory()
	publisher := newTestPublisher(t, store)
	first, err := publisher.PublishCatalog(t.Context(), originInput(t, true), "")
	if err != nil {
		t.Fatal(err)
	}
	withdrawal, err := publisher.PrepareCatalog(t.Context(), originInput(t, false), first.Manifest.GenerationID)
	if err != nil {
		t.Fatal(err)
	}
	if withdrawal.Manifest.AuthorityHead.Sequence != 2 {
		t.Fatal("preparation selected the wrong successor")
	}
	current, err := store.Current(t.Context())
	if err != nil || current.Manifest.GenerationID != first.Manifest.GenerationID {
		t.Fatalf("preparation changed storage: %v", err)
	}
	competing := originInput(t, true)
	competing.Manifest.SyncRunID = "competing"
	accepted, err := publisher.PublishCatalog(t.Context(), competing, first.Manifest.GenerationID)
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.Commit(t.Context(), withdrawal, first.Manifest.GenerationID); !errors.IsConflict(err) {
		t.Fatalf("stale prepared publication bypassed CAS: %v", err)
	}
	current, err = store.Current(t.Context())
	if err != nil || current.Manifest.GenerationID != accepted.Manifest.GenerationID {
		t.Fatalf("stale preparation replaced the accepted catalog: %v", err)
	}
}
