package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// preserveOperatorFiles retains non-record files that the catalog writer removes with model directories.
func preserveOperatorFiles(ctx context.Context, render, backup string) error {
	tree, err := snapshotTree(ctx, render)
	if err != nil {
		return err
	}
	if err := os.Mkdir(backup, directoryMode); err != nil {
		return err
	}
	input, err := os.OpenRoot(render)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	output, err := os.OpenRoot(backup)
	if err != nil {
		return err
	}
	defer func() { _ = output.Close() }()
	for _, entry := range tree.Entries {
		if !strings.Contains(entry.Path, "/models/") || (!entry.Directory && strings.HasSuffix(entry.Path, ".yaml")) {
			continue
		}
		name := filepath.FromSlash(entry.Path)
		if entry.Directory {
			if err := os.MkdirAll(filepath.Join(backup, name), directoryMode); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(backup, name)), directoryMode); err != nil {
			return err
		}
		if err := copyRecordedSourceFile(ctx, input, output, entry); err != nil {
			return err
		}
	}
	return nil
}

func restoreOperatorFiles(backup, render string) error {
	return os.CopyFS(render, os.DirFS(backup))
}
