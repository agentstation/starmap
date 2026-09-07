package runtime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func directoryMigrationRequestFixture(t *testing.T) DirectoryMigrationRequest {
	t.Helper()
	root, manifest := migrationJournalFixture(t)
	if err := os.MkdirAll(filepath.Join(manifest.SourceDirectory, layerDirectoryName), runtimeDirectoryMode); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"catalog-runtime/instance-seed": "0123456789abcdef0123456789abcdef",
		"catalog-runtime/layers.json":   "retained runtime bytes",
	} {
		if err := os.WriteFile(filepath.Join(manifest.SourceDirectory, name), []byte(content), ownerRecordMode); err != nil {
			t.Fatal(err)
		}
	}
	return DirectoryMigrationRequest{
		OperationID: manifest.OperationID, SourceDirectory: manifest.SourceDirectory, TargetDirectory: manifest.TargetDirectory,
		JournalRoot: root, SourceIdentity: manifest.SourceIdentity, Owner: manifest.Owner,
	}
}

func TestPrepareDirectoryMigrationPreservesLegacyFiles(t *testing.T) {
	t.Parallel()
	request := directoryMigrationRequestFixture(t)
	result, err := PrepareDirectoryMigration(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Phase != "prepared" || result.FileCount != 2 || result.IdentityVerified {
		t.Fatalf("preparation = %+v", result)
	}
	raw, err := os.ReadFile(filepath.Join(result.JournalDirectory, migrationManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest directoryMigrationManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SourceIdentity != request.SourceIdentity || manifest.Owner != request.Owner {
		t.Fatal("preparation changed requested identity")
	}
	for _, file := range manifest.Files {
		original, err := os.ReadFile(filepath.Join(request.SourceDirectory, file.Source))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(original)
		if file.SHA256 != hex.EncodeToString(digest[:]) || file.Size != int64(len(original)) {
			t.Fatal("manifest does not describe the source bytes")
		}
		if file.Source == "catalog-runtime/instance-seed" && file.Target != instanceSeedFileName {
			t.Fatal("manifest did not relocate the legacy seed")
		}
	}
	if _, err := os.Lstat(request.TargetDirectory); !os.IsNotExist(err) {
		t.Fatal("preparation created the target")
	}
	repeated, err := PrepareDirectoryMigration(t.Context(), request)
	if err != nil || repeated != result {
		t.Fatalf("repeated preparation = %+v, %v", repeated, err)
	}
	if err := os.WriteFile(filepath.Join(request.SourceDirectory, "catalog-runtime/layers.json"), []byte("changed"), ownerRecordMode); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareDirectoryMigration(t.Context(), request); err == nil {
		t.Fatal("repeated operation accepted a different source snapshot")
	}
	after, err := os.ReadFile(filepath.Join(result.JournalDirectory, migrationManifestName))
	if err != nil || !bytes.Equal(raw, after) {
		t.Fatal("conflict replaced the retained manifest")
	}
}

func TestPrepareDirectoryMigrationVerifiesOwnedIdentity(t *testing.T) {
	t.Parallel()
	for _, explicit := range []string{"", "explicit-instance"} {
		t.Run(explicit, func(t *testing.T) {
			root, manifest := migrationJournalFixture(t)
			connected := openTestRuntime(t, WithStateDirectory(manifest.SourceDirectory), WithSchedulerIdentity(explicit))
			identity := connected.Status().InstanceIdentity
			if err := connected.Close(); err != nil {
				t.Fatal(err)
			}
			request := DirectoryMigrationRequest{
				OperationID: manifest.OperationID, SourceDirectory: manifest.SourceDirectory, TargetDirectory: manifest.TargetDirectory,
				JournalRoot: root, SourceIdentity: identity, Owner: manifest.Owner,
			}
			result, err := PrepareDirectoryMigration(t.Context(), request)
			if err != nil || !result.IdentityVerified {
				t.Fatalf("owned preparation = %+v, %v", result, err)
			}
			request.OperationID = "different-operation"
			request.SourceIdentity = "wrong-identity"
			if _, err := PrepareDirectoryMigration(t.Context(), request); err == nil {
				t.Fatal("migration accepted an identity that differs from the source record")
			}
		})
	}
}

func TestPrepareDirectoryMigrationRefusesUnsafeSources(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"locked", "target exists", "source symlink", "file symlink", "missing seed", "invalid seed", "journal alias"} {
		t.Run(change, func(t *testing.T) {
			request := directoryMigrationRequestFixture(t)
			switch change {
			case "locked":
				lock, err := acquireDirectory(t.Context(), request.SourceDirectory)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = lock.Close() }()
			case "target exists":
				if err := os.Mkdir(request.TargetDirectory, runtimeDirectoryMode); err != nil {
					t.Fatal(err)
				}
			case "source symlink":
				alias := request.SourceDirectory + "-alias"
				if err := os.Symlink(request.SourceDirectory, alias); err != nil {
					t.Fatal(err)
				}
				request.SourceDirectory = alias
			case "file symlink":
				if err := os.Symlink(filepath.Join(request.SourceDirectory, "catalog-runtime/layers.json"), filepath.Join(request.SourceDirectory, "alias")); err != nil {
					t.Fatal(err)
				}
			case "missing seed":
				if err := os.Remove(filepath.Join(request.SourceDirectory, "catalog-runtime/instance-seed")); err != nil {
					t.Fatal(err)
				}
			case "invalid seed":
				if err := os.WriteFile(filepath.Join(request.SourceDirectory, "catalog-runtime/instance-seed"), []byte("invalid"), ownerRecordMode); err != nil {
					t.Fatal(err)
				}
			case "journal alias":
				alias := request.SourceDirectory + "-alias"
				if err := os.Symlink(request.SourceDirectory, alias); err != nil {
					t.Fatal(err)
				}
				request.JournalRoot = filepath.Join(alias, "journals")
			}
			if _, err := PrepareDirectoryMigration(t.Context(), request); err == nil {
				t.Fatal("unsafe migration preparation succeeded")
			}
			if _, err := os.Lstat(request.JournalRoot); !os.IsNotExist(err) {
				t.Fatal("refused source created a journal")
			}
		})
	}
}
