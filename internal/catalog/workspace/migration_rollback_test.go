package workspace

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestLegacyRollbackPreservesProjectedOperatorChanges(t *testing.T) {
	for _, change := range []string{"unknown-file", "changed-file", "replacement-file", "replacement-root"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
			store := migrationStore(t, legacy)
			generation := migrationGeneration(t, "rollback-operator-change", "model", "Model")
			if err := store.Commit(t.Context(), generation, ""); err != nil {
				t.Fatal(err)
			}
			fault := stderrors.New("stop after projected operator change")
			var preserved string
			var want []byte
			m := legacyLayoutMigrator{projector: projector{beforeMarker: func() error {
				preserved = filepath.Join(legacy, "providers.yaml")
				var err error
				want, err = os.ReadFile(preserved)
				if err != nil {
					return err
				}
				switch change {
				case "unknown-file":
					preserved, want = filepath.Join(legacy, "operator-note"), []byte("preserve this note")
				case "changed-file":
					want = append(want, []byte("\n# operator comment\n")...)
				case "replacement-file":
					if err := os.Rename(preserved, filepath.Join(root, "original-providers.yaml")); err != nil {
						return err
					}
				case "replacement-root":
					original := legacy + ".original"
					if err := os.Rename(legacy, original); err != nil {
						return err
					}
					if err := os.CopyFS(legacy, os.DirFS(original)); err != nil {
						return err
					}
				}
				if err := os.WriteFile(preserved, want, fileMode); err != nil {
					return err
				}
				return fault
			}}}
			_, err := m.migrate(t.Context(), legacy, state)
			if !stderrors.Is(err, fault) {
				t.Fatalf("migration did not retain the original failure: %v", err)
			}
			data, readErr := os.ReadFile(preserved)
			if readErr != nil || !bytes.Equal(data, want) {
				t.Fatalf("rollback removed operator content: %q, %v", data, readErr)
			}
			var conflict *errors.ConflictError
			if !stderrors.As(err, &conflict) {
				t.Fatalf("rollback did not report an ownership conflict: %v", err)
			}
			current, err := migrationStore(t, state).Current(t.Context())
			if err != nil || !sameMigrationGeneration(generation, current) {
				t.Fatalf("relocated store changed: %v", err)
			}
		})
	}
}

func TestLegacyRollbackCancellationRestoresOwnedWorkspace(t *testing.T) {
	root := t.TempDir()
	legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
	generation := migrationGeneration(t, "rollback-canceled", "model", "Model")
	if err := migrationStore(t, legacy).Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	before := migrationTree(t, legacy)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, err := (legacyLayoutMigrator{projector: projector{beforeMarker: func() error {
		cancel()
		return ctx.Err()
	}}}).migrate(ctx, legacy, state)
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("migration cancellation: %v", err)
	}
	if after := migrationTree(t, legacy); !reflect.DeepEqual(before, after) {
		t.Fatal("canceled rollback changed the original store")
	}
	if _, err := os.Lstat(state); !stderrors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled rollback left relocated state: %v", err)
	}
}

func TestLegacyRollbackRetainsWriterLockIdentity(t *testing.T) {
	root := t.TempDir()
	legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
	generation := migrationGeneration(t, "rollback-lock-identity", "model", "Model")
	if err := migrationStore(t, legacy).Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	var before os.FileInfo
	fault := stderrors.New("stop with the writer lock held")
	_, err := (legacyLayoutMigrator{afterMove: func() error {
		var err error
		before, err = os.Stat(writerLockPath(legacy))
		if err != nil {
			return err
		}
		return fault
	}}).migrate(t.Context(), legacy, state)
	if !stderrors.Is(err, fault) {
		t.Fatalf("migration failure: %v", err)
	}
	after, err := os.Stat(writerLockPath(legacy))
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("rollback replaced the lock identity: %v", err)
	}
	release, err := acquireWriterLock(legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	contender, err := acquireWriterLock(legacy)
	if contender != nil {
		contender()
		t.Fatal("two writers acquired the retained lock")
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("retained lock did not exclude a second writer: %v", err)
	}
}

func TestLegacyStoreMoveNeverReplacesDirectories(t *testing.T) {
	for _, phase := range []string{"relocate", "restore"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state")
			if err := os.Mkdir(legacy, directoryMode); err != nil {
				t.Fatal(err)
			}
			identity, err := captureLegacyStoreIdentity(legacy)
			if err != nil {
				t.Fatal(err)
			}
			move, err := prepareLegacyStoreMove(legacy, state, identity)
			if err != nil {
				t.Fatal(err)
			}
			defer move.close()
			destination := state
			operation := move.relocate
			if phase == "restore" {
				if err := move.relocate(); err != nil {
					t.Fatal(err)
				}
				destination, operation = legacy, move.restore
			}
			if err := os.Mkdir(destination, directoryMode); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(destination)
			if err != nil {
				t.Fatal(err)
			}
			if err := operation(); !stderrors.Is(err, os.ErrExist) {
				t.Fatalf("%s replaced an existing directory: %v", phase, err)
			}
			after, err := os.Stat(destination)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("destination identity changed: %v", err)
			}
			parent, name := move.originalParent, move.originalName
			if phase == "restore" {
				parent, name = move.stateParent, move.stateName
			}
			if err := move.checkStore(parent, name); err != nil {
				t.Fatalf("owned store moved after refusal: %v", err)
			}
		})
	}
}

func TestLegacyRollbackPreservesOperatorSidecars(t *testing.T) {
	for _, sidecar := range []string{"projection-marker", "writer-lock"} {
		t.Run(sidecar, func(t *testing.T) {
			root := t.TempDir()
			legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
			generation := migrationGeneration(t, "rollback-sidecar", "model", "Model")
			if err := migrationStore(t, legacy).Commit(t.Context(), generation, ""); err != nil {
				t.Fatal(err)
			}
			preserved := projectionMarkerPath(legacy)
			if sidecar == "writer-lock" {
				preserved = writerLockPath(legacy)
			}
			fault := stderrors.New("stop after sidecar write")
			_, err := (legacyLayoutMigrator{afterMove: func() error {
				if err := os.WriteFile(preserved, []byte("operator sidecar"), fileMode); err != nil {
					return err
				}
				return fault
			}}).migrate(t.Context(), legacy, state)
			if !stderrors.Is(err, fault) {
				t.Fatalf("migration failure: %v", err)
			}
			data, readErr := os.ReadFile(preserved)
			if readErr != nil || string(data) != "operator sidecar" {
				t.Fatalf("rollback removed an operator sidecar: %q, %v", data, readErr)
			}
			current, err := migrationStore(t, legacy).Current(t.Context())
			if err != nil || !sameMigrationGeneration(generation, current) {
				t.Fatalf("original store did not return: %v", err)
			}
		})
	}
}

func TestLegacyRollbackPreservesReplacementStore(t *testing.T) {
	root := t.TempDir()
	legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
	generation := migrationGeneration(t, "rollback-replaced-store", "model", "Model")
	if err := migrationStore(t, legacy).Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	fault := stderrors.New("stop after store replacement")
	_, err := (legacyLayoutMigrator{afterMove: func() error {
		if err := os.Rename(state, state+".original"); err != nil {
			return err
		}
		if err := os.Mkdir(state, directoryMode); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(state, "operator-note"), []byte("replacement store"), fileMode); err != nil {
			return err
		}
		return fault
	}}).migrate(context.Background(), legacy, state)
	if !stderrors.Is(err, fault) {
		t.Fatalf("migration failure: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(state, "operator-note"))
	if readErr != nil || string(data) != "replacement store" {
		t.Fatalf("rollback moved an unowned store: %q, %v", data, readErr)
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("rollback did not report a replaced store: %v", err)
	}
	current, err := migrationStore(t, state+".original").Current(t.Context())
	if err != nil || !sameMigrationGeneration(generation, current) {
		t.Fatalf("original retained store changed: %v", err)
	}
}
