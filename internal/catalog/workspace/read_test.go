package workspace

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestReadRefusesPendingReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	journal := filepath.Join(filepath.Dir(path), ".workspace.starmap-replacement.json")
	if err := os.WriteFile(journal, []byte("{}\n"), fileMode); err != nil {
		t.Fatal(err)
	}
	input, err := ObserveInput(path)
	assertReadConflict(t, err)
	if input != (InputExpectation{}) {
		t.Fatalf("pending replacement produced input: %+v", input)
	}
	data, err := os.ReadFile(journal)
	if err != nil || string(data) != "{}\n" {
		t.Fatal("passive read changed the journal", err)
	}
}

func TestReadPreservesPassiveFiles(t *testing.T) {
	for _, mode := range []string{"absent_parent", "absent", "authored", "existing_lock"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "workspace")
			switch mode {
			case "absent_parent":
				path = filepath.Join(root, "missing", "workspace")
			case "authored", "existing_lock":
				if err := os.Mkdir(path, directoryMode); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(path, "notes.txt"), []byte("operator notes"), fileMode); err != nil {
					t.Fatal(err)
				}
				if mode == "existing_lock" {
					release, err := acquireWriterLock(path)
					if err != nil {
						t.Fatal(err)
					}
					release()
				}
			}
			before := migrationTree(t, root)
			called := false
			if err := Read(t.Context(), path, func(input InputExpectation) error {
				called = true
				wantPresent := mode == "authored" || mode == "existing_lock"
				if input.Path != path || input.Exists != wantPresent {
					t.Fatalf("unexpected input: %+v", input)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if !called || !reflect.DeepEqual(before, migrationTree(t, root)) {
				t.Fatal("passive read did not run or changed files")
			}
		})
	}
}

func TestReadExcludesWritersUntilCallbackReturns(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "callback_error"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			release, err := acquireWriterLock(path)
			if err != nil {
				t.Fatal(err)
			}
			release()
			fault := stderrors.New("callback failed")
			err = Read(t.Context(), path, func(InputExpectation) error {
				if err := Read(t.Context(), path, func(InputExpectation) error { return nil }); err != nil {
					t.Fatalf("concurrent shared read: %v", err)
				}
				writer, err := acquireWriterLock(path)
				if err == nil {
					writer()
					t.Fatal("writer acquired the lock during a read")
				}
				assertReadConflict(t, err)
				if fail {
					return fault
				}
				return nil
			})
			if fail && !stderrors.Is(err, fault) || !fail && err != nil {
				t.Fatalf("Read: %v", err)
			}
			release, err = acquireWriterLock(path)
			if err != nil {
				t.Fatalf("writer after read: %v", err)
			}
			release()
		})
	}
}

func TestReadDetectsFirstWriterAfterItFinishes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	catalog, identity := testCatalog(t, "new", "New Model")
	err := Read(t.Context(), path, func(input InputExpectation) error {
		if input.Exists {
			t.Fatal("workspace exists before first publication")
		}
		_, err := Project(t.Context(), path, catalog, identity)
		return err
	})
	assertReadConflict(t, err)
	assertWorkspaceModel(t, path, "new", "New Model")
	if _, err := ObserveInput(path); err != nil {
		t.Fatalf("read after first writer: %v", err)
	}
}

func TestReadDetectsFirstWriterWithoutDirectoryChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	err := Read(t.Context(), path, func(InputExpectation) error {
		release, err := acquireWriterLock(path)
		if err != nil {
			return err
		}
		release()
		return nil
	})
	assertReadConflict(t, err)
}

func TestReadDetectsExternalDirectoryReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	if err := os.Mkdir(path, directoryMode); err != nil {
		t.Fatal(err)
	}
	err := Read(t.Context(), path, func(InputExpectation) error {
		if err := os.Rename(path, path+".old"); err != nil {
			return err
		}
		return os.Mkdir(path, directoryMode)
	})
	assertReadConflict(t, err)
}

func TestReadRefusesInvalidLockWithoutReading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	if err := os.Mkdir(writerLockPath(path), directoryMode); err != nil {
		t.Fatal(err)
	}
	err := Read(t.Context(), path, func(InputExpectation) error {
		t.Fatal("read under an invalid lock")
		return nil
	})
	var invalid *errors.ValidationError
	if !stderrors.As(err, &invalid) {
		t.Fatalf("Read: %v", err)
	}
}

func TestReadCancellationAndCallbackValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Read(ctx, path, func(InputExpectation) error {
		t.Fatal("read after cancellation")
		return nil
	}); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("Read: %v", err)
	}
	ctx, cancel = context.WithCancel(t.Context())
	defer cancel()
	if err := Read(ctx, path, func(InputExpectation) error {
		cancel()
		return nil
	}); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("Read: %v", err)
	}
	var invalid *errors.ValidationError
	if err := Read(t.Context(), path, nil); !stderrors.As(err, &invalid) {
		t.Fatalf("nil callback: %v", err)
	}
}

func assertReadConflict(t testing.TB, err error) {
	t.Helper()
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("error = %v; want a retryable conflict", err)
	}
}

func TestObserveInputRefusesActiveWriter(t *testing.T) {
	for _, present := range []bool{false, true} {
		name := "absent"
		if present {
			name = "present"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "workspace")
			if present {
				if err := os.Mkdir(path, directoryMode); err != nil {
					t.Fatal(err)
				}
			}
			release, err := acquireWriterLock(path)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			input, err := ObserveInput(path)
			var conflict *errors.ConflictError
			if !stderrors.As(err, &conflict) || input != (InputExpectation{}) {
				t.Fatalf("input during writer = %+v, %v; want no input and a conflict", input, err)
			}
		})
	}
}
