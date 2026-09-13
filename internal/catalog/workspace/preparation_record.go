package workspace

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/filepublish"
)

type preparationRecord struct {
	Destination string    `json:"destination"`
	Entry       treeEntry `json:"entry"`
	Identity    string    `json:"identity"`
}

func (r preparationRecord) state() workspaceRecordState {
	return workspaceRecordState{entry: r.Entry, identity: r.Identity}
}

func (r preparationRecord) validate(stage *workspaceStage, target string) error {
	if r.Destination != filepath.Base(projectionMarkerPath(target)) && r.Destination != filepath.Base(replacementJournalPath(target)) {
		return invalidReplacement("preparation_record_destination")
	}
	suffix, found := strings.CutPrefix(stage.name, "."+filepath.Base(target)+".preparing-")
	if !found || suffix == "" || r.Entry.Path != "."+r.Destination+"."+suffix || !replacementChildName(r.Entry.Path) {
		return invalidReplacement("preparation_record_path")
	}
	if r.Identity == "" || len(r.Identity) > replacementIdentityMax || r.Entry.Directory || r.Entry.Size < 0 || r.Entry.Size > replacementJournalMax ||
		r.Entry.Mode&^uint32(workspaceAccessMode) != 0 || !replacementDigest(r.Entry.SHA256) || !replacementDigest(r.Entry.AccessSHA256) {
		return invalidReplacement("preparation_record_entry")
	}
	if previous := stage.record; previous != nil && (previous.Destination != r.Destination || previous.Entry.Path != r.Entry.Path || previous.Identity != r.Identity) {
		return invalidReplacement("preparation_record_identity")
	}
	return nil
}

func (h workspaceRecordWriter) prepareRecord(ctx context.Context, root *os.Root, name string) (*workspaceStage, error) {
	if h.writer == nil {
		return nil, nil
	}
	if name != filepath.Base(projectionMarkerPath(h.writer.target)) && name != filepath.Base(replacementJournalPath(h.writer.target)) {
		return nil, invalidReplacement("record_destination")
	}
	stage, err := prepareWorkspaceEnclosure(ctx, h.writer.target, h.writer)
	if err != nil {
		return nil, err
	}
	before, err := root.Lstat(".")
	if err == nil {
		var after os.FileInfo
		after, err = stage.parent.Lstat(".")
		if err == nil && !os.SameFile(before, after) {
			err = replacementConflict(name, "record parent differs from the workspace parent")
		}
	}
	if err != nil {
		return nil, stderrors.Join(err, stage.close(ctx))
	}
	return stage, nil
}

func (h workspaceRecordWriter) persistRecord(ctx context.Context, stage *workspaceStage, root *os.Root, destination string, file *os.File, state workspaceRecordState) error {
	if stage == nil {
		return nil
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := filepublish.SyncDirectory(root); err != nil {
		return err
	}
	if err := state.check(ctx, root); err != nil {
		return err
	}
	r := &preparationRecord{Destination: destination, Entry: state.entry, Identity: state.identity}
	if err := r.validate(stage, stage.journal.writer.target); err != nil {
		return err
	}
	if err := stage.journal.unchanged(ctx); err != nil {
		return err
	}
	if err := stage.journal.append(preparationEvent{Record: r}); err != nil {
		return err
	}
	stage.record = r
	if h.afterRecord != nil {
		return h.afterRecord(filepath.Join(root.Name(), state.entry.Path))
	}
	return nil
}

func (s *workspaceStage) cleanupRecord(ctx context.Context) error {
	if s.record == nil {
		return nil
	}
	if err := s.journal.unchanged(ctx); err != nil {
		return err
	}
	state := s.record.state()
	if err := state.check(ctx, s.parent); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := s.checkWriter(); err != nil {
		return err
	}
	if err := s.parent.Remove(state.entry.Path); err != nil {
		return err
	}
	return filepublish.SyncDirectory(s.parent)
}
