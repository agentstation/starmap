package runtime

import (
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestOriginPublicationReopensFilesystemWithExactIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog-store")
	store, err := storage.NewFilesystem(path)
	if err != nil {
		t.Fatal(err)
	}
	directory := privateRuntimeDirectory(t)
	connected := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithAuthorityOrigin(store, originTestConfig()))
	accepted := connected.State()
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.NewFilesystem(path)
	if err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithAuthorityOrigin(store, originTestConfig()))
	receipt, err := reopened.ReadPermission(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State().GenerationID != accepted.GenerationID || receipt.Head.GenerationID != accepted.GenerationID || receipt.Head.Sequence != 1 {
		t.Fatal("filesystem restart changed authority identity")
	}
}
