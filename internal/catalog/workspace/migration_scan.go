package workspace

import (
	"cmp"
	"context"
	stderrors "errors"
	"io"
	"io/fs"
	"os"
	"slices"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	legacyLayoutReadLimit     = 4 // Three permitted entries and one excess entry.
	legacyGenerationReadBatch = 128
)

func readLegacyLayoutEntries(path string) ([]fs.DirEntry, error) {
	directory, err := privatefiles.ExistingDirectory(path)
	if err != nil {
		return nil, err
	}
	root, err := directory.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	entries, err := file.ReadDir(legacyLayoutReadLimit)
	if err != nil && !stderrors.Is(err, io.EOF) {
		return nil, err
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })
	return entries, nil
}

func scanLegacyGenerations(ctx context.Context, path string, visit func(fs.DirEntry) error) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	directory, err := privatefiles.ExistingDirectory(path)
	if err != nil {
		return 0, err
	}
	root, err := directory.Open()
	if err != nil {
		return 0, err
	}
	defer func() { _ = root.Close() }()
	file, err := root.Open(".")
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()
	return walkLegacyGenerations(ctx, file, visit)
}

func walkLegacyGenerations(ctx context.Context, directory *os.File, visit func(fs.DirEntry) error) (int, error) {
	count := 0
	for {
		if err := ctx.Err(); err != nil {
			return count, err
		}
		entries, err := directory.ReadDir(legacyGenerationReadBatch)
		if err != nil && !stderrors.Is(err, io.EOF) {
			return count, errors.WrapIO("read", directory.Name(), err)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return count, err
			}
			if err := visit(entry); err != nil {
				return count, err
			}
			count++
		}
		if stderrors.Is(err, io.EOF) {
			return count, ctx.Err()
		}
	}
}
