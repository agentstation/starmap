package workspace

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestTreeSnapshotBindsIdentityAndOperatorBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	if err := os.Mkdir(path, directoryMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "notes.txt"), []byte("before"), fileMode); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotTree(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal(err)
	}
	moved, err := snapshotTree(t.Context(), path+".moved")
	if err != nil || !sameTree(before, moved) {
		t.Fatalf("identity changed after rename: %v", err)
	}
	if err := os.CopyFS(path, os.DirFS(path+".moved")); err != nil {
		t.Fatal(err)
	}
	copy, err := snapshotTree(t.Context(), path)
	if err != nil || copy.Digest != before.Digest || copy.ID == before.ID {
		t.Fatalf("copy did not preserve content with a distinct identity: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "notes.txt"), []byte("edited"), fileMode); err != nil {
		t.Fatal(err)
	}
	edited, err := snapshotTree(t.Context(), path)
	if err != nil || edited.ID != copy.ID || edited.Digest == copy.Digest {
		t.Fatalf("operator edit did not change digest: %v", err)
	}
}

func TestTreeSnapshotEnforcesResourceBoundsAndCancellation(t *testing.T) {
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "one"), []byte("x"), fileMode); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	scanner := treeScanner{ctx: t.Context(), root: root, bytes: replacementMaxBytes}
	_, err = scanner.entry("one")
	var validation *errors.ValidationError
	if !stderrors.As(err, &validation) {
		t.Fatalf("byte limit: %v", err)
	}
	scanner = treeScanner{ctx: t.Context(), root: root, seen: replacementMaxEntries}
	if err := scanner.visit("."); !stderrors.As(err, &validation) {
		t.Fatalf("entry limit: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := snapshotTree(ctx, path); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}
