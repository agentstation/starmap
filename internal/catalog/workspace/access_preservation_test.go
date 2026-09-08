package workspace

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
)

func TestWorkspaceRenderRemainsInsidePrivateStaging(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "private-note"), []byte("private operator content"), 0o600); err != nil {
		t.Fatal(err)
	}
	next, identity := testCatalog(t, "new", "New")
	checked := false
	_, err := (projector{afterStageRender: func(render string) error {
		root, err := os.OpenRoot(filepath.Dir(render))
		if err != nil {
			return err
		}
		defer func() { _ = root.Close() }()
		info, err := root.Stat(".")
		if err != nil {
			return err
		}
		if err := privatefiles.ValidateMetadata(info, "workspace staging"); err != nil {
			return err
		}
		if err := privatefiles.ValidateACL(root, ".", info, "workspace staging"); err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(render, "private-note"))
		if err != nil {
			return err
		}
		if string(data) != "private operator content" {
			t.Fatal("render lost operator content")
		}
		checked = true
		return nil
	}}).project(t.Context(), path, next, identity, InputExpectation{})
	if err != nil || !checked {
		t.Fatal("private render check", err)
	}
	assertReplacementFinished(t, path)
}

func TestWorkspaceAccessFailurePreservesOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	old, oldIdentity := testCatalog(t, "old", "Old")
	if _, err := Project(t.Context(), path, old, oldIdentity); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotTree(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	next, identity := testCatalog(t, "new", "New")
	fault := stderrors.New("access restoration failed")
	_, err = (projector{beforeAccessRestore: func(name string) error {
		if name == "providers.yaml" {
			return fault
		}
		return nil
	}}).project(t.Context(), path, next, identity, InputExpectation{})
	if !stderrors.Is(err, fault) {
		t.Fatal("restore failure", err)
	}
	after, err := snapshotTree(t.Context(), path)
	if err != nil || !sameTree(before, after) {
		t.Fatal("restore failure changed original", err)
	}
	assertReplacementFinished(t, path)
}
