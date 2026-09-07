package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func migrationPartialName(target string) string {
	digest := sha256.Sum256([]byte(target))
	return migrationWorkDirectory + "/" + hex.EncodeToString(digest[:]) + ".partial"
}

func copyMigrationFile(ctx context.Context, source, stage *os.Root, expected directoryMigrationFile, checkpoint migrationCheckpoint) error {
	partial := filepath.FromSlash(migrationPartialName(expected.Target))
	if present, err := migrationFilePresent(ctx, stage, expected); err != nil {
		return err
	} else if present {
		if err := syncMigrationParent(stage, path.Dir(expected.Target)); err != nil {
			return err
		}
		return removeMigrationPartial(stage, partial)
	}
	if err := makeMigrationDirectories(stage, path.Dir(expected.Target)); err != nil {
		return err
	}
	input, err := openMigrationInput(source, filepath.FromSlash(expected.Source))
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	if err := removeMigrationPartial(stage, partial); err != nil {
		return err
	}
	output, err := stage.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, ownerRecordMode)
	if err != nil {
		return err
	}
	defer func() { _ = output.Close() }()
	digest := sha256.New()
	writer := migrationCopyWriter{writer: io.MultiWriter(output, digest), checkpoint: checkpoint, target: expected.Target}
	n, err := io.Copy(writer, migrationContextReader{ctx: ctx, reader: io.LimitReader(input, expected.Size)})
	if err != nil {
		return err
	}
	var extra [1]byte
	count, readErr := input.Read(extra[:])
	if n != expected.Size || count != 0 || !stderrors.Is(readErr, io.EOF) || hex.EncodeToString(digest.Sum(nil)) != expected.SHA256 {
		return invalidMigrationIntent("changed_source")
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.Link(partial, filepath.FromSlash(expected.Target)); err != nil {
		return err
	}
	if err := syncMigrationParent(stage, path.Dir(expected.Target)); err != nil {
		return err
	}
	if err := migrationReached(checkpoint, "file-published", expected.Target); err != nil {
		return err
	}
	return removeMigrationPartial(stage, partial)
}

func openMigrationInput(root *os.Root, name string) (_ *os.File, resultErr error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, invalidMigrationIntent("files.source")
	}
	file, err := root.Open(name)
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
		return nil, invalidMigrationIntent("changed_source")
	}
	return file, nil
}

func removeMigrationPartial(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return invalidMigrationIntent("partial_file")
	}
	if err := root.Remove(name); err != nil {
		return err
	}
	return syncMigrationParent(root, migrationWorkDirectory)
}

func makeMigrationDirectories(root *os.Root, directory string) error {
	if directory == "." {
		return nil
	}
	current := ""
	for _, component := range strings.Split(directory, "/") {
		current = path.Join(current, component)
		if err := root.Mkdir(filepath.FromSlash(current), runtimeDirectoryMode); err != nil && !os.IsExist(err) {
			return err
		}
		info, err := root.Lstat(filepath.FromSlash(current))
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return invalidMigrationIntent("stage_directory")
		}
		if err := syncMigrationParent(root, path.Dir(current)); err != nil {
			return err
		}
	}
	return nil
}

func syncMigrationParent(root *os.Root, directory string) error {
	parent, err := root.OpenRoot(filepath.FromSlash(directory))
	if err != nil {
		return err
	}
	defer func() { _ = parent.Close() }()
	return syncMigrationDirectory(parent)
}

type migrationCopyWriter struct {
	writer     io.Writer
	checkpoint migrationCheckpoint
	target     string
}

func (w migrationCopyWriter) Write(data []byte) (int, error) {
	n, err := w.writer.Write(data)
	if err == nil {
		err = migrationReached(w.checkpoint, "copy-chunk", w.target)
	}
	return n, err
}
