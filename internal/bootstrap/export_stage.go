package bootstrap

import (
	"bytes"
	stderrors "errors"
	"io"
	"os"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

// baselineStage retains creation identities until publication or local cleanup.
// It never adopts staging files from another export or an earlier process.
type baselineStage struct {
	parent    *os.Root
	root      *os.Root
	name      string
	identity  os.FileInfo
	files     []*baselineStageFile
	published bool
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
	file, err := s.root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, baselineFileMode)
	if err != nil {
		return err
	}
	defer func() { result = stderrors.Join(result, file.Close()) }()
	record := &baselineStageFile{name: name}
	s.files = append(s.files, record)
	written, writeErr := file.Write(contents)
	record.contents = contents[:written]
	record.info, err = file.Stat()
	if err := stderrors.Join(writeErr, err); err != nil {
		return err
	}
	return file.Sync()
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

func (s *baselineStage) cleanup() error {
	if s.published {
		return nil
	}
	if err := s.validate(); err != nil {
		return err
	}
	for _, record := range s.files {
		if err := record.validate(s.root); err != nil {
			return err
		}
		if err := s.root.Remove(record.name); err != nil {
			return err
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
	return filepublish.SyncDirectory(s.parent)
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
