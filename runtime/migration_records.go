package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
)

func verifyMigrationRecord(root *os.Root, name string, manifest directoryMigrationManifest) error {
	expected, err := manifest.encode()
	if err != nil {
		return err
	}
	actual, err := readMigrationFile(root, name, migrationManifestMaxBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return migrationJournalConflict("migration record differs from the operation intent")
	}
	return nil
}

func writeMigrationRecord(ctx context.Context, root *os.Root, name string, manifest directoryMigrationManifest) error {
	encoded, err := manifest.encode()
	if err != nil {
		return err
	}
	if err := writeOwnerFile(ctx, root, name, encoded); err != nil && !os.IsExist(err) {
		return err
	}
	if err := verifyMigrationRecord(root, name, manifest); err != nil {
		return err
	}
	return syncMigrationDirectory(root)
}

func migrationSourceReceipt(root *os.Root, directory, identity string) (string, error) {
	encoded, err := readMigrationFile(root, migrationReceiptName, migrationManifestMaxBytes)
	if os.IsNotExist(err) {
		if _, completedErr := root.Lstat(migrationCompletionName); !os.IsNotExist(completedErr) {
			return "", invalidMigrationIntent("source_receipt")
		}
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var previous directoryMigrationManifest
	if err := json.Unmarshal(encoded, &previous); err != nil {
		return "", invalidMigrationIntent("source_receipt")
	}
	canonical, err := previous.encode()
	if err != nil {
		return "", err
	}
	if !bytes.Equal(canonical, encoded) || previous.TargetDirectory != directory || previous.SourceIdentity != identity {
		return "", invalidMigrationIntent("source_receipt")
	}
	if _, err := root.Lstat(migrationCompletionName); err == nil {
		if err := verifyMigrationRecord(root, migrationCompletionName, previous); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	owner, err := encodeOwnerRecord(previous.Owner, identity)
	if err != nil {
		return "", err
	}
	if err := verifyOwnerRecord(root, owner); err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
