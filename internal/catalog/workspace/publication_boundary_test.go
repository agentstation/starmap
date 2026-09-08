package workspace

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestProjectPreservesDirectoryCreatedAtPublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	catalog, identity := testCatalog(t, "new", "New Model")
	var before os.FileInfo
	receipt, err := (projector{beforePromote: func() error {
		if err := os.Mkdir(path, directoryMode); err != nil {
			return err
		}
		var err error
		before, err = os.Stat(path)
		return err
	}}).project(t.Context(), path, catalog, identity, InputExpectation{})
	var conflict *pkgerrors.ConflictError
	if !errors.As(err, &conflict) || receipt != (Receipt{}) {
		t.Fatalf("publication = %+v, %v; want no receipt and a conflict", receipt, err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("publication replaced the operator directory", err)
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatal("publication wrote into the operator directory", err)
	}
	assertNoProjectionStaging(t, path)
}

func TestProjectRefusesCancellationBeforePublication(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "first"
		if existing {
			name = "replacement"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			var before os.FileInfo
			var marker []byte
			if existing {
				catalog, identity := testCatalog(t, "old", "Old Model")
				if _, err := Project(t.Context(), path, catalog, identity); err != nil {
					t.Fatal(err)
				}
				var err error
				before, err = os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				marker, err = os.ReadFile(projectionMarkerPath(path))
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			catalog, identity := testCatalog(t, "new", "New Model")
			receipt, err := (projector{beforePromote: func() error { cancel(); return nil }}).
				project(ctx, path, catalog, identity, InputExpectation{})
			if !errors.Is(err, context.Canceled) || receipt != (Receipt{}) {
				t.Fatalf("canceled publication = %+v, %v", receipt, err)
			}
			if existing {
				after, err := os.Stat(path)
				if err != nil || !os.SameFile(before, after) {
					t.Fatal("cancellation changed workspace identity", err)
				}
				actual, err := os.ReadFile(projectionMarkerPath(path))
				if err != nil || !bytes.Equal(marker, actual) {
					t.Fatal("cancellation changed the projection marker", err)
				}
				assertWorkspaceModel(t, path, "old", "Old Model")
				assertWorkspaceModelMissing(t, path, "new")
			} else {
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatal("cancellation created a workspace", err)
				}
				if _, err := os.Lstat(projectionMarkerPath(path)); !os.IsNotExist(err) {
					t.Fatal("cancellation created a marker", err)
				}
			}
			assertNoProjectionStaging(t, path)
		})
	}
}
