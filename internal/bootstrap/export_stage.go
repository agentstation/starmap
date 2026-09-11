package bootstrap

import (
	"bytes"
	"context"
	stderrors "errors"
	"io"
	"os"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
)

// baselineStage retains creation identities until publication or cleanup.
// Recovery restores earlier identities only after it verifies the private journal and file contents.
type baselineStage struct {
	parent    *os.Root
	root      *os.Root
	name      string
	identity  os.FileInfo
	files     []*baselineStageFile
	published bool
	journal   *baselineJournal
}

type baselineStageFile struct {
	name     string
	info     os.FileInfo
	contents []byte
}

func openBaselineStage(parent *os.Root, name string) (*baselineStage, error) {
	identity, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	stage := &baselineStage{parent: parent, root: root, name: name, identity: identity}
	if err := stage.validateLocation(); err != nil {
		_ = root.Close()
		return nil, err
	}
	return stage, nil
}

func (s *baselineStage) write(name string, contents []byte) (result error) {
	file, err := privatefiles.CreateFile(s.root, name)
	if err != nil {
		return err
	}
	record := &baselineStageFile{name: name}
	defer func() {
		closeErr := file.Close()
		result = stderrors.Join(result, closeErr)
		if closeErr == nil && record.info != nil {
			result = stderrors.Join(result, record.seal(s.root))
		}
	}()
	s.files = append(s.files, record)
	written, writeErr := file.Write(contents)
	record.contents = contents[:written]
	record.info, err = file.Stat()
	if err := stderrors.Join(writeErr, err); err != nil {
		return err
	}
	return file.Sync()
}

// seal records final timestamps after the writing handle closes.
func (r *baselineStageFile) seal(root *os.Root) error {
	current, err := root.Lstat(r.name)
	if err != nil {
		return stderrors.Join(stageConflict(r.name), err)
	}
	if !current.Mode().IsRegular() || !os.SameFile(r.info, current) || r.info.Mode() != current.Mode() || r.info.Size() != current.Size() {
		return stageConflict(r.name)
	}
	r.info = current
	return nil
}

func (s *baselineStage) validateLocation() error {
	current, err := s.parent.Lstat(s.name)
	if err != nil {
		return stderrors.Join(stageConflict(s.name), err)
	}
	bound, err := s.root.Stat(".")
	if err != nil {
		return err
	}
	if !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(s.identity, current) || !os.SameFile(s.identity, bound) {
		return stageConflict(s.name)
	}
	return nil
}

func (s *baselineStage) validate() error {
	if err := s.validateLocation(); err != nil {
		return err
	}
	directory, err := s.root.Open(".")
	if err != nil {
		return err
	}
	entries, readErr := directory.ReadDir(len(s.files) + 1)
	closeErr := directory.Close()
	if readErr != nil && !stderrors.Is(readErr, io.EOF) {
		return stderrors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if len(entries) != len(s.files) {
		return stageConflict(s.name)
	}
	for _, record := range s.files {
		if err := record.validate(s.root); err != nil {
			return err
		}
	}
	return nil
}

func (r *baselineStageFile) validate(root *os.Root) (result error) {
	current, err := root.Lstat(r.name)
	if err != nil {
		return stderrors.Join(stageConflict(r.name), err)
	}
	if r.info == nil || !current.Mode().IsRegular() || !os.SameFile(r.info, current) || r.info.Mode() != current.Mode() || r.info.Size() != current.Size() || !r.info.ModTime().Equal(current.ModTime()) {
		return stageConflict(r.name)
	}
	file, err := root.Open(r.name)
	if err != nil {
		return stderrors.Join(stageConflict(r.name), err)
	}
	defer func() { result = stderrors.Join(result, file.Close()) }()
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(r.info, opened) || r.info.Mode() != opened.Mode() || r.info.Size() != opened.Size() || !r.info.ModTime().Equal(opened.ModTime()) {
		return stageConflict(r.name)
	}
	contents, err := io.ReadAll(io.NewSectionReader(file, 0, int64(len(r.contents))+1))
	if err != nil {
		return err
	}
	if !bytes.Equal(contents, r.contents) {
		return stageConflict(r.name)
	}
	return nil
}

func (s *baselineStage) cleanup(ctx context.Context, checkpoint func(string) error) error {
	if s.published {
		if s.journal != nil && s.journal.raw != nil {
			return s.journal.remove(ctx)
		}
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.validate(); err != nil {
		return err
	}
	if s.journal != nil && s.journal.raw != nil {
		if err := s.journal.save(ctx, s, baselineJournalCollect); err != nil {
			return err
		}
	}
	if checkpoint != nil {
		if err := checkpoint("recovery-collecting"); err != nil {
			return err
		}
	}
	for _, record := range s.files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := record.validate(s.root); err != nil {
			return err
		}
		if err := s.root.Remove(record.name); err != nil {
			return err
		}
		if checkpoint != nil {
			if err := checkpoint("recovery-file-removed"); err != nil {
				return err
			}
		}
	}
	if err := s.validateLocation(); err != nil {
		return err
	}
	// Remove refuses a nonempty directory if another entry appeared during cleanup.
	if err := s.close(); err != nil {
		return err
	}
	current, err := s.parent.Lstat(s.name)
	if err != nil || !os.SameFile(s.identity, current) {
		return stderrors.Join(stageConflict(s.name), err)
	}
	if err := s.parent.Remove(s.name); err != nil {
		return err
	}
	if err := filepublish.SyncDirectory(s.parent); err != nil {
		return err
	}
	if checkpoint != nil {
		if err := checkpoint("recovery-stage-removed"); err != nil {
			return err
		}
	}
	if s.journal != nil && s.journal.raw != nil {
		return s.journal.remove(ctx)
	}
	return nil
}

func (s *baselineStage) close() error {
	var result error
	if s.root != nil {
		result = stderrors.Join(result, s.root.Close())
		s.root = nil
	}
	return result
}

func stageConflict(name string) error {
	return &errors.ConflictError{Resource: "embedded baseline staging " + name, Message: "directory or files changed after creation"}
}
