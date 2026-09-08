package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/pkg/errors"
)

// DirectoryMigrationStage identifies a verified copy that cannot yet serve a runtime.
type DirectoryMigrationStage struct {
	JournalDirectory string `json:"journal_directory"`
	StageDirectory   string `json:"stage_directory"`
	Phase            string `json:"phase"`
	FileCount        int    `json:"file_count"`
	IdentityVerified bool   `json:"identity_verified"`
}

// StageDirectoryMigration copies a stopped source into private staging and verifies its bytes.
// It preserves the source and leaves the selected target and configuration roots unchanged.
func StageDirectoryMigration(ctx context.Context, request DirectoryMigrationRequest) (DirectoryMigrationStage, error) {
	result, err := stageDirectoryMigration(ctx, request, nil)
	if err != nil {
		return DirectoryMigrationStage{}, errors.WrapResource("stage", "runtime directory migration", request.OperationID, err)
	}
	return result, nil
}

type migrationCheckpoint func(event, path string) error

func migrationReached(checkpoint migrationCheckpoint, event, path string) error {
	if checkpoint == nil {
		return nil
	}
	return checkpoint(event, path)
}

func stageDirectoryMigration(ctx context.Context, request DirectoryMigrationRequest, checkpoint migrationCheckpoint) (result DirectoryMigrationStage, resultErr error) {
	operation, err := openDirectoryMigration(ctx, request)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, operation.Close()) }()
	stage, err := stageMigrationOperation(ctx, operation, checkpoint)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = stderrors.Join(resultErr, stage.Close()) }()
	return DirectoryMigrationStage{
		JournalDirectory: operation.journal.directory, StageDirectory: stage.directory, Phase: string(operation.journal.phase),
		FileCount: len(operation.manifest.Files), IdentityVerified: operation.manifest.SourceOwnerSHA256 != "",
	}, nil
}

func stageMigrationOperation(ctx context.Context, operation *directoryMigration, checkpoint migrationCheckpoint) (_ *directoryMigrationStage, resultErr error) {
	if operation.journal.phase != migrationPrepared && operation.journal.phase != migrationCopied && operation.journal.phase != migrationVerified {
		return nil, migrationJournalConflict("operation already passed staging")
	}
	stage, err := operation.openStage(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = stderrors.Join(resultErr, stage.Close())
		}
	}()
	if err := migrationReached(checkpoint, "stage-ready", stage.directory); err != nil {
		return nil, err
	}
	complete := operation.journal.phase != migrationPrepared
	if err := verifyMigrationStage(ctx, stage.root, operation.manifest, complete); err != nil {
		return nil, err
	}
	if operation.journal.phase == migrationPrepared {
		for _, file := range operation.manifest.Files {
			if err := copyMigrationFile(ctx, operation.source, stage.root, file, checkpoint); err != nil {
				return nil, err
			}
		}
		if err := operation.journal.advance(ctx, migrationCopied); err != nil {
			return nil, err
		}
		if err := migrationReached(checkpoint, "copied", stage.directory); err != nil {
			return nil, err
		}
	}
	if err := verifyMigrationStage(ctx, stage.root, operation.manifest, true); err != nil {
		return nil, err
	}
	if err := operation.journal.advance(ctx, migrationVerified); err != nil {
		return nil, err
	}
	if err := migrationReached(checkpoint, "verified", stage.directory); err != nil {
		return nil, err
	}
	return stage, nil
}

type directoryMigrationStage struct {
	root      *os.Root
	lock      *flock.Flock
	directory string
}

func (s *directoryMigrationStage) Close() error {
	var rootErr, lockErr error
	if s.root != nil {
		rootErr = s.root.Close()
		s.root = nil
	}
	if s.lock != nil {
		lockErr = s.lock.Close()
		s.lock = nil
	}
	return stderrors.Join(rootErr, lockErr)
}

func (m *directoryMigration) openStage(ctx context.Context) (_ *directoryMigrationStage, resultErr error) {
	encoded, err := m.manifest.encode()
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	name := ".migration-" + hex.EncodeToString(digest[:])
	parentPath := filepath.Dir(m.manifest.TargetDirectory)
	directory := filepath.Join(parentPath, name)
	if err := validateMigrationLocations(filepath.Dir(m.journal.directory), m.manifest.SourceDirectory, directory); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(parentPath, runtimeDirectoryMode); err != nil {
		return nil, err
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = parent.Close() }()
	if _, err := parent.Lstat(name); os.IsNotExist(err) {
		if err := m.initializeStage(ctx, parent, name, encoded); err != nil && !os.IsExist(err) {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	info, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, invalidMigrationIntent("stage_directory")
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			_ = root.Close()
		}
	}()
	actual, err := readMigrationFile(root, migrationPendingName, migrationManifestMaxBytes)
	if err != nil || !bytes.Equal(actual, encoded) {
		return nil, migrationJournalConflict("staging directory belongs to different or incomplete intent")
	}
	lock, err := acquireDirectory(ctx, directory)
	if err != nil {
		return nil, err
	}
	return &directoryMigrationStage{root: root, lock: lock, directory: directory}, nil
}

func (m *directoryMigration) initializeStage(ctx context.Context, parent *os.Root, name string, encoded []byte) error {
	temporary := ".migration-build-" + rand.Text()
	if err := createPrivateRuntimeChild(parent, temporary); err != nil {
		return err
	}
	defer func() { _ = parent.RemoveAll(temporary) }()
	root, err := parent.OpenRoot(temporary)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := writeOwnerFile(ctx, root, migrationPendingName, encoded); err != nil {
		return err
	}
	if err := bindDirectoryOwner(ctx, filepath.Join(parent.Name(), temporary), m.manifest.Owner, m.manifest.SourceIdentity); err != nil {
		return err
	}
	if err := root.Mkdir(migrationWorkDirectory, runtimeDirectoryMode); err != nil {
		return err
	}
	if err := syncMigrationDirectory(root); err != nil {
		return err
	}
	if err := root.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := publishMigrationDirectory(parent, temporary, name); err != nil {
		return err
	}
	return syncMigrationDirectory(parent)
}
