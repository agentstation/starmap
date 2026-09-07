package runtime

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/pkg/errors"
)

// DirectoryMigrationPublication identifies the replacement and its migration phase.
// SchedulerIdentity must accompany the target directory in the replacement runtime's options.
type DirectoryMigrationPublication struct {
	JournalDirectory  string `json:"journal_directory"`
	TargetDirectory   string `json:"target_directory"`
	SchedulerIdentity string `json:"scheduler_identity"`
	Phase             string `json:"phase"`
}

// PublishDirectoryMigration stages and publishes a verified copy, then retires the source.
// It preserves source data files and does not edit host configuration or complete root selection.
// Older source binaries must stay stopped because they do not honor retirement records.
func PublishDirectoryMigration(ctx context.Context, request DirectoryMigrationRequest) (DirectoryMigrationPublication, error) {
	result, err := publishDirectoryMigration(ctx, request, nil)
	if err != nil {
		return DirectoryMigrationPublication{}, errors.WrapResource("publish", "runtime directory migration", request.OperationID, err)
	}
	return result, nil
}

func publishDirectoryMigration(ctx context.Context, request DirectoryMigrationRequest, checkpoint migrationCheckpoint) (result DirectoryMigrationPublication, resultErr error) {
	operation, err := openMigrationOperation(ctx, request, true)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, operation.Close()) }()
	if _, err := os.Lstat(request.TargetDirectory); os.IsNotExist(err) {
		if err := operation.publishStage(ctx, checkpoint); err != nil {
			return result, err
		}
	} else if err != nil {
		return result, err
	}
	if err := operation.activatePublication(ctx, checkpoint); err != nil {
		return result, err
	}
	return DirectoryMigrationPublication{
		JournalDirectory: operation.journal.directory, TargetDirectory: request.TargetDirectory,
		SchedulerIdentity: request.SourceIdentity, Phase: string(operation.journal.phase),
	}, nil
}

func (m *directoryMigration) publishStage(ctx context.Context, checkpoint migrationCheckpoint) (resultErr error) {
	stage, err := stageMigrationOperation(ctx, m, checkpoint)
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, stage.Close()) }()
	if err := validateMigrationCatalog(ctx, stage.directory); err != nil {
		return err
	}
	if err := stage.Close(); err != nil {
		return err
	}
	parent, err := os.OpenRoot(filepath.Dir(m.manifest.TargetDirectory))
	if err != nil {
		return err
	}
	defer func() { _ = parent.Close() }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := publishMigrationDirectory(parent, filepath.Base(stage.directory), filepath.Base(m.manifest.TargetDirectory)); err != nil {
		return err
	}
	if err := syncMigrationDirectory(parent); err != nil {
		return err
	}
	return migrationReached(checkpoint, "renamed", m.manifest.TargetDirectory)
}

func (m *directoryMigration) activatePublication(ctx context.Context, checkpoint migrationCheckpoint) (resultErr error) {
	directory := m.manifest.TargetDirectory
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return invalidMigrationIntent("target_directory")
	}
	if err := refuseRetiredMigration(directory); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	if _, err := root.Lstat(migrationPendingName); os.IsNotExist(err) {
		return m.verifyPublishedTarget(ctx, root)
	} else if err != nil {
		return err
	}
	if m.journal.phase != migrationVerified && m.journal.phase != migrationPromoted {
		return migrationJournalConflict("target publication has no verified journal state")
	}
	if err := verifyMigrationRecord(root, migrationPendingName, m.manifest); err != nil {
		return err
	}
	lock, err := acquireDirectory(ctx, directory)
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, lock.Close()) }()
	if err := verifyMigrationStage(ctx, root, m.manifest, true); err != nil {
		return err
	}
	if err := validateMigrationCatalog(ctx, directory); err != nil {
		return err
	}
	if err := m.journal.advance(ctx, migrationPromoted); err != nil {
		return err
	}
	if err := migrationReached(checkpoint, "promoted", directory); err != nil {
		return err
	}
	if err := writeMigrationRecord(ctx, m.source, migrationRetiredName, m.manifest); err != nil {
		return err
	}
	if err := migrationReached(checkpoint, "source-retired", directory); err != nil {
		return err
	}
	if err := writeMigrationRecord(ctx, root, migrationReceiptName, m.manifest); err != nil {
		return err
	}
	if err := migrationReached(checkpoint, "receipt-written", directory); err != nil {
		return err
	}
	if err := verifyMigrationRecord(root, migrationPendingName, m.manifest); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := root.Remove(migrationPendingName); err != nil {
		return err
	}
	if err := syncMigrationDirectory(root); err != nil {
		return err
	}
	return migrationReached(checkpoint, "target-ready", directory)
}

func (m *directoryMigration) verifyPublishedTarget(ctx context.Context, root *os.Root) error {
	if m.journal.phase != migrationPromoted && m.journal.phase != migrationCompleted {
		return migrationJournalConflict("published target has no publication journal state")
	}
	if err := verifyMigrationRecord(root, migrationReceiptName, m.manifest); err != nil {
		return err
	}
	if err := verifyMigrationRecord(m.source, migrationRetiredName, m.manifest); err != nil {
		return err
	}
	if _, err := root.Lstat(migrationCompletionName); err == nil {
		if m.journal.phase != migrationCompleted {
			return migrationJournalConflict("completion record precedes journal completion")
		}
		if err := verifyMigrationRecord(root, migrationCompletionName, m.manifest); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return verifyMigrationTargetIdentity(ctx, root, m.manifest)
}

func verifyMigrationTargetIdentity(ctx context.Context, root *os.Root, manifest directoryMigrationManifest) error {
	owner, err := encodeOwnerRecord(manifest.Owner, manifest.SourceIdentity)
	if err != nil {
		return err
	}
	if err := verifyOwnerRecord(root, owner); err != nil {
		return err
	}
	for _, file := range manifest.Files {
		if file.Target != instanceSeedFileName {
			continue
		}
		present, err := migrationFilePresent(ctx, root, file)
		if err != nil {
			return err
		}
		if !present {
			return invalidMigrationIntent("missing_published_seed")
		}
	}
	return nil
}
