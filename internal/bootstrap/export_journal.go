package bootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
)

const (
	baselineJournalVersion  = 1
	baselineJournalLimit    = 32 << 10
	baselineStagePrefix     = ".baseline-"
	baselineJournalWriting  = "writing"
	baselineJournalCollect  = "collecting"
	baselineJournalTokenLen = 26
)

type baselineJournalRecord struct {
	Version      int                   `json:"version"`
	Phase        string                `json:"phase"`
	Stage        string                `json:"stage"`
	Target       string                `json:"target"`
	Identity     string                `json:"identity"`
	LockIdentity string                `json:"lock_identity"`
	Mode         uint32                `json:"mode"`
	Files        []baselineJournalFile `json:"files"`
}

type baselineJournalFile struct {
	Name     string    `json:"name"`
	Identity string    `json:"identity"`
	Mode     uint32    `json:"mode"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	SHA256   string    `json:"sha256"`
}

type baselineJournal struct {
	recovery *baselineRecovery
	name     string
	record   baselineJournalRecord
	raw      []byte
	info     os.FileInfo
}

func (r *baselineRecovery) newJournal(ctx context.Context, stage *baselineStage, target string) error {
	identity, err := baselineEntryIdentity(stage.root, ".", stage.identity)
	if err != nil {
		return err
	}
	journal := &baselineJournal{recovery: r, name: strings.TrimPrefix(stage.name, baselineStagePrefix) + ".json",
		record: baselineJournalRecord{Version: baselineJournalVersion, Phase: baselineJournalWriting, Stage: stage.name,
			Target: target, Identity: identity, LockIdentity: r.lockIdentity, Mode: uint32(stage.identity.Mode())}}
	stage.journal = journal
	return journal.save(ctx, stage, baselineJournalWriting)
}

func (j *baselineJournal) save(ctx context.Context, stage *baselineStage, phase string) error {
	if err := j.recovery.ownsLock(); err != nil {
		return err
	}
	if err := stage.validate(); err != nil {
		return err
	}
	if j.raw != nil {
		if err := j.unchanged(); err != nil {
			return err
		}
	}
	record := j.record
	record.Phase = phase
	record.Files = make([]baselineJournalFile, 0, len(stage.files))
	for _, file := range stage.files {
		identity, err := baselineEntryIdentity(stage.root, file.name, file.info)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(file.contents)
		record.Files = append(record.Files, baselineJournalFile{Name: file.name, Identity: identity, Mode: uint32(file.info.Mode()),
			Size: file.info.Size(), Modified: file.info.ModTime().UTC(), SHA256: hex.EncodeToString(digest[:])})
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if j.raw == nil {
		err = j.recovery.directory.WriteFileIfAbsentContext(ctx, j.name, data, ".record-")
	} else {
		err = j.recovery.directory.WriteFileContext(ctx, j.name, data, ".record-")
	}
	if err != nil {
		return err
	}
	info, err := j.recovery.root.Lstat(j.name)
	if err != nil {
		return err
	}
	j.record, j.raw, j.info = record, data, info
	return j.unchanged()
}

func (j *baselineJournal) unchanged() error {
	if err := j.recovery.ownsLock(); err != nil {
		return err
	}
	checked, err := j.recovery.directory.Open()
	if err != nil {
		return err
	}
	defer func() { _ = checked.Close() }()
	before, err := checked.Lstat(j.name)
	if err != nil || j.info == nil || !os.SameFile(j.info, before) || !sameBaselineMetadata(j.info, before) {
		return stderrors.Join(stageConflict(j.name), err)
	}
	data, err := privatefiles.ReadFile(checked, j.name, baselineJournalLimit)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, j.raw) {
		return stageConflict(j.name)
	}
	after, err := checked.Lstat(j.name)
	if err != nil || !os.SameFile(before, after) || !sameBaselineMetadata(before, after) {
		return stderrors.Join(stageConflict(j.name), err)
	}
	return nil
}

func (j *baselineJournal) remove(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := j.unchanged(); err != nil {
		return err
	}
	if err := j.recovery.root.Remove(j.name); err != nil {
		return err
	}
	return filepublish.SyncDirectory(j.recovery.root)
}

func (r *baselineRecovery) readJournal(name string) (*baselineJournal, error) {
	before, err := r.root.Lstat(name)
	if err != nil {
		return nil, err
	}
	data, err := privatefiles.ReadFile(r.root, name, baselineJournalLimit)
	if err != nil {
		return nil, err
	}
	var record baselineJournalRecord
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, stageConflict(name)
	}
	if !validBaselineJournal(name, record) {
		return nil, stageConflict(name)
	}
	journal := &baselineJournal{recovery: r, name: name, record: record, raw: data, info: before}
	if err := journal.unchanged(); err != nil {
		return nil, err
	}
	return journal, nil
}

func validBaselineJournal(name string, record baselineJournalRecord) bool {
	token := strings.TrimSuffix(name, ".json")
	if len(token) != baselineJournalTokenLen || name != token+".json" || record.Stage != baselineStagePrefix+token {
		return false
	}
	for _, char := range token {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567", char) {
			return false
		}
	}
	if record.Version != baselineJournalVersion || record.Phase != baselineJournalWriting && record.Phase != baselineJournalCollect || record.Identity == "" || record.LockIdentity == "" {
		return false
	}
	if !os.FileMode(record.Mode).IsDir() || len(record.Files) > 2 || !baselineDigest(record.Target) {
		return false
	}
	seen := map[string]bool{}
	for _, file := range record.Files {
		if file.Name != baselineManifestName && file.Name != baselinePayloadName || seen[file.Name] || file.Identity == "" || file.Size < 0 ||
			!os.FileMode(file.Mode).IsRegular() || !baselineDigest(file.SHA256) {
			return false
		}
		seen[file.Name] = true
	}
	return true
}

func baselineDigest(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == sha256.Size && hex.EncodeToString(data) == value
}

func baselineEntryIdentity(root *os.Root, name string, expected os.FileInfo) (string, error) {
	before, err := root.Lstat(name)
	if err != nil || expected == nil || !os.SameFile(expected, before) || before.Mode()&os.ModeSymlink != 0 {
		return "", stderrors.Join(stageConflict(name), err)
	}
	file, err := root.Open(name)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return "", stderrors.Join(stageConflict(name), err)
	}
	return filepublish.Identity(file)
}

func sameBaselineMetadata(before, after os.FileInfo) bool {
	return before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}
