package runtime

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/errors"
)

// DirectoryMigrationRequest selects an explicit runtime-directory migration.
// Stop the source process before preparation, including older binaries without directory locks.
type DirectoryMigrationRequest struct {
	OperationID     string
	SourceDirectory string
	TargetDirectory string
	JournalRoot     string
	SourceIdentity  string
	Owner           DirectoryOwner
}

// DirectoryMigrationPreparation identifies the retained inventory and its journal.
// IdentityVerified is false for legacy state without an ownership record.
type DirectoryMigrationPreparation struct {
	JournalDirectory string `json:"journal_directory"`
	Phase            string `json:"phase"`
	FileCount        int    `json:"file_count"`
	IdentityVerified bool   `json:"identity_verified"`
}

// PrepareDirectoryMigration records source checksums under an exclusive directory lock.
// It preserves source files, creates no target, and leaves configured roots unchanged.
// SourceIdentity must come from the old deployment. Legacy seeds alone cannot prove it.
func PrepareDirectoryMigration(ctx context.Context, request DirectoryMigrationRequest) (DirectoryMigrationPreparation, error) {
	result, err := prepareDirectoryMigration(ctx, request)
	if err != nil {
		return DirectoryMigrationPreparation{}, errors.WrapResource("prepare", "runtime directory migration", request.OperationID, err)
	}
	return result, nil
}

func prepareDirectoryMigration(ctx context.Context, request DirectoryMigrationRequest) (DirectoryMigrationPreparation, error) {
	operation, err := openDirectoryMigration(ctx, request)
	if err != nil {
		return DirectoryMigrationPreparation{}, err
	}
	result := operation.preparation()
	return result, operation.Close()
}

type directoryMigration struct {
	source   *os.Root
	lock     *flock.Flock
	journal  *directoryMigrationJournal
	manifest directoryMigrationManifest
}

func (m *directoryMigration) preparation() DirectoryMigrationPreparation {
	return DirectoryMigrationPreparation{
		JournalDirectory: m.journal.directory, Phase: string(m.journal.phase), FileCount: len(m.manifest.Files),
		IdentityVerified: m.manifest.SourceOwnerSHA256 != "",
	}
}

func (m *directoryMigration) Close() error {
	var journalErr, sourceErr error
	if m.journal != nil {
		journalErr = m.journal.Close()
	}
	if m.source != nil {
		sourceErr = m.source.Close()
	}
	return stderrors.Join(journalErr, sourceErr, m.lock.Close())
}

func openDirectoryMigration(ctx context.Context, request DirectoryMigrationRequest) (_ *directoryMigration, resultErr error) {
	return openMigrationOperation(ctx, request, false)
}

func openMigrationOperation(ctx context.Context, request DirectoryMigrationRequest, publication bool) (_ *directoryMigration, resultErr error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	manifest := directoryMigrationManifest{
		SchemaVersion: 1, OperationID: request.OperationID, SourceDirectory: request.SourceDirectory,
		TargetDirectory: request.TargetDirectory, SourceIdentity: request.SourceIdentity, Owner: request.Owner,
	}
	if err := manifest.validateIntent(); err != nil {
		return nil, err
	}
	if err := validateMigrationLocations(request.JournalRoot, request.SourceDirectory, request.TargetDirectory); err != nil {
		return nil, err
	}
	info, err := os.Lstat(request.SourceDirectory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, invalidMigrationIntent("source_directory")
	}
	if _, err := os.Lstat(request.TargetDirectory); err == nil && !publication {
		return nil, &errors.ConflictError{Resource: "runtime migration target", Message: "target already exists; select an unused directory"}
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	lock, err := acquireDirectory(ctx, request.SourceDirectory)
	if err != nil {
		return nil, err
	}
	operation := &directoryMigration{lock: lock}
	defer func() {
		if resultErr != nil {
			_ = operation.Close()
		}
	}()
	if err := refusePendingMigration(request.SourceDirectory); err != nil {
		return nil, err
	}
	if !publication {
		if err := refuseRetiredMigration(request.SourceDirectory); err != nil {
			return nil, err
		}
	}
	operation.source, err = os.OpenRoot(request.SourceDirectory)
	if err != nil {
		return nil, err
	}
	manifest.SourceOwnerSHA256, err = verifyMigrationSourceIdentity(operation.source, request.SourceIdentity)
	if err != nil {
		return nil, err
	}
	manifest.Files, err = inspectMigrationFiles(ctx, operation.source)
	if err != nil {
		return nil, err
	}
	manifest.SourceReceiptSHA256, err = migrationSourceReceipt(operation.source, request.SourceDirectory, request.SourceIdentity)
	if err != nil {
		return nil, err
	}
	if _, err := operation.source.Lstat(migrationRetiredName); err == nil {
		if err := verifyMigrationRecord(operation.source, migrationRetiredName, manifest); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	operation.journal, err = openDirectoryMigrationJournal(ctx, request.JournalRoot, manifest)
	if err != nil {
		return nil, err
	}
	operation.manifest = manifest
	return operation, nil
}

func refusePendingMigration(directory string) error {
	return checkMigrationMarker(directory, migrationPendingName, "directory contains a pending migration; resume that operation before startup")
}

func refuseRetiredMigration(directory string) error {
	return checkMigrationMarker(directory, migrationRetiredName, "directory is retired by migration; select its replacement instead of restarting this source")
}

func checkMigrationMarker(directory, name, message string) error {
	if directory == "" {
		return nil
	}
	if _, err := os.Lstat(filepath.Join(directory, name)); err == nil {
		return &errors.ConflictError{Resource: "runtime directory migration", Message: message}
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}
