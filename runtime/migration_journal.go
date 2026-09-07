package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

type migrationPhase string

const (
	migrationPrepared  migrationPhase = "prepared"
	migrationCopied    migrationPhase = "copied"
	migrationVerified  migrationPhase = "verified"
	migrationPromoted  migrationPhase = "promoted"
	migrationCompleted migrationPhase = "completed"
)

func migrationPhases() []migrationPhase {
	return []migrationPhase{migrationPrepared, migrationCopied, migrationVerified, migrationPromoted, migrationCompleted}
}

type migrationEvent struct {
	Sequence       int            `json:"sequence"`
	Phase          migrationPhase `json:"phase"`
	PreviousSHA256 string         `json:"previous_sha256"`
}

type directoryMigrationJournal struct {
	mu        sync.Mutex
	root      *os.Root
	lock      *flock.Flock
	directory string
	phase     migrationPhase
	sequence  int
	previous  string
	manifest  []byte
	accepted  []byte
	closed    bool
	failed    bool
}

func openDirectoryMigrationJournal(ctx context.Context, root string, manifest directoryMigrationManifest) (_ *directoryMigrationJournal, resultErr error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	encoded, err := manifest.encode()
	if err != nil {
		return nil, err
	}
	if err := validateMigrationLocations(root, manifest.SourceDirectory, manifest.TargetDirectory); err != nil {
		return nil, err
	}
	operationHash := sha256.Sum256([]byte(manifest.OperationID))
	directory := filepath.Join(root, hex.EncodeToString(operationHash[:]))
	lock, err := acquireDirectory(ctx, directory)
	if err != nil {
		return nil, err
	}
	journal := &directoryMigrationJournal{lock: lock, directory: directory, manifest: encoded}
	defer func() {
		if resultErr != nil {
			_ = journal.Close()
		}
	}()
	journal.root, err = os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	if err := writeOwnerFile(ctx, journal.root, migrationManifestName, encoded); err != nil && !os.IsExist(err) {
		return nil, err
	}
	actual, err := readMigrationFile(journal.root, migrationManifestName, migrationManifestMaxBytes)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(actual, encoded) {
		return nil, migrationJournalConflict("operation ID is already bound to different intent")
	}
	manifestHash := sha256.Sum256(encoded)
	journal.previous = hex.EncodeToString(manifestHash[:])
	if err := journal.recover(ctx); err != nil {
		return nil, err
	}
	if journal.sequence == 0 {
		if err := journal.advance(ctx, migrationPrepared); err != nil {
			return nil, err
		}
	}
	if err := syncMigrationDirectory(journal.root); err != nil {
		return nil, err
	}
	return journal, nil
}

func migrationJournalConflict(message string) error {
	return &errors.ConflictError{Resource: "runtime migration journal", Message: message}
}

func readMigrationFile(root *os.Root, name string, limit int64) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, migrationJournalConflict("journal input is not a bounded regular file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, migrationJournalConflict("journal input changed during verification")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != info.Size() || int64(len(data)) > limit {
		return nil, migrationJournalConflict("journal input changed during verification")
	}
	return data, nil
}

func (j *directoryMigrationJournal) nextEvent(phase migrationPhase) ([]byte, error) {
	encoded, err := json.Marshal(migrationEvent{Sequence: j.sequence + 1, Phase: phase, PreviousSHA256: j.previous})
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func (j *directoryMigrationJournal) accept(line []byte, phase migrationPhase) {
	digest := sha256.Sum256(line)
	j.previous = hex.EncodeToString(digest[:])
	j.sequence++
	j.phase = phase
	j.accepted = append(j.accepted, line...)
}

// recover verifies the whole chain before it repairs an incomplete final write.
func (j *directoryMigrationJournal) recover(ctx context.Context) error {
	data, err := readMigrationFile(j.root, migrationJournalName, migrationJournalMaxBytes)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	phases := migrationPhases()
	offset := 0
	for offset < len(data) {
		if j.sequence >= len(phases) {
			return migrationJournalConflict("journal contains data after completion")
		}
		expected, err := j.nextEvent(phases[j.sequence])
		if err != nil {
			return err
		}
		end := bytes.IndexByte(data[offset:], '\n')
		if end < 0 {
			tail := data[offset:]
			if !bytes.HasPrefix(expected, tail) {
				return migrationJournalConflict("incomplete journal suffix does not match the next event")
			}
			return j.preservePartial(ctx, tail, int64(offset))
		}
		line := data[offset : offset+end+1]
		if !bytes.Equal(line, expected) {
			return migrationJournalConflict("journal sequence or integrity chain is invalid")
		}
		j.accept(line, phases[j.sequence])
		offset += end + 1
	}
	file, err := j.root.OpenFile(migrationJournalName, os.O_RDWR, ownerRecordMode)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return file.Sync()
}

func (j *directoryMigrationJournal) preservePartial(ctx context.Context, tail []byte, offset int64) error {
	digest := sha256.Sum256(tail)
	name := "journal.partial-" + hex.EncodeToString(digest[:])
	if err := writeOwnerFile(ctx, j.root, name, tail); err != nil && !os.IsExist(err) {
		return err
	}
	saved, err := readMigrationFile(j.root, name, migrationJournalMaxBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(saved, tail) {
		return migrationJournalConflict("partial-write archive differs from the journal suffix")
	}
	if err := syncMigrationDirectory(j.root); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := j.root.OpenFile(migrationJournalName, os.O_RDWR, ownerRecordMode)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if err := file.Truncate(offset); err != nil {
		return err
	}
	return file.Sync()
}

func (j *directoryMigrationJournal) advance(ctx context.Context, phase migrationPhase) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if ctx == nil {
		return &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if j.closed || j.failed {
		return migrationJournalConflict("reopen the journal before advancing")
	}
	index := slices.Index(migrationPhases(), phase)
	if phase != j.phase && index != j.sequence {
		return migrationJournalConflict("journal phases must advance in order")
	}
	actual, err := readMigrationFile(j.root, migrationManifestName, migrationManifestMaxBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, j.manifest) {
		return migrationJournalConflict("manifest changed while the operation was open")
	}
	file, err := j.openAppend()
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if phase == j.phase {
		return nil
	}
	line, err := j.nextEvent(phase)
	if err != nil {
		return err
	}
	if _, err := file.Write(line); err != nil {
		j.failed = true
		return err
	}
	if err := file.Sync(); err != nil {
		j.failed = true
		return err
	}
	if err := file.Close(); err != nil {
		j.failed = true
		return err
	}
	if err := syncMigrationDirectory(j.root); err != nil {
		j.failed = true
		return err
	}
	j.accept(line, phase)
	return nil
}

func (j *directoryMigrationJournal) openAppend() (_ *os.File, resultErr error) {
	info, err := j.root.Lstat(migrationJournalName)
	if os.IsNotExist(err) && j.sequence == 0 {
		return j.root.OpenFile(migrationJournalName, os.O_CREATE|os.O_EXCL|os.O_RDWR|os.O_APPEND, ownerRecordMode)
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() != int64(len(j.accepted)) {
		return nil, migrationJournalConflict("journal changed while the operation was open")
	}
	file, err := j.root.OpenFile(migrationJournalName, os.O_RDWR|os.O_APPEND, ownerRecordMode)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			_ = file.Close()
		}
	}()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, migrationJournalConflict("journal file changed before append")
	}
	actual, err := io.ReadAll(io.LimitReader(file, migrationJournalMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(actual, j.accepted) {
		return nil, migrationJournalConflict("journal changed while the operation was open")
	}
	return file, nil
}

func syncMigrationDirectory(root *os.Root) error {
	return filepublish.SyncDirectory(root)
}

func (j *directoryMigrationJournal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed = true
	var rootErr error
	if j.root != nil {
		rootErr = j.root.Close()
	}
	return stderrors.Join(rootErr, j.lock.Close())
}
