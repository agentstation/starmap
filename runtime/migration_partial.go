package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
)

const migrationPartialRecordLimit = 16 << 10

// migrationPartialRecord binds a mutable copy to its immutable source and owning stage.
// Interrupted bytes must remain a prefix of that source before recovery can remove them.
type migrationPartialRecord struct {
	Version int                    `json:"version"`
	RootID  string                 `json:"root_id"`
	WorkID  string                 `json:"work_id"`
	LockID  string                 `json:"lock_id"`
	FileID  string                 `json:"file_id"`
	Mode    uint32                 `json:"mode"`
	File    directoryMigrationFile `json:"file"`
}

func migrationEntryIdentity(root *os.Root, name string, expected os.FileInfo) (string, error) {
	before, err := root.Lstat(name)
	if err != nil {
		return "", err
	}
	if before.Mode()&os.ModeSymlink != 0 || (expected != nil && !os.SameFile(before, expected)) {
		return "", invalidMigrationIntent("entry_identity")
	}
	if err := validatePrivateMetadata(before, "migration.entry"); err != nil {
		return "", err
	}
	if err := validatePrivateACL(root, name, before, "migration.entry"); err != nil {
		return "", err
	}
	file, err := root.Open(name)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return "", stderrors.Join(invalidMigrationIntent("entry_identity"), err)
	}
	return filepublish.Identity(file)
}

func (s *directoryMigrationStage) partialBinding(work *os.Root) (migrationPartialRecord, error) {
	var record migrationPartialRecord
	var err error
	record.Version = 1
	record.RootID, err = migrationEntryIdentity(s.root, ".", nil)
	if err != nil {
		return record, err
	}
	info, err := s.root.Lstat(migrationWorkDirectory)
	if err != nil || !info.IsDir() {
		return record, stderrors.Join(invalidMigrationIntent("partial_directory"), err)
	}
	record.WorkID, err = migrationEntryIdentity(work, ".", info)
	if err != nil {
		return record, err
	}
	held, err := s.lock.Stat()
	if err != nil {
		return record, err
	}
	if _, err := privatefiles.ReadFile(s.root, directoryLockName, 0); err != nil {
		return record, err
	}
	record.LockID, err = migrationEntryIdentity(s.root, directoryLockName, held)
	return record, err
}

func (s *directoryMigrationStage) createPartial(ctx context.Context, expected directoryMigrationFile) (_ *os.File, resultErr error) {
	work, err := s.root.OpenRoot(migrationWorkDirectory)
	if err != nil {
		return nil, err
	}
	defer func() { _ = work.Close() }()
	record, err := s.partialBinding(work)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(migrationPartialName(expected.Target))
	output, err := createPrivateRuntimeFile(work, name)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = stderrors.Join(resultErr, output.Close())
		}
	}()
	info, err := output.Stat()
	if err != nil {
		return nil, err
	}
	record.FileID, err = filepublish.Identity(output)
	if err != nil {
		return nil, err
	}
	record.Mode = uint32(info.Mode())
	record.File = expected
	encoded, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	if len(encoded) > migrationPartialRecordLimit {
		return nil, invalidMigrationIntent("partial_record_size")
	}
	if err := output.Sync(); err != nil {
		return nil, err
	}
	if err := writeOwnerFile(ctx, work, name+".json", encoded); err != nil {
		return nil, err
	}
	if err := syncMigrationDirectory(work); err != nil {
		return nil, err
	}
	return output, nil
}

func (s *directoryMigrationStage) removePartial(ctx context.Context, source *os.Root, expected directoryMigrationFile, checkpoint migrationCheckpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	work, err := s.root.OpenRoot(migrationWorkDirectory)
	if err != nil {
		return err
	}
	defer func() { _ = work.Close() }()
	binding, err := s.partialBinding(work)
	if err != nil {
		return err
	}
	name := filepath.Base(migrationPartialName(expected.Target))
	encoded, err := privatefiles.ReadFile(work, name+".json", migrationPartialRecordLimit)
	if os.IsNotExist(err) {
		if _, err := work.Lstat(name); os.IsNotExist(err) {
			return nil
		}
		return invalidMigrationIntent("unrecorded_partial")
	}
	if err != nil {
		return err
	}
	recordInfo, err := work.Lstat(name + ".json")
	if err != nil {
		return err
	}
	var record migrationPartialRecord
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return invalidMigrationIntent("stage_record")
	}
	if record.Version != 1 || record.RootID != binding.RootID || record.WorkID != binding.WorkID ||
		record.LockID != binding.LockID || record.FileID == "" || record.File != expected {
		return invalidMigrationIntent("partial_owner")
	}
	info, err := work.Lstat(name)
	if err == nil {
		if !info.Mode().IsRegular() || uint32(info.Mode()) != record.Mode || info.Size() < 0 || info.Size() > expected.Size {
			return invalidMigrationIntent("partial_metadata")
		}
		id, err := migrationEntryIdentity(work, name, info)
		if err != nil || id != record.FileID {
			return stderrors.Join(invalidMigrationIntent("partial_identity"), err)
		}
		if err := verifyMigrationPartialPrefix(ctx, source, work, name, expected, info); err != nil {
			return err
		}
		currentBinding, err := s.partialBinding(work)
		if err != nil || currentBinding != binding {
			return stderrors.Join(invalidMigrationIntent("partial_owner"), err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		current, err := work.Lstat(name)
		if err != nil || !sameMigrationPartial(info, current) {
			return stderrors.Join(invalidMigrationIntent("partial_metadata"), err)
		}
		if err := verifyMigrationStageRecord(work, name+".json", encoded, recordInfo); err != nil {
			return err
		}
		if err := work.Remove(name); err != nil {
			return err
		}
		if err := syncMigrationDirectory(work); err != nil {
			return err
		}
		if err := migrationReached(checkpoint, "partial-removed", expected.Target); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := verifyMigrationStageRecord(work, name+".json", encoded, recordInfo); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := work.Remove(name + ".json"); err != nil {
		return err
	}
	return syncMigrationDirectory(work)
}

func verifyMigrationPartialPrefix(ctx context.Context, source, work *os.Root, name string, expected directoryMigrationFile, info os.FileInfo) error {
	input, err := openMigrationInput(source, filepath.FromSlash(expected.Source))
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	partial, err := openMigrationInput(work, name)
	if err != nil {
		return err
	}
	defer func() { _ = partial.Close() }()
	opened, err := partial.Stat()
	if err != nil || !sameMigrationPartial(info, opened) {
		return stderrors.Join(invalidMigrationIntent("partial_identity"), err)
	}
	var expectedBytes, actualBytes [32 << 10]byte
	for remaining := info.Size(); remaining > 0; {
		if err := ctx.Err(); err != nil {
			return err
		}
		size := min(remaining, int64(len(expectedBytes)))
		if _, err := io.ReadFull(input, expectedBytes[:size]); err != nil {
			return err
		}
		if _, err := io.ReadFull(partial, actualBytes[:size]); err != nil {
			return err
		}
		if !bytes.Equal(expectedBytes[:size], actualBytes[:size]) {
			return invalidMigrationIntent("partial_content")
		}
		remaining -= size
	}
	current, err := work.Lstat(name)
	if err != nil || !sameMigrationPartial(info, current) {
		return stderrors.Join(invalidMigrationIntent("partial_metadata"), err)
	}
	return nil
}

func sameMigrationPartial(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime() == b.ModTime()
}

func verifyMigrationStageRecord(work *os.Root, name string, encoded []byte, info os.FileInfo) error {
	actual, err := privatefiles.ReadFile(work, name, migrationPartialRecordLimit)
	if err != nil || !bytes.Equal(actual, encoded) {
		return stderrors.Join(invalidMigrationIntent("stage_record"), err)
	}
	current, err := work.Lstat(name)
	if err != nil || !sameMigrationPartial(current, info) {
		return stderrors.Join(invalidMigrationIntent("stage_record"), err)
	}
	return nil
}
