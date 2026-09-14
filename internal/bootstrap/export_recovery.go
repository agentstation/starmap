package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	baselineRecoveryEntryLimit = 4096
	baselineRecoveryByteLimit  = 64 << 20
)

// BaselineRecovery reports completed recovery operations and paths that recovery preserves.
// Preserved paths are relative to the baseline directory. They require separate ownership review.
type BaselineRecovery struct {
	RecoveredOperations int      `json:"recovered_operations"`
	PreservedPaths      []string `json:"preserved_paths,omitempty"`
}

func (r *baselineRecovery) recover(ctx context.Context, checkpoint func(string) error) (BaselineRecovery, error) {
	var report BaselineRecovery
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if err := r.ownsLock(); err != nil {
		return report, err
	}
	parents, err := baselineRecoveryEntries(r.parent, baselineRecoveryEntryLimit)
	if err != nil {
		return report, err
	}
	records, err := baselineRecoveryEntries(r.root, baselineRecoveryEntryLimit-len(parents))
	if err != nil {
		return report, err
	}
	slices.SortFunc(records, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	remaining := int64(baselineRecoveryByteLimit)
	for _, entry := range records {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if entry.Name() == baselineRecoveryLock {
			continue
		}
		journal, err := r.readJournal(entry.Name())
		if err == nil {
			err = r.recoverJournal(ctx, journal, &remaining, checkpoint)
		}
		if err != nil {
			if ctx.Err() != nil {
				return report, ctx.Err()
			}
			report.PreservedPaths = append(report.PreservedPaths, filepath.Join(baselineRecoveryDirectory, entry.Name()))
			continue
		}
		report.RecoveredOperations++
	}
	for _, entry := range parents {
		if !strings.HasPrefix(entry.Name(), baselineStagePrefix) {
			continue
		}
		if _, err := r.parent.Lstat(entry.Name()); !os.IsNotExist(err) {
			report.PreservedPaths = append(report.PreservedPaths, entry.Name())
		}
	}
	slices.Sort(report.PreservedPaths)
	return report, nil
}

func baselineRecoveryEntries(root *os.Root, limit int) ([]fs.DirEntry, error) {
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	entries, err := file.ReadDir(limit + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > limit {
		return nil, &errors.ValidationError{Field: "baseline.recovery.entry_limit", Value: baselineRecoveryEntryLimit,
			Message: "baseline and recovery directories exceed the bounded scan limit"}
	}
	return entries, nil
}

func (r *baselineRecovery) recoverJournal(ctx context.Context, journal *baselineJournal, remaining *int64, checkpoint func(string) error) error {
	record := journal.record
	if record.LockIdentity != r.lockIdentity {
		return stageConflict(baselineRecoveryLock)
	}
	// A successful rename moves the original identity to the immutable target.
	// A reused stage name can belong to another writer or operator.
	if info, err := r.parent.Lstat(record.Target); err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		identity, err := baselineEntryIdentity(r.parent, record.Target, info)
		if err != nil {
			return err
		}
		if identity == record.Identity {
			if len(record.Files) != 2 {
				return stageConflict(record.Target)
			}
			stage, err := r.restoreRecordedStage(record.Target, record, remaining)
			if err != nil {
				return err
			}
			if err := stderrors.Join(syncBaselineDirectory(stage.root), syncBaselineDirectory(r.parent), stage.close()); err != nil {
				return err
			}
			return journal.remove(ctx)
		}
	}
	if _, err := r.parent.Lstat(record.Stage); os.IsNotExist(err) && record.Phase == baselineJournalCollect {
		return journal.remove(ctx)
	}
	stage, err := r.restoreRecordedStage(record.Stage, record, remaining)
	if err != nil {
		return err
	}
	defer func() { _ = stage.close() }()
	stage.journal = journal
	return stage.cleanup(ctx, checkpoint)
}

func (r *baselineRecovery) restoreRecordedStage(name string, record baselineJournalRecord, remaining *int64) (_ *baselineStage, resultErr error) {
	stage, err := openBaselineStage(r.parent, name)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = stderrors.Join(resultErr, stage.close())
		}
	}()
	identity, err := baselineEntryIdentity(stage.root, ".", stage.identity)
	if err != nil {
		return nil, err
	}
	if identity != record.Identity || uint32(stage.identity.Mode()) != record.Mode {
		return nil, stageConflict(name)
	}
	if err := privatefiles.ValidateMetadata(stage.identity, "baseline.stage"); err != nil {
		return nil, err
	}
	if err := privatefiles.ValidateACL(stage.root, ".", stage.identity, "baseline.stage"); err != nil {
		return nil, err
	}
	for _, expected := range record.Files {
		info, err := stage.root.Lstat(expected.Name)
		if os.IsNotExist(err) && record.Phase == baselineJournalCollect && name == record.Stage {
			continue
		}
		if err != nil {
			return nil, err
		}
		if uint32(info.Mode()) != expected.Mode || info.Size() != expected.Size || !info.ModTime().Equal(expected.Modified) || !info.Mode().IsRegular() {
			return nil, stageConflict(expected.Name)
		}
		identity, err := baselineEntryIdentity(stage.root, expected.Name, info)
		if err != nil {
			return nil, err
		}
		if identity != expected.Identity {
			return nil, stageConflict(expected.Name)
		}
		if expected.Size > *remaining {
			return nil, &errors.ValidationError{Field: "baseline.recovery.byte_limit", Message: "stage exceeds the remaining content verification budget"}
		}
		*remaining -= expected.Size
		data, err := privatefiles.ReadFile(stage.root, expected.Name, expected.Size)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != expected.SHA256 {
			return nil, stageConflict(expected.Name)
		}
		stage.files = append(stage.files, &baselineStageFile{name: expected.Name, info: info, contents: data})
	}
	if err := stage.validate(); err != nil {
		return nil, err
	}
	return stage, nil
}
