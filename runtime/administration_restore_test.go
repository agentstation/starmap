package runtime

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/server/administration"
)

func TestOfflineRestorePreservesAdministrationAndCatalogAuthority(t *testing.T) {
	root := filepath.Join(t.TempDir(), "state")
	if _, err := privatefiles.NewDirectory(root); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(root, "catalog-store")
	store, err := storage.NewFilesystem(storePath)
	if err != nil {
		t.Fatal(err)
	}
	origin := originTestConfig()
	connected := openTestRuntime(t, WithStateDirectory(filepath.Join(root, "runtime")), WithCatalogSource("embedded"), WithAuthorityOrigin(store, origin))
	accepted, err := connected.ReadPermission(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cfg := administration.Config{StateDirectory: root, Audience: origin.AuthorityID}
	manager, adminToken, err := administration.Initialize(t.Context(), cfg, "operator")
	if err != nil {
		t.Fatal(err)
	}
	actor, _ := manager.Authenticate(adminToken, cfg.Audience)
	subscriberToken, err := manager.Create(t.Context(), actor, "gateway", administration.Subscriber)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := manager.StartOperation(t.Context(), actor, administration.RefreshCatalog, "catalog")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Complete(t.Context(), receipt.ID, true); err != nil {
		t.Fatal(err)
	}
	before := manager.Status()
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	backup := root + "-backup"
	if err := os.Rename(root, backup); err != nil {
		t.Fatal(err)
	}
	copyPrivateOfflineBackup(t, backup, root)
	restored, err := administration.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if restored.Status() != before {
		t.Fatalf("administration revision changed: before=%+v after=%+v", before, restored.Status())
	}
	actor, ok := restored.Authenticate(adminToken, cfg.Audience)
	if !ok {
		t.Fatal("restore lost administrator authority")
	}
	if principal, ok := restored.Authenticate(subscriberToken, cfg.Audience); !ok || principal.Role() != administration.Subscriber {
		t.Fatal("restore changed subscriber authority")
	}
	operation, err := restored.Operation(actor, receipt.ID)
	if err != nil || operation.State != administration.Succeeded {
		t.Fatal("restore lost durable operation history")
	}
	store, err = storage.NewFilesystem(storePath)
	if err != nil {
		t.Fatal(err)
	}
	reopened := openTestRuntime(t, WithStateDirectory(filepath.Join(root, "runtime")), WithCatalogSource("embedded"), WithAuthorityOrigin(store, origin))
	permission, err := reopened.ReadPermission(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if permission.Head != accepted.Head || reopened.State().GenerationID != accepted.Head.GenerationID {
		t.Fatal("restore changed accepted catalog authority")
	}
}

func copyPrivateOfflineBackup(t *testing.T, source, destination string) {
	t.Helper()
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			_, err := privatefiles.NewDirectory(target)
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		directory, err := privatefiles.ExistingDirectory(filepath.Dir(target))
		if err != nil {
			return err
		}
		return directory.WriteFileIfAbsentContext(t.Context(), filepath.Base(target), data, ".restore-")
	}); err != nil {
		t.Fatal(err)
	}
}
