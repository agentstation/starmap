package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"os"
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
)

// DirectoryMigrationCompletion binds a completed move to its retained source and target.
type DirectoryMigrationCompletion struct {
	OperationID       string         `json:"operation_id"`
	SourceDirectory   string         `json:"source_directory"`
	TargetDirectory   string         `json:"target_directory"`
	SchedulerIdentity string         `json:"scheduler_identity"`
	Owner             DirectoryOwner `json:"owner"`
	ManifestSHA256    string         `json:"manifest_sha256"`
}

// ReadDirectoryMigrationCompletion verifies completion and the preserved source without writing files.
// It requires the source inventory, retirement record, target receipts, owner, and seed to match.
// It does not read the operation journal or compare mutable target catalog layers.
func ReadDirectoryMigrationCompletion(ctx context.Context, directory string) (DirectoryMigrationCompletion, error) {
	result, err := readDirectoryMigrationCompletion(ctx, directory)
	if err != nil {
		return DirectoryMigrationCompletion{}, errors.WrapResource("read", "runtime migration completion", directory, err)
	}
	return result, nil
}

func readDirectoryMigrationCompletion(ctx context.Context, directory string) (result DirectoryMigrationCompletion, resultErr error) {
	if ctx == nil {
		return result, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	root, err := openMigrationReceiptRoot(directory)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	if err := refusePendingMigration(directory); err != nil {
		return result, err
	}
	if err := refuseRetiredMigration(directory); err != nil {
		return result, err
	}
	encoded, err := readMigrationFile(root, migrationCompletionName, migrationManifestMaxBytes)
	if err != nil {
		return result, err
	}
	var manifest directoryMigrationManifest
	if err := json.Unmarshal(encoded, &manifest); err != nil {
		return result, invalidMigrationIntent("completion_record")
	}
	canonical, err := manifest.encode()
	if err != nil {
		return result, err
	}
	if !bytes.Equal(encoded, canonical) || manifest.TargetDirectory != directory {
		return result, invalidMigrationIntent("completion_record")
	}
	if err := verifyMigrationRecord(root, migrationReceiptName, manifest); err != nil {
		return result, err
	}
	if err := verifyMigrationTargetIdentity(ctx, root, manifest); err != nil {
		return result, err
	}
	if err := verifyCompletedMigrationSource(ctx, manifest); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	digest := sha256.Sum256(encoded)
	return DirectoryMigrationCompletion{OperationID: manifest.OperationID, SourceDirectory: manifest.SourceDirectory,
		TargetDirectory: manifest.TargetDirectory, SchedulerIdentity: manifest.SourceIdentity, Owner: manifest.Owner,
		ManifestSHA256: hex.EncodeToString(digest[:])}, nil
}

func verifyCompletedMigrationSource(ctx context.Context, manifest directoryMigrationManifest) (resultErr error) {
	sourcePath, err := migrationPhysicalPath(manifest.SourceDirectory)
	if err != nil {
		return err
	}
	targetPath, err := migrationPhysicalPath(manifest.TargetDirectory)
	if err != nil {
		return err
	}
	if migrationPathsOverlap(sourcePath, targetPath) {
		return invalidMigrationIntent("overlapping_directories")
	}
	source, err := openMigrationReceiptRoot(manifest.SourceDirectory)
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, source.Close()) }()
	ownerDigest, err := verifyMigrationSourceIdentity(source, manifest.SourceIdentity)
	if err != nil {
		return err
	}
	receiptDigest, err := migrationSourceReceipt(source, manifest.SourceDirectory, manifest.SourceIdentity)
	if err != nil {
		return err
	}
	files, err := inspectMigrationFiles(ctx, source)
	if err != nil {
		return err
	}
	if ownerDigest != manifest.SourceOwnerSHA256 || receiptDigest != manifest.SourceReceiptSHA256 || !slices.Equal(files, manifest.Files) {
		return migrationJournalConflict("preserved source differs from the completed migration")
	}
	return verifyMigrationRecord(source, migrationRetiredName, manifest)
}

func openMigrationReceiptRoot(directory string) (*os.Root, error) {
	if !absoluteMigrationPath(directory) {
		return nil, invalidMigrationIntent("directories")
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, invalidMigrationIntent("directories")
	}
	return os.OpenRoot(directory)
}
