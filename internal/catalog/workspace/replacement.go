package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

type replacementPhase string

const (
	replacementJournalSaved  replacementPhase = "journal_saved"
	replacementBackupMoved   replacementPhase = "backup_moved"
	replacementInstalled     replacementPhase = "candidate_installed"
	replacementMarkerSaved   replacementPhase = "marker_saved"
	replacementEntryRemoved  replacementPhase = "backup_entry_removed"
	replacementBackupRemoved replacementPhase = "backup_removed"
)

type replacementHooks struct {
	recordWrites workspaceRecordWriter
	after        func(replacementPhase) error
	beforeMarker func() error
}

func (h replacementHooks) reached(phase replacementPhase) error {
	if h.after != nil {
		return h.after(phase)
	}
	return nil
}

func (p projector) replaceWithJournal(
	ctx context.Context, target, staged string, old treeSnapshot, marker projectionMarker,
) (owned, visible bool, resultErr error) {
	root, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return false, false, err
	}
	defer func() { _ = root.Close() }()
	current, err := snapshotTreeAt(ctx, root, filepath.Base(target))
	if err != nil {
		return false, false, err
	}
	if !sameTree(current, old) {
		return false, false, replacementConflict(target, "workspace files changed during staging")
	}
	candidate, err := snapshotTreeAt(ctx, root, filepath.Base(staged))
	if err != nil {
		return false, false, err
	}
	prefix := "." + filepath.Base(target) + ".candidate-"
	record := replacementRecord{
		Version: replacementVersion, Target: target, Candidate: filepath.Base(staged),
		Backup: "." + filepath.Base(target) + ".backup-" + strings.TrimPrefix(filepath.Base(staged), prefix),
		Old:    old, New: candidate, Marker: marker,
	}
	if _, err := root.Lstat(record.Backup); !os.IsNotExist(err) {
		if err != nil {
			return false, false, err
		}
		return false, false, replacementConflict(target, "backup destination already exists")
	}
	owned, err = writeReplacementRecord(ctx, root, record, p.recordWrites)
	if err != nil {
		return owned, false, err
	}
	hooks := replacementHooks{after: p.afterReplacementPhase, beforeMarker: p.beforeMarker, recordWrites: p.recordWrites}
	if err := hooks.reached(replacementJournalSaved); err != nil {
		return true, false, err
	}
	visible, err = advanceReplacement(ctx, root, record, hooks)
	return true, visible, err
}

func sameTree(a, b treeSnapshot) bool {
	return a.ID != "" && a.ID == b.ID && a.Digest == b.Digest
}

func optionalTree(ctx context.Context, root *os.Root, name string) (treeSnapshot, error) {
	tree, err := snapshotTreeAt(ctx, root, name)
	if os.IsNotExist(err) {
		// Only an absent root is optional. Missing descendants are conflicts.
		if _, rootErr := root.Lstat(name); os.IsNotExist(rootErr) {
			return treeSnapshot{}, nil
		}
	}
	return tree, err
}

func recoverReplacement(ctx context.Context, target string) (bool, error) {
	root, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return false, err
	}
	defer func() { _ = root.Close() }()
	record, err := readReplacementRecord(root, target)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = advanceReplacement(ctx, root, record, replacementHooks{})
	return true, err
}

func advanceReplacement(ctx context.Context, root *os.Root, record replacementRecord, hooks replacementHooks) (bool, error) {
	target := filepath.Base(record.Target)
	live, err := optionalTree(ctx, root, target)
	if err != nil {
		return false, err
	}
	candidate, err := optionalTree(ctx, root, record.Candidate)
	if err != nil {
		return false, err
	}
	backup, err := optionalTree(ctx, root, record.Backup)
	if err != nil {
		return false, err
	}
	if sameTree(candidate, record.New) {
		if err := validateReplacementCatalog(ctx, root, record.Candidate, record); err != nil {
			return false, err
		}
	}
	if sameTree(live, record.Old) && backup.ID == "" && candidate.ID == "" {
		return false, finishReplacementRecord(root, record)
	}
	if live.ID == "" && sameTree(backup, record.Old) && candidate.ID == "" {
		if err := moveReplacementDirectory(root, record.Backup, target); err != nil {
			return false, err
		}
		if err := filepublish.SyncDirectory(root); err != nil {
			return false, err
		}
		return false, finishReplacementRecord(root, record)
	}
	if sameTree(live, record.Old) && backup.ID == "" && sameTree(candidate, record.New) {
		backup, err = preserveReplacementBackup(ctx, root, record, hooks)
		if err != nil {
			return false, err
		}
		live = treeSnapshot{}
	}
	if live.ID == "" && sameTree(backup, record.Old) && sameTree(candidate, record.New) {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if err := moveReplacementDirectory(root, record.Candidate, target); err != nil {
			return false, err
		}
		if err := hooks.reached(replacementInstalled); err != nil {
			return true, err
		}
		if err := filepublish.SyncDirectory(root); err != nil {
			return true, err
		}
		live, err = snapshotTreeAt(ctx, root, target)
		if err != nil {
			return true, err
		}
		candidate = treeSnapshot{}
	}
	if !sameTree(live, record.New) || candidate.ID != "" {
		return false, replacementConflict(record.Target, "journal paths do not match the recorded directories and contents")
	}
	if err := finishInstalledReplacement(ctx, root, record, hooks); err != nil {
		return true, err
	}
	return true, nil
}

func preserveReplacementBackup(ctx context.Context, root *os.Root, record replacementRecord, hooks replacementHooks) (treeSnapshot, error) {
	if err := moveReplacementDirectory(root, filepath.Base(record.Target), record.Backup); err != nil {
		return treeSnapshot{}, err
	}
	if err := hooks.reached(replacementBackupMoved); err != nil {
		return treeSnapshot{}, err
	}
	if err := filepublish.SyncDirectory(root); err != nil {
		return treeSnapshot{}, err
	}
	return snapshotTreeAt(ctx, root, record.Backup)
}

func moveReplacementDirectory(root *os.Root, source, target string) error {
	if err := filepublish.DirectoryNoReplace(root, source, target); err != nil {
		if os.IsExist(err) {
			return replacementConflict(target, "replacement destination already exists")
		}
		return errors.WrapIO("rename", target, err)
	}
	return nil
}
