package runtime

import (
	"context"
	stderrors "errors"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

// CompleteDirectoryMigration confirms this runtime's selected directory, owner, and retained identity.
// Call it after the host selects the replacement configuration and opens this runtime.
// This method records completion without editing configuration files or deleting the source.
func (r *Runtime) CompleteDirectoryMigration(ctx context.Context, request DirectoryMigrationRequest) (DirectoryMigrationPublication, error) {
	result, err := r.completeDirectoryMigration(ctx, request, nil)
	if err != nil {
		return DirectoryMigrationPublication{}, errors.WrapResource("complete", "runtime directory migration", request.OperationID, err)
	}
	return result, nil
}

func (r *Runtime) completeDirectoryMigration(ctx context.Context, request DirectoryMigrationRequest, checkpoint migrationCheckpoint) (result DirectoryMigrationPublication, resultErr error) {
	if ctx == nil {
		return result, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if r == nil || r.ctx == nil || r.ctx.Err() != nil {
		return result, migrationJournalConflict("completion requires an active replacement runtime")
	}
	if r.config.stateDirectory != request.TargetDirectory || r.config.directoryOwner != request.Owner || r.Status().InstanceIdentity != request.SourceIdentity {
		return result, migrationJournalConflict("runtime does not select the migration target, owner, and identity")
	}
	active, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(r.ctx, cancel)
	defer stop()
	operation, err := openMigrationOperation(active, request, true)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, operation.Close()) }()
	if err := refusePendingMigration(request.TargetDirectory); err != nil {
		return result, err
	}
	if err := refuseRetiredMigration(request.TargetDirectory); err != nil {
		return result, err
	}
	root, err := os.OpenRoot(request.TargetDirectory)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	if err := operation.verifyPublishedTarget(active, root); err != nil {
		return result, err
	}
	if err := operation.journal.advance(active, migrationCompleted); err != nil {
		return result, err
	}
	if err := migrationReached(checkpoint, "journal-completed", request.TargetDirectory); err != nil {
		return result, err
	}
	if err := writeMigrationRecord(active, root, migrationCompletionName, operation.manifest); err != nil {
		return result, err
	}
	if err := migrationReached(checkpoint, "completion-recorded", request.TargetDirectory); err != nil {
		return result, err
	}
	return DirectoryMigrationPublication{
		JournalDirectory: operation.journal.directory, TargetDirectory: request.TargetDirectory,
		SchedulerIdentity: request.SourceIdentity, Phase: string(operation.journal.phase),
	}, nil
}
