package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
)

const migrationInitializationName = "stage-initialization.json"

// migrationInitializationRecord binds unfinished initialization to one journal and native stage.
type migrationInitializationRecord struct {
	Version        int    `json:"version"`
	ParentID       string `json:"parent_id"`
	LockID         string `json:"lock_id"`
	RootID         string `json:"root_id"`
	Stage          string `json:"stage"`
	Target         string `json:"target"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

type migrationInitialization struct {
	record migrationInitializationRecord
	bytes  []byte
	info   os.FileInfo
}

func (m *directoryMigration) initializeStage(ctx context.Context, parent *os.Root, name string, encoded []byte, checkpoint migrationCheckpoint) error {
	binding, err := m.initializationBinding(parent, name, encoded)
	if err != nil {
		return err
	}
	initial, err := m.loadInitialization(binding)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	hasRecord := err == nil
	if _, err := parent.Lstat(name); err == nil {
		return m.finishExistingInitialization(ctx, parent, name, initial, hasRecord)
	} else if !os.IsNotExist(err) {
		return err
	}
	if !hasRecord {
		initial, err = m.createInitialization(ctx, parent, binding)
		if err != nil {
			return err
		}
	}
	stage := initial.record.Stage
	id, err := migrationEntryIdentity(parent, stage, nil)
	if err != nil || id != initial.record.RootID {
		return stderrors.Join(invalidMigrationIntent("initialization_identity"), err)
	}
	root, err := parent.OpenRoot(stage)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	id, err = migrationEntryIdentity(root, ".", nil)
	if err != nil || id != initial.record.RootID {
		return stderrors.Join(invalidMigrationIntent("initialization_identity"), err)
	}
	owner, err := m.prepareMigrationInitialization(ctx, root, encoded, checkpoint, filepath.Join(parent.Name(), stage))
	if err != nil {
		return err
	}
	if err := syncMigrationDirectory(root); err != nil {
		return err
	}
	if err := root.Close(); err != nil {
		return err
	}
	if err := migrationReached(checkpoint, "initialization-publish", filepath.Join(parent.Name(), stage)); err != nil {
		return err
	}
	if err := m.checkInitialization(parent, binding, initial); err != nil {
		return err
	}
	// Recheck content after the last checkpoint before publishing the bound stage.
	if err := migrationReached(checkpoint, "initialization-verify", filepath.Join(parent.Name(), stage)); err != nil {
		return err
	}
	checked, err := parent.OpenRoot(stage)
	if err != nil {
		return err
	}
	if err := verifyMigrationInitialization(ctx, checked, encoded, owner, true); err != nil {
		_ = checked.Close()
		return err
	}
	if err := checked.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.checkInitialization(parent, binding, initial); err != nil {
		return err
	}
	if err := publishMigrationDirectory(parent, stage, name); err != nil {
		return err
	}
	if err := syncMigrationDirectory(parent); err != nil {
		return err
	}
	if err := migrationReached(checkpoint, "initialization-published", filepath.Join(parent.Name(), name)); err != nil {
		return err
	}
	return m.finishInitialization(ctx, initial)
}

func (m *directoryMigration) initializationBinding(parent *os.Root, target string, encoded []byte) (migrationInitializationRecord, error) {
	binding := migrationInitializationRecord{Version: 1, Target: target}
	file, err := parent.Open(".")
	if err != nil {
		return binding, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return binding, err
	}
	selected, err := os.Lstat(filepath.Dir(m.manifest.TargetDirectory))
	if err != nil || !os.SameFile(opened, selected) {
		return binding, stderrors.Join(invalidMigrationIntent("initialization_parent"), err)
	}
	binding.ParentID, err = filepublish.Identity(file)
	if err != nil {
		return binding, err
	}
	held, err := m.journal.lock.Stat()
	if err != nil {
		return binding, err
	}
	binding.LockID, err = migrationEntryIdentity(m.journal.root, directoryLockName, held)
	digest := sha256.Sum256(encoded)
	binding.ManifestSHA256 = hex.EncodeToString(digest[:])
	return binding, err
}

func (m *directoryMigration) loadInitialization(binding migrationInitializationRecord) (migrationInitialization, error) {
	var initial migrationInitialization
	data, err := privatefiles.ReadFile(m.journal.root, migrationInitializationName, migrationPartialRecordLimit)
	if err != nil {
		return initial, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&initial.record); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return initial, invalidMigrationIntent("initialization_record")
	}
	r := initial.record
	token, ok := strings.CutPrefix(r.Stage, ".migration-build-")
	if !ok || len(token) != 26 || strings.ContainsFunc(token, func(r rune) bool { return (r < 'A' || r > 'Z') && (r < '2' || r > '7') }) ||
		r.RootID == "" || len(r.RootID) > migrationIdentityMaxBytes || r.Version != 1 || r.ParentID != binding.ParentID ||
		r.LockID != binding.LockID || r.Target != binding.Target || r.ManifestSHA256 != binding.ManifestSHA256 {
		return initial, invalidMigrationIntent("initialization_owner")
	}
	initial.bytes = data
	initial.info, err = m.journal.root.Lstat(migrationInitializationName)
	return initial, err
}

func (m *directoryMigration) createInitialization(ctx context.Context, parent *os.Root, binding migrationInitializationRecord) (migrationInitialization, error) {
	binding.Stage = ".migration-build-" + rand.Text()
	if err := createPrivateRuntimeChild(parent, binding.Stage); err != nil {
		return migrationInitialization{}, err
	}
	var err error
	binding.RootID, err = migrationEntryIdentity(parent, binding.Stage, nil)
	if err != nil {
		return migrationInitialization{}, err
	}
	encoded, err := json.Marshal(binding)
	if err != nil {
		return migrationInitialization{}, err
	}
	stage, err := parent.OpenRoot(binding.Stage)
	if err != nil {
		return migrationInitialization{}, err
	}
	if err := stderrors.Join(syncMigrationDirectory(stage), stage.Close()); err != nil {
		return migrationInitialization{}, err
	}
	if err := syncMigrationDirectory(parent); err != nil {
		return migrationInitialization{}, err
	}
	if err := writeOwnerFile(ctx, m.journal.root, migrationInitializationName, encoded); err != nil {
		return migrationInitialization{}, err
	}
	if err := syncMigrationDirectory(m.journal.root); err != nil {
		return migrationInitialization{}, err
	}
	return m.loadInitialization(binding)
}

func (m *directoryMigration) checkInitialization(parent *os.Root, binding migrationInitializationRecord, initial migrationInitialization) error {
	current, err := m.initializationBinding(parent, binding.Target, m.journal.manifest)
	if err != nil || current != binding {
		return stderrors.Join(invalidMigrationIntent("initialization_owner"), err)
	}
	id, err := migrationEntryIdentity(parent, initial.record.Stage, nil)
	if err != nil || id != initial.record.RootID {
		return stderrors.Join(invalidMigrationIntent("initialization_identity"), err)
	}
	return verifyMigrationStageRecord(m.journal.root, migrationInitializationName, initial.bytes, initial.info)
}

func (m *directoryMigration) finishInitialization(ctx context.Context, initial migrationInitialization) error {
	held, err := m.journal.lock.Stat()
	if err != nil {
		return err
	}
	lockID, err := migrationEntryIdentity(m.journal.root, directoryLockName, held)
	if err != nil || lockID != initial.record.LockID {
		return stderrors.Join(invalidMigrationIntent("initialization_owner"), err)
	}
	if err := verifyMigrationStageRecord(m.journal.root, migrationInitializationName, initial.bytes, initial.info); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.journal.root.Remove(migrationInitializationName); err != nil {
		return err
	}
	return syncMigrationDirectory(m.journal.root)
}

func verifyMigrationInitialization(ctx context.Context, root *os.Root, manifest, owner []byte, complete bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, readErr := directory.ReadDir(4)
	closeErr := directory.Close()
	if readErr != nil && !stderrors.Is(readErr, io.EOF) {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if len(entries) > 3 || (complete && len(entries) != 3) {
		return invalidMigrationIntent("initialization_entries")
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == migrationWorkDirectory {
			if !entry.IsDir() {
				return invalidMigrationIntent("initialization_directory")
			}
			if _, err := migrationEntryIdentity(root, name, nil); err != nil {
				return err
			}
			work, err := root.Open(name)
			if err != nil {
				return err
			}
			children, readErr := work.ReadDir(1)
			closeErr := work.Close()
			if readErr != nil && !stderrors.Is(readErr, io.EOF) {
				return readErr
			}
			if closeErr != nil {
				return closeErr
			}
			if len(children) != 0 {
				return invalidMigrationIntent("initialization_directory")
			}
			continue
		}
		var expected []byte
		switch name {
		case migrationPendingName:
			expected = manifest
		case ownerRecordName:
			expected = owner
		default:
			return invalidMigrationIntent("initialization_entry")
		}
		actual, err := privatefiles.ReadFile(root, name, migrationManifestMaxBytes)
		if err != nil || !bytes.Equal(actual, expected) {
			return stderrors.Join(invalidMigrationIntent("initialization_content"), err)
		}
	}
	return nil
}

func (m *directoryMigration) prepareMigrationInitialization(ctx context.Context, root *os.Root, encoded []byte, checkpoint migrationCheckpoint, stagePath string) ([]byte, error) {
	if err := migrationReached(checkpoint, "initialization-created", stagePath); err != nil {
		return nil, err
	}
	owner, err := encodeOwnerRecord(m.manifest.Owner, m.manifest.SourceIdentity)
	if err != nil {
		return nil, err
	}
	if err := verifyMigrationInitialization(ctx, root, encoded, owner, false); err != nil {
		return nil, err
	}
	for _, file := range []struct {
		name, phase string
		data        []byte
	}{
		{migrationPendingName, "initialization-intent", encoded},
		{ownerRecordName, "initialization-owner", owner},
	} {
		if _, err := root.Lstat(file.name); os.IsNotExist(err) {
			if err := writeOwnerFile(ctx, root, file.name, file.data); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
		if err := migrationReached(checkpoint, file.phase, stagePath); err != nil {
			return nil, err
		}
	}
	if err := createPrivateRuntimeChild(root, migrationWorkDirectory); err != nil && !os.IsExist(err) {
		return nil, err
	}
	if err := verifyMigrationInitialization(ctx, root, encoded, owner, true); err != nil {
		return nil, err
	}
	return owner, nil
}

func (m *directoryMigration) finishExistingInitialization(ctx context.Context, parent *os.Root, name string, initial migrationInitialization, hasRecord bool) error {
	if hasRecord {
		id, err := migrationEntryIdentity(parent, name, nil)
		if err != nil || id != initial.record.RootID {
			return stderrors.Join(invalidMigrationIntent("initialization_target"), err)
		}
	}
	if err := syncMigrationDirectory(parent); err != nil {
		return err
	}
	if hasRecord {
		return m.finishInitialization(ctx, initial)
	}
	return nil
}
