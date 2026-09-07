package runtime

import (
	"context"
	stderrors "errors"
	"os"

	"github.com/agentstation/starmap/pkg/errors"
)

// WithPublishedDirectoryMigration requires the exact published target during startup.
// The host must also select the request's directory, owner, and scheduler identity.
// Open verifies migration records under its directory lock before persistent initialization.
func WithPublishedDirectoryMigration(request DirectoryMigrationRequest) Option {
	return func(config *options) error {
		migration := request
		config.publishedMigration = &migration
		return nil
	}
}

func (config options) validatePublishedMigration() error {
	request := config.publishedMigration
	if request == nil {
		return nil
	}
	if config.stateDirectory != request.TargetDirectory || config.directoryOwner != request.Owner || config.schedulerIdentity != request.SourceIdentity {
		return migrationJournalConflict("runtime options do not select the published migration")
	}
	manifest := directoryMigrationManifest{SchemaVersion: 1, OperationID: request.OperationID,
		SourceDirectory: request.SourceDirectory, TargetDirectory: request.TargetDirectory,
		SourceIdentity: request.SourceIdentity, Owner: request.Owner}
	if err := manifest.validateIntent(); err != nil {
		return err
	}
	if err := validateMigrationLocations(request.JournalRoot, request.SourceDirectory, request.TargetDirectory); err != nil {
		return err
	}
	info, err := os.Lstat(request.TargetDirectory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return invalidMigrationIntent("target_directory")
	}
	return nil
}

// VerifyDirectoryMigrationPublication verifies publication before a host opens the target.
// It verifies the source inventory, journal, receipts, owner, and seed without starting a runtime.
// Journal recovery can preserve and truncate an interrupted final event.
func VerifyDirectoryMigrationPublication(ctx context.Context, request DirectoryMigrationRequest) error {
	if err := verifyDirectoryMigrationPublication(ctx, request); err != nil {
		return errors.WrapResource("verify", "runtime directory migration", request.OperationID, err)
	}
	return nil
}

func verifyDirectoryMigrationPublication(ctx context.Context, request DirectoryMigrationRequest) (resultErr error) {
	operation, err := openMigrationOperation(ctx, request, true)
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, operation.Close()) }()
	info, err := os.Lstat(request.TargetDirectory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return invalidMigrationIntent("target_directory")
	}
	if err := refusePendingMigration(request.TargetDirectory); err != nil {
		return err
	}
	if err := refuseRetiredMigration(request.TargetDirectory); err != nil {
		return err
	}
	root, err := os.OpenRoot(request.TargetDirectory)
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	return operation.verifyPublishedTarget(ctx, root)
}
