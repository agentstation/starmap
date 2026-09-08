package workspace

import (
	"context"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/filepublish"
)

func cleanupReplacementBackup(ctx context.Context, parent *os.Root, record replacementRecord, hooks replacementHooks) error {
	backup, err := optionalTree(ctx, parent, record.Backup)
	if err != nil {
		return err
	}
	if backup.ID == "" {
		return nil
	}
	if backup.ID != record.Old.ID {
		return replacementConflict(record.Target, "backup directory identity changed")
	}
	expected := make(map[string]treeEntry, len(record.Old.Entries))
	for _, entry := range record.Old.Entries {
		expected[entry.Path] = entry
	}
	for _, entry := range backup.Entries {
		if expected[entry.Path] != entry {
			return replacementConflict(entry.Path, "backup contains changed or unrecognized files")
		}
	}
	root, err := parent.OpenRoot(record.Backup)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	scanner := treeScanner{ctx: ctx, root: root}
	for i := len(backup.Entries) - 1; i > 0; i-- {
		entry := backup.Entries[i]
		current, err := scanner.entry(entry.Path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if current != entry {
			return replacementConflict(entry.Path, "backup changed during cleanup")
		}
		if err := root.Remove(filepath.FromSlash(entry.Path)); err != nil {
			return err
		}
		if err := hooks.reached(replacementEntryRemoved); err != nil {
			return err
		}
	}
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	id, identityErr := entryIdentity(file)
	_ = file.Close()
	if identityErr != nil {
		return identityErr
	}
	if id != record.Old.ID {
		return replacementConflict(record.Backup, "backup root changed during cleanup")
	}
	if err := root.Close(); err != nil {
		return err
	}
	current, err := snapshotTreeAt(ctx, parent, record.Backup)
	if err != nil {
		return err
	}
	if current.ID != record.Old.ID || len(current.Entries) != 1 || current.Entries[0] != record.Old.Entries[0] {
		return replacementConflict(record.Backup, "backup is not the verified empty directory")
	}
	if err := parent.Remove(record.Backup); err != nil {
		return err
	}
	return filepublish.SyncDirectory(parent)
}
