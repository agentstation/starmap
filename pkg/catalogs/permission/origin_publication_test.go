package permission

import (
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestOriginPublicationSelectsDurableSequenceAfterReopen(t *testing.T) {
	root := filepath.Join(t.TempDir(), "catalog-store")
	store, err := storage.NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	publisher := newTestPublisher(t, store)
	input := originInput(t, true)
	first, err := publisher.PublishCatalog(t.Context(), input, "")
	if err != nil || first.Manifest.AuthorityHead.Sequence != 1 {
		t.Fatalf("initial publication: %+v / %v", first.Manifest.AuthorityHead, err)
	}
	store, err = storage.NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	publisher = newTestPublisher(t, store)
	retry, err := publisher.PublishCatalog(t.Context(), input, "")
	if err != nil || retry.Manifest.GenerationID != first.Manifest.GenerationID {
		t.Fatalf("publication retry after reopen: %s / %v", retry.Manifest.GenerationID, err)
	}
	next, err := publisher.PublishCatalog(t.Context(), originInput(t, false), first.Manifest.GenerationID)
	if err != nil || next.Manifest.AuthorityHead.Sequence != 2 {
		t.Fatalf("withdrawal publication: %+v / %v", next.Manifest.AuthorityHead, err)
	}
	if next.Manifest.AuthorityHead.RequiredPermissionRevision == first.Manifest.AuthorityHead.RequiredPermissionRevision {
		t.Fatal("withdrawal retained the old required revision")
	}
	if _, err := publisher.PublishCatalog(t.Context(), input, first.Manifest.GenerationID); err == nil {
		t.Fatal("stale source replaced a committed withdrawal")
	}
	current, err := store.Current(t.Context())
	if err != nil || current.Manifest.GenerationID != next.Manifest.GenerationID {
		t.Fatalf("stale publication changed the stored catalog: %s / %v", current.Manifest.GenerationID, err)
	}
}
