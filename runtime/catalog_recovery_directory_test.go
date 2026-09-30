package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareCatalogRecoveryDirectoryCreatesOnlyStoppedOwnerState(t *testing.T) {
	request, opts, _ := materializationFixture(t)
	request.Directory = filepath.Join(privateRuntimeDirectory(t), "target")
	if err := PrepareCatalogRecoveryDirectory(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity); err != nil {
		t.Fatal(err)
	}
	if err := InspectRetainedDirectory(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity); err != nil {
		t.Fatal(err)
	}
	store, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	files, err := captureMaterializationFiles(t.Context(), store.directory)
	if err != nil || len(files) != 0 {
		t.Fatalf("preparation created catalog or permission state: count=%d error=%v", len(files), err)
	}
	seed, err := os.ReadFile(filepath.Join(request.Directory, instanceSeedFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := PrepareCatalogRecoveryDirectory(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity); err != nil {
		t.Fatal(err)
	}
	repeated, err := os.ReadFile(filepath.Join(request.Directory, instanceSeedFileName))
	if err != nil || !bytes.Equal(seed, repeated) {
		t.Fatal("preparation changed the retained native seed")
	}
	if _, err := MaterializeCatalogRecovery(t.Context(), request, opts...); err != nil {
		t.Fatal(err)
	}
	if err := PrepareCatalogRecoveryDirectory(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity); err == nil {
		t.Fatal("selected materialization permits new preparation")
	}
	if err := InspectRetainedDirectory(t.Context(), request.Directory, request.Owner, request.SchedulerIdentity); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareCatalogRecoveryDirectoryRefusesInvalidOrConflictingOwners(t *testing.T) {
	for _, mode := range []string{"nil-context", "cancelled", "relative", "unclean", "invalid-owner", "different-owner", "different-scheduler", "held", "missing-seed", "invalid-seed", "orphan-state", "pending-migration", "retired"} {
		t.Run(mode, func(t *testing.T) {
			owner := defaults().directoryOwner
			path := filepath.Join(privateRuntimeDirectory(t), "target")
			if err := PrepareCatalogRecoveryDirectory(t.Context(), path, owner, "original"); err != nil {
				t.Fatal(err)
			}
			ctx := t.Context()
			identity := "original"
			switch mode {
			case "nil-context":
				ctx = nil
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "relative":
				path = "relative"
			case "unclean":
				path += string(os.PathSeparator) + "."
			case "invalid-owner":
				owner.Product = "unknown"
			case "different-owner":
				owner.Deployment = "another"
			case "different-scheduler":
				identity = "another"
			case "held":
				lock, err := acquireDirectory(t.Context(), path)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = lock.Close() })
			case "missing-seed", "invalid-seed":
				if err := os.Remove(filepath.Join(path, instanceSeedFileName)); err != nil {
					t.Fatal(err)
				}
				if mode == "invalid-seed" {
					if err := writePrivateRecoveryFixture(t.Context(), path, instanceSeedFileName, []byte("invalid")); err != nil {
						t.Fatal(err)
					}
				}
			case "orphan-state":
				if err := os.Remove(filepath.Join(path, ownerRecordName)); err != nil {
					t.Fatal(err)
				}
			case "pending-migration", "retired":
				name := migrationPendingName
				if mode == "retired" {
					name = migrationRetiredName
				}
				if err := writePrivateRecoveryFixture(t.Context(), path, name, []byte("existing marker")); err != nil {
					t.Fatal(err)
				}
			}
			if err := PrepareCatalogRecoveryDirectory(ctx, path, owner, identity); err == nil {
				t.Fatal("unsafe preparation succeeded")
			}
		})
	}
}

func writePrivateRecoveryFixture(ctx context.Context, path, name string, data []byte) error {
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return writeOwnerFile(ctx, root, name, data)
}
