package workspace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

type workspaceStage struct {
	parent    *os.Root
	private   *os.Root
	name      string
	candidate string
	source    *os.Root
	original  treeSnapshot
}

func prepareWorkspaceStage(target string) (*workspaceStage, error) {
	if err := policy.Require("workspace-preparing", policy.OwnerOnly); err != nil {
		return nil, err
	}
	parent, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return nil, err
	}
	suffix := rand.Text()
	s := &workspaceStage{parent: parent, name: "." + filepath.Base(target) + ".preparing-" + suffix, candidate: "." + filepath.Base(target) + ".candidate-" + suffix}
	if err := privatefiles.CreateChild(parent, s.name); err != nil {
		_ = parent.Close()
		return nil, err
	}
	complete := false
	defer func() {
		if !complete {
			s.close()
		}
	}()
	s.private, err = parent.OpenRoot(s.name)
	if err != nil {
		return nil, err
	}
	file, err := openStagedDirectory(s.private, ".")
	if err != nil {
		return nil, err
	}
	if err := privateStageAccess(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	info, err := file.Stat()
	_ = file.Close()
	if err != nil {
		return nil, err
	}
	if err := privatefiles.ValidateMetadata(info, "workspace staging"); err != nil {
		return nil, err
	}
	if err := privatefiles.ValidateACL(s.private, ".", info, "workspace staging"); err != nil {
		return nil, err
	}
	if err := s.private.Mkdir("render", directoryMode); err != nil {
		return nil, err
	}
	// The empty candidate inherits the selected parent before entering private staging.
	if err := parent.Mkdir(s.candidate, directoryMode); err != nil {
		return nil, err
	}
	if err := filepublish.DirectoryBetweenRootsNoReplace(parent, s.candidate, s.private, "tree"); err != nil {
		_ = parent.Remove(s.candidate)
		return nil, err
	}
	complete = true
	return s, nil
}

func (s *workspaceStage) close() {
	if s.source != nil {
		_ = s.source.Close()
	}
	if s.private != nil {
		_ = s.private.Close()
	}
	_ = os.RemoveAll(filepath.Join(s.parent.Name(), s.name))
	_ = s.parent.Close()
}

func (s *workspaceStage) renderPath() string { return filepath.Join(s.private.Name(), "render") }

func (s *workspaceStage) finish(ctx context.Context, target string, beforeRestore func(string) error) (string, error) {
	rendered, err := snapshotTree(ctx, s.renderPath())
	if err != nil {
		return "", err
	}
	render, err := s.private.OpenRoot("render")
	if err != nil {
		return "", err
	}
	defer func() { _ = render.Close() }()
	output, err := s.private.OpenRoot("tree")
	if err != nil {
		return "", err
	}
	defer func() { _ = output.Close() }()
	a := workspaceAssembler{ctx: ctx, source: s.source, render: render, output: output, entries: make(map[string]treeEntry, len(rendered.Entries)), beforeRestore: beforeRestore}
	for _, entry := range rendered.Entries {
		a.entries[entry.Path] = entry
	}
	if err := a.directory("."); err != nil {
		return "", err
	}
	actual, err := snapshotTree(ctx, filepath.Join(s.private.Name(), "tree"))
	if err != nil {
		return "", err
	}
	if len(actual.Entries) != len(rendered.Entries) {
		return "", replacementConflict(target, "staged workspace entries changed")
	}
	for _, entry := range actual.Entries {
		want := a.entries[entry.Path]
		if entry.Path != want.Path || entry.Directory != want.Directory || entry.Size != want.Size || entry.SHA256 != want.SHA256 {
			return "", replacementConflict(entry.Path, "staged workspace content changed")
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	current, err := optionalTree(ctx, s.parent, filepath.Base(target))
	if err != nil {
		return "", err
	}
	if (current.ID != "" || s.original.ID != "") && !sameTree(current, s.original) {
		return "", replacementConflict(target, "workspace changed before candidate publication")
	}
	if err := filepublish.DirectoryBetweenRootsNoReplace(s.private, "tree", s.parent, s.candidate); err != nil {
		return "", err
	}
	path := filepath.Join(s.parent.Name(), s.candidate)
	if err := filepublish.SyncDirectory(s.parent); err != nil {
		_ = os.RemoveAll(path)
		return "", err
	}
	return path, nil
}

type workspaceAssembler struct {
	ctx                    context.Context
	source, render, output *os.Root
	entries                map[string]treeEntry
	beforeRestore          func(string) error
}

func (a workspaceAssembler) restore(name string, destination *os.File) error {
	if a.beforeRestore != nil {
		if err := a.beforeRestore(name); err != nil {
			return err
		}
	}
	if a.source == nil {
		return nil
	}
	info, err := a.source.Lstat(filepath.FromSlash(name))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	want := a.entries[name]
	if info.IsDir() != want.Directory || (!info.IsDir() && !info.Mode().IsRegular()) {
		return replacementConflict(name, "source entry type changed")
	}
	file, err := openSnapshotEntry(a.source, filepath.FromSlash(name), info)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) {
		return replacementConflict(name, "source entry changed before access copy")
	}
	before, err := entryAccessDigest(file)
	if err != nil {
		return err
	}
	if err := copyNativeAccess(file, destination); err != nil {
		return errors.WrapResource("restore", "workspace access", name, err)
	}
	actual, err := entryAccessDigest(destination)
	if err != nil {
		return err
	}
	scanner := treeScanner{ctx: a.ctx, root: a.source}
	if err := scanner.checkEntryAccess(name, file, info, before); err != nil {
		return err
	}
	if actual != before {
		return replacementConflict(name, "native access could not be preserved")
	}
	return nil
}

func (a workspaceAssembler) directory(name string) error {
	if err := a.ctx.Err(); err != nil {
		return err
	}
	if name != "." {
		if err := a.output.Mkdir(filepath.FromSlash(name), directoryMode); err != nil {
			return err
		}
	}
	file, err := openStagedDirectory(a.output, filepath.FromSlash(name))
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if err := a.restore(name, file); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	mode := info.Mode() & workspaceAccessMode
	access, err := entryAccessDigest(file)
	if err != nil {
		return err
	}
	restoreAccess, err := writableStagedDirectory(file)
	if err != nil {
		return err
	}
	restored := false
	defer func() {
		if !restored {
			_ = restoreAccess()
		}
	}()
	if err := file.Chmod(mode | privatefiles.DirectoryMode); err != nil {
		return err
	}
	dir, err := a.render.Open(filepath.FromSlash(name))
	if err != nil {
		return err
	}
	children, err := dir.ReadDir(-1)
	_ = dir.Close()
	if err != nil {
		return err
	}
	for _, child := range children {
		relative := filepath.ToSlash(filepath.Join(name, child.Name()))
		if child.IsDir() {
			err = a.directory(relative)
		} else {
			err = a.file(relative)
		}
		if err != nil {
			return err
		}
	}
	if err := restoreAccess(); err != nil {
		return err
	}
	restored = true
	if err := file.Chmod(mode); err != nil {
		return err
	}
	actual, err := entryAccessDigest(file)
	if err != nil {
		return err
	}
	if actual != access {
		return replacementConflict(name, "directory access changed during assembly")
	}
	return file.Sync()
}

func (a workspaceAssembler) file(name string) error {
	if err := a.ctx.Err(); err != nil {
		return err
	}
	want := a.entries[name]
	input, err := a.render.Open(filepath.FromSlash(name))
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	output, err := createStagedFile(a.output, filepath.FromSlash(name))
	if err != nil {
		return err
	}
	defer func() { _ = output.Close() }()
	hash := sha256.New()
	written, err := io.CopyN(io.MultiWriter(output, hash), snapshotReader{ctx: a.ctx, file: input}, want.Size)
	if err != nil {
		return err
	}
	if written != want.Size || hex.EncodeToString(hash.Sum(nil)) != want.SHA256 {
		return replacementConflict(name, "rendered content changed during copy")
	}
	if err := a.restore(name, output); err != nil {
		return err
	}
	return output.Sync()
}
