package productfiles_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/productfiles"
)

func TestPrivatePublicationInspectionPreservesFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private")
	directory := newDirectory(t, path)
	if err := directory.CheckNoPendingPublications(t.Context()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatalf("inspection created metadata: %v, %v", entries, err)
	}
	if err := directory.CompareAndPublish(t.Context(), "record", nil, []byte("accepted")); err != nil {
		t.Fatal(err)
	}
	if err := directory.CheckNoPendingPublications(t.Context()); err != nil {
		t.Fatal(err)
	}
	metadata, err := directory.ExistingChild(productfiles.PublicationDirectoryName)
	if err != nil {
		t.Fatal(err)
	}
	pending := filepath.Join(path, productfiles.PublicationDirectoryName, "pending.jsonl")
	if err := os.WriteFile(pending, []byte("retained incomplete receipt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := directory.CheckNoPendingPublications(t.Context()); err == nil {
		t.Fatal("accepted pending publication")
	}
	body, err := metadata.ReadFile("pending.jsonl", 1024)
	if err != nil || string(body) != "retained incomplete receipt" {
		t.Fatalf("changed pending receipt: %v", err)
	}
	body, err = directory.ReadFile("record", 1024)
	if err != nil || string(body) != "accepted" {
		t.Fatalf("changed published record: %v", err)
	}
}

func TestPrivatePublicationInspectionValidatesContext(t *testing.T) {
	var zero productfiles.Directory
	if err := zero.CheckNoPendingPublications(t.Context()); err == nil {
		t.Fatal("accepted zero directory")
	}
	directory := newDirectory(t, filepath.Join(t.TempDir(), "private"))
	if err := directory.CheckNoPendingPublications(nil); err == nil {
		t.Fatal("accepted nil context")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := directory.CheckNoPendingPublications(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
