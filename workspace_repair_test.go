package starmap

import (
	"context"
	stderrors "errors"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestExplicitWorkspaceRepairPreservesUnrecognizedOperatorFiles(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "workspace")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "operator.txt"), []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := storage.NewMemory()
	if err := store.Commit(t.Context(), rootRemoteGeneration(t), ""); err != nil {
		t.Fatal(err)
	}
	client, err := New(WithCatalogStore(store), WithCatalogPath(path))
	if err != nil {
		t.Fatal(err)
	}
	before := filesystemContents(t, path)
	result, err := client.RepairWorkspace(t.Context())
	if err != nil || result.Changed || result.IssueCode == "" {
		t.Fatalf("operator workspace repair = %+v, %v", result, err)
	}
	if !maps.Equal(before, filesystemContents(t, path)) {
		t.Fatal("repair changed operator files")
	}
}

func TestExplicitWorkspaceRepairRequiresDurableStateAndHonorsCancellation(t *testing.T) {
	directory := t.TempDir()
	client, err := New(WithCatalogStore(storage.NewMemory()), WithCatalogPath(filepath.Join(directory, "workspace")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.RepairWorkspace(t.Context())
	if err != nil || result.Changed || result.GenerationID != "" {
		t.Fatalf("empty store repair = %+v, %v", result, err)
	}
	if files := filesystemContents(t, directory); len(files) != 1 {
		t.Fatal("repair without durable state created files")
	}
	if _, err := client.RepairWorkspace(nil); err == nil {
		t.Fatal("repair accepted a nil context")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.RepairWorkspace(ctx); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled repair = %v", err)
	}
	release, err := client.updates.acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel = context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if _, err := client.RepairWorkspace(ctx); !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("repair bypassed publication ownership: %v", err)
	}
}
