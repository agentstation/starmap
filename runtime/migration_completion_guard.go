package runtime

import "context"

// WithCompletedDirectoryMigration requires the observed completion record during startup.
// The host must select its target directory, owner, and retained scheduler identity.
// Open rechecks the preserved source and target under its directory lock.
func WithCompletedDirectoryMigration(completion DirectoryMigrationCompletion) Option {
	return func(config *options) error {
		selected := completion
		config.completedMigration = &selected
		return nil
	}
}

func (config options) validateCompletedMigration() error {
	selected := config.completedMigration
	if selected == nil {
		return nil
	}
	if config.stateDirectory != selected.TargetDirectory || config.directoryOwner != selected.Owner || config.schedulerIdentity != selected.SchedulerIdentity || !migrationDigestValid(selected.ManifestSHA256) {
		return migrationJournalConflict("runtime options do not select the completed migration")
	}
	root, err := openMigrationReceiptRoot(selected.TargetDirectory)
	if err != nil {
		return err
	}
	return root.Close()
}

func verifyCompletedMigrationSelection(ctx context.Context, expected DirectoryMigrationCompletion) error {
	actual, err := ReadDirectoryMigrationCompletion(ctx, expected.TargetDirectory)
	if err != nil {
		return err
	}
	if actual != expected {
		return migrationJournalConflict("completion record changed after selection")
	}
	return nil
}
