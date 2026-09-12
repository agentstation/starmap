package workspace

import (
	"context"
	"crypto/rand"
	stderrors "errors"
	"maps"
	"os"
	"path"
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
	published treeSnapshot
	enclosure treeSnapshot
	trees     map[string]*preparationTree
	journal   *preparationJournal
	handoff   *preparationHandoff
	record    *preparationRecord
}

func prepareWorkspaceStage(ctx context.Context, target string, writer *workspaceWriter) (result *workspaceStage, resultErr error) {
	s, err := prepareWorkspaceEnclosure(ctx, target, writer)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = stderrors.Join(resultErr, s.close(ctx))
		}
	}()
	if _, err := s.createTree(ctx, "render"); err != nil {
		return nil, err
	}
	// The empty candidate inherits the selected parent before entering private staging.
	if err := s.parent.Mkdir(s.candidate, directoryMode); err != nil {
		return nil, err
	}
	empty, err := snapshotTreeAt(ctx, s.parent, s.candidate)
	if err != nil {
		return nil, err
	}
	if len(empty.Entries) != 1 {
		return nil, replacementConflict(s.candidate, "new candidate is not empty")
	}
	if err := filepublish.DirectoryBetweenRootsNoReplace(s.parent, s.candidate, s.private, "tree"); err != nil {
		return nil, stderrors.Join(err, cleanupWorkspaceTreeAt(ctx, s.parent, s.candidate, empty))
	}
	if _, err := s.trackTree("tree"); err != nil {
		return nil, err
	}
	return s, nil
}

func prepareWorkspaceEnclosure(ctx context.Context, target string, writer *workspaceWriter) (result *workspaceStage, resultErr error) {
	if err := writer.check(); err != nil {
		return nil, err
	}
	if writer.target != target {
		return nil, writerConflict(target)
	}
	if err := policy.Require("workspace-preparing", policy.OwnerOnly); err != nil {
		return nil, err
	}
	parent, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return nil, err
	}
	suffix := rand.Text()
	s := &workspaceStage{parent: parent, name: "." + filepath.Base(target) + ".preparing-" + suffix, candidate: "." + filepath.Base(target) + ".candidate-" + suffix, trees: make(map[string]*preparationTree)}
	if err := privatefiles.CreateChild(parent, s.name); err != nil {
		_ = parent.Close()
		return nil, err
	}
	complete := false
	defer func() {
		if !complete {
			resultErr = stderrors.Join(resultErr, s.close(ctx))
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
	container, err := trackPreparationTree(s.private)
	if err != nil {
		return nil, err
	}
	s.enclosure, err = container.snapshot()
	if err != nil {
		return nil, err
	}
	s.journal, err = newPreparationJournal(s, writer)
	if err != nil {
		return nil, err
	}
	complete = true
	return s, nil
}

func (s *workspaceStage) renderPath() string { return filepath.Join(s.private.Name(), "render") }

func (s *workspaceStage) finish(ctx context.Context, target string, beforeRestore func(string) error) (string, error) {
	rendered, err := s.trees["render"].snapshot()
	if err != nil {
		return "", err
	}
	currentRender, err := snapshotTreeAt(ctx, s.private, "render")
	if err != nil {
		return "", err
	}
	if !sameTree(currentRender, rendered) || !maps.Equal(currentRender.identities, rendered.identities) {
		return "", replacementConflict(s.renderPath(), "rendered workspace changed before assembly")
	}
	render := s.trees["render"].root
	output := s.trees["tree"].root
	a := workspaceAssembler{ctx: ctx, source: s.source, render: render, output: output, owned: s.trees["tree"], entries: make(map[string]treeEntry, len(rendered.Entries)), children: make(map[string]int), beforeRestore: beforeRestore}
	for _, entry := range rendered.Entries {
		a.entries[entry.Path] = entry
		if entry.Path != "." {
			a.children[path.Dir(entry.Path)]++
		}
	}
	if err := a.directory("."); err != nil {
		return "", err
	}
	owned, err := s.trees["tree"].snapshot()
	if err != nil {
		return "", err
	}
	actual, err := snapshotTree(ctx, filepath.Join(s.private.Name(), "tree"))
	if err != nil {
		return "", err
	}
	if !sameTree(actual, owned) || !maps.Equal(actual.identities, owned.identities) {
		return "", replacementConflict(target, "assembled workspace ownership changed before publication")
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
	if err := output.Close(); err != nil {
		return "", err
	}
	s.trees["tree"].root = nil
	if err := s.recordHandoff(actual); err != nil {
		return "", err
	}
	if err := s.checkWriter(); err != nil {
		return "", err
	}
	if err := filepublish.DirectoryBetweenRootsNoReplace(s.private, "tree", s.parent, s.candidate); err != nil {
		return "", err
	}
	delete(s.trees, "tree")
	s.published = actual
	path := filepath.Join(s.parent.Name(), s.candidate)
	if err := filepublish.SyncDirectory(s.parent); err != nil {
		return "", err
	}
	return path, nil
}

type workspaceAssembler struct {
	ctx                    context.Context
	source, render, output *os.Root
	entries                map[string]treeEntry
	children               map[string]int
	owned                  *preparationTree
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

func (a workspaceAssembler) directory(name string) (resultErr error) {
	if err := a.ctx.Err(); err != nil {
		return err
	}
	if name != "." {
		if err := a.owned.directory(a.ctx, name); err != nil {
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
	if err := a.owned.rememberAccess(name, file); err != nil {
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
	if err := a.owned.rememberAccess(name, file); err != nil {
		return err
	}
	restored := false
	defer func() {
		if !restored {
			if err := a.owned.check(context.WithoutCancel(a.ctx), name); err != nil {
				resultErr = stderrors.Join(resultErr, err)
				return
			}
			err := restoreAccess()
			if err == nil {
				err = file.Chmod(mode)
			}
			if err == nil {
				err = a.owned.rememberAccess(name, file)
			}
			resultErr = stderrors.Join(resultErr, err)
		}
	}()
	if err := file.Chmod(mode | privatefiles.DirectoryMode); err != nil {
		return err
	}
	if err := a.owned.rememberAccess(name, file); err != nil {
		return err
	}
	dir, err := a.render.Open(filepath.FromSlash(name))
	if err != nil {
		return err
	}
	scanner := treeScanner{ctx: a.ctx, root: a.render, seen: replacementMaxEntries - a.children[name]}
	children, err := scanner.readChildren(dir, name)
	_ = dir.Close()
	if err != nil {
		return err
	}
	for _, child := range children {
		relative := path.Join(name, child)
		entry, present := a.entries[relative]
		if !present {
			return replacementConflict(relative, "unrecognized rendered entry")
		}
		if entry.Directory {
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
	if err := a.owned.rememberAccess(name, file); err != nil {
		return err
	}
	return file.Sync()
}

func (a workspaceAssembler) file(name string) error {
	if err := a.ctx.Err(); err != nil {
		return err
	}
	want, exists := a.entries[name]
	if !exists || want.Directory {
		return replacementConflict(name, "unrecognized rendered file")
	}
	info, err := a.render.Lstat(filepath.FromSlash(name))
	if err != nil {
		return err
	}
	input, err := openSnapshotEntry(a.render, filepath.FromSlash(name), info)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	return a.owned.writeFrom(a.ctx, name, snapshotReader{ctx: a.ctx, file: input}, want.Size, want.SHA256, func(output *os.File) error { return a.restore(name, output) })
}
