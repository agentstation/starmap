package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"hash"
	"io"
	"os"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
)

const (
	preparationJournalName    = ".preparation.jsonl"
	preparationJournalVersion = 2
	preparationJournalMax     = 32 << 20
	preparationEventMax       = 12 * replacementMaxEntries
	preparationTreeMax        = 4
	preparationScanMax        = 4096
)

type preparationHeader struct {
	Version         int               `json:"version"`
	Target          string            `json:"target"`
	Stage           string            `json:"stage"`
	LockIdentity    string            `json:"lock_identity"`
	JournalIdentity string            `json:"journal_identity"`
	Enclosure       treeSnapshot      `json:"enclosure"`
	Identities      map[string]string `json:"identities"`
}

type preparationEvent struct {
	Header   *preparationHeader  `json:"header,omitempty"`
	Handoff  *preparationHandoff `json:"handoff,omitempty"`
	Tree     string              `json:"tree,omitempty"`
	Entry    *treeEntry          `json:"entry,omitempty"`
	Identity string              `json:"identity,omitempty"`
}

type preparationJournal struct {
	stage   *workspaceStage
	writer  *workspaceWriter
	file    *os.File
	state   workspaceRecordState
	hash    hash.Hash
	events  int
	failure error
}

func newPreparationJournal(stage *workspaceStage, writer *workspaceWriter) (*preparationJournal, error) {
	file, err := privatefiles.CreateFile(stage.private, preparationJournalName)
	if err != nil {
		return nil, err
	}
	j := &preparationJournal{stage: stage, writer: writer, file: file, hash: sha256.New()}
	j.state, err = newWorkspaceRecordState(preparationJournalName, file)
	if err == nil {
		err = j.append(preparationEvent{Header: &preparationHeader{
			Version: preparationJournalVersion, Target: writer.target, Stage: stage.name,
			LockIdentity: writer.identity, JournalIdentity: j.state.identity,
			Enclosure: stage.enclosure, Identities: stage.enclosure.identities,
		}})
	}
	if err == nil {
		err = filepublish.SyncDirectory(stage.private)
	}
	if err == nil {
		err = filepublish.SyncDirectory(stage.parent)
	}
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return j, nil
}

func (j *preparationJournal) record(tree string, entry treeEntry, identity string) error {
	if err := filepublish.SyncDirectory(j.stage.trees[tree].root); err != nil {
		return err
	}
	if entry.Path == "." {
		if err := filepublish.SyncDirectory(j.stage.private); err != nil {
			return err
		}
	}
	return j.append(preparationEvent{Tree: tree, Entry: &entry, Identity: identity})
}

func (j *preparationJournal) append(event preparationEvent) error {
	if j.failure != nil {
		return j.failure
	}
	if err := j.check(); err != nil {
		return err
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if j.events >= preparationEventMax || int64(len(data)) > preparationJournalMax-j.state.entry.Size {
		return replacementLimit("preparation_journal")
	}
	n, err := j.file.Write(data)
	_, _ = j.hash.Write(data[:n])
	j.state.entry.Size += int64(n)
	j.state.entry.SHA256 = hex.EncodeToString(j.hash.Sum(nil))
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = j.file.Sync()
	}
	j.failure = err
	if err == nil {
		j.events++
	}
	return err
}

func (j *preparationJournal) check() error {
	if j.failure != nil {
		return j.failure
	}
	if err := j.writer.check(); err != nil {
		return err
	}
	if err := verifyCleanupRoot(j.stage.parent, j.stage.name, j.stage.private, j.stage.enclosure.ID); err != nil {
		return err
	}
	info, err := j.stage.private.Lstat(preparationJournalName)
	if err != nil {
		return err
	}
	opened, err := j.file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) || !info.Mode().IsRegular() || info.Size() != j.state.entry.Size {
		return replacementConflict(preparationJournalName, "preparation journal changed")
	}
	id, err := entryIdentity(j.file)
	if err != nil {
		return err
	}
	access, err := entryAccessDigest(j.file)
	if err != nil {
		return err
	}
	if id != j.state.identity || access != j.state.entry.AccessSHA256 || uint32(info.Mode()&workspaceAccessMode) != j.state.entry.Mode {
		return replacementConflict(preparationJournalName, "preparation journal identity or access changed")
	}
	if err := privatefiles.ValidateMetadata(info, "workspace preparation journal"); err != nil {
		return err
	}
	return privatefiles.ValidateACL(j.stage.private, preparationJournalName, info, "workspace preparation journal")
}

func (j *preparationJournal) unchanged(ctx context.Context) error {
	if err := j.check(); err != nil {
		return err
	}
	return j.state.check(ctx, j.stage.private)
}

func (j *preparationJournal) remove(ctx context.Context) error {
	if err := j.unchanged(ctx); err != nil {
		return err
	}
	if err := j.writer.check(); err != nil {
		return err
	}
	if err := j.stage.private.Remove(preparationJournalName); err != nil {
		return err
	}
	return stderrors.Join(j.file.Close(), filepublish.SyncDirectory(j.stage.private))
}
