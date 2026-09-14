package runtime

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

func migrationFilePresent(ctx context.Context, root *os.Root, expected directoryMigrationFile) (bool, error) {
	actual, err := inspectMigrationFile(ctx, root, expected.Target)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if actual.Size != expected.Size || actual.SHA256 != expected.SHA256 {
		return false, invalidMigrationIntent("staged_file_checksum")
	}
	return true, nil
}

func verifyMigrationStage(ctx context.Context, root *os.Root, manifest directoryMigrationManifest, complete bool) error {
	encoded, err := manifest.encode()
	if err != nil {
		return err
	}
	actual, err := readMigrationFile(root, migrationPendingName, migrationManifestMaxBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, encoded) {
		return invalidMigrationIntent("stage_manifest")
	}
	if _, err := root.Lstat(migrationReceiptName); err == nil {
		if err := verifyMigrationRecord(root, migrationReceiptName, manifest); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := bindDirectoryOwner(ctx, root.Name(), manifest.Owner, manifest.SourceIdentity); err != nil {
		return err
	}
	if err := verifyMigrationStageLayout(ctx, root, manifest, complete); err != nil {
		return err
	}
	for _, file := range manifest.Files {
		present, err := migrationFilePresent(ctx, root, file)
		if err != nil {
			return err
		}
		if !present && complete {
			return invalidMigrationIntent("missing_staged_file")
		}
	}
	return nil
}

func verifyMigrationStageLayout(ctx context.Context, root *os.Root, manifest directoryMigrationManifest, complete bool) error {
	files := map[string]bool{ownerRecordName: true, directoryLockName: true, migrationPendingName: true, migrationReceiptName: true}
	directories := map[string]bool{".": true, migrationWorkDirectory: true}
	for _, file := range manifest.Files {
		files[file.Target] = true
		if !complete {
			files[migrationPartialName(file.Target)] = true
			files[migrationPartialName(file.Target)+".json"] = true
		}
		for parent := path.Dir(file.Target); parent != "."; parent = path.Dir(parent) {
			directories[parent] = true
		}
	}
	return walkMigrationTree(ctx, root, len(files)+len(directories), func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() && directories[name] {
			return nil
		}
		if !entry.Type().IsRegular() || !files[name] {
			return invalidMigrationIntent("unexpected_staged_entry")
		}
		info, err := root.Lstat(filepath.FromSlash(name))
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return invalidMigrationIntent("staged_entry_type")
		}
		return nil
	})
}
