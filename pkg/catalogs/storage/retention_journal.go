package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
)

const (
	retirementVersion    = 1
	retirementPrefix     = ".retirement-"
	retiredPrefix        = ".retired-"
	maxRetirementBytes   = 64 << 10
	maxRetirementRecords = 4
)

type retirementRecord struct {
	Name    string                     `json:"name"`
	Receipt privatefiles.RecordReceipt `json:"receipt"`
}

type retirementJournal struct {
	Version      int                       `json:"version"`
	GenerationID string                    `json:"generation_id"`
	Source       string                    `json:"source"`
	Retired      string                    `json:"retired"`
	Parent       privatefiles.EntryReceipt `json:"parent"`
	Writer       privatefiles.EntryReceipt `json:"writer"`
	Directory    privatefiles.EntryReceipt `json:"directory"`
	Files        []retirementRecord        `json:"files"`
}

func parseRetentionManifest(data []byte, directory string) (catalogs.GenerationManifest, error) {
	manifest, err := catalogs.ParseGenerationManifestJSON(data)
	if err != nil {
		return catalogs.GenerationManifest{}, err
	}
	if generationDirectoryID(manifest.GenerationID) != directory {
		return catalogs.GenerationManifest{}, retentionConflict(directory, "manifest identity does not match its directory")
	}
	return manifest, nil
}

func generationDirectoryID(id string) string {
	digest := sha256.Sum256([]byte(id))
	return hex.EncodeToString(digest[:])
}

func generationDirectoryName(name string) bool {
	decoded, err := hex.DecodeString(name)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == name
}

func (s *Filesystem) retireGeneration(ctx context.Context, parent *os.Root, journal retirementJournal) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var err error
	journal.Parent, err = privatefiles.CaptureEntry(parent, ".", true)
	if err != nil {
		return err
	}
	writerRoot, err := os.OpenRoot(s.root)
	if err != nil {
		return err
	}
	journal.Writer, err = privatefiles.CaptureEntry(writerRoot, ".commit.lock", false)
	_ = writerRoot.Close()
	if err != nil {
		return err
	}
	if err := s.checkRetirementTree(parent, journal.Source, journal, false); err != nil {
		return err
	}
	token := rand.Text()
	name := retirementPrefix + token + ".json"
	journal.Retired = retiredPrefix + token
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	if len(data) > maxRetirementBytes {
		return retentionConflict(journal.GenerationID, "retirement journal exceeds its byte limit")
	}
	directory, err := privatefiles.ExistingDirectory(parent.Name())
	if err != nil {
		return err
	}
	if err := directory.CompareAndPublishFileContext(ctx, name, nil, data, ".retirement-write-"); err != nil {
		return err
	}
	receipt, err := privatefiles.CaptureRecord(parent, name, maxRetirementBytes)
	if err != nil {
		return err
	}
	if err := s.retentionStep(ctx, "journaled", journal.GenerationID); err != nil {
		return err
	}
	if err := s.checkRetirementTree(parent, journal.Source, journal, false); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := filepublish.DirectoryNoReplace(parent, journal.Source, journal.Retired); err != nil {
		return err
	}
	if err := filepublish.SyncDirectory(parent); err != nil {
		return err
	}
	if err := s.retentionStep(ctx, "retired", journal.GenerationID); err != nil {
		return err
	}
	return s.cleanRetirement(ctx, parent, name, receipt, journal)
}

func (s *Filesystem) recoverRetirements(ctx context.Context, parent *os.Root, entries []fs.DirEntry, required map[string]bool, dry bool) error {
	type pending struct {
		name    string
		journal retirementJournal
		receipt privatefiles.RecordReceipt
	}
	var journals []pending
	claimed := make(map[string]bool)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := entry.Name()
		if generationDirectoryName(name) || strings.HasPrefix(name, retiredPrefix) || name == privatefiles.PublicationDirectoryName {
			continue
		}
		if !strings.HasPrefix(name, retirementPrefix) || !strings.HasSuffix(name, ".json") {
			return retentionConflict(name, "unrecognized generation entry")
		}
		data, err := privatefiles.ReadFile(parent, name, maxRetirementBytes)
		if err != nil {
			return err
		}
		journal, err := decodeRetirement(name, data)
		if err != nil {
			return err
		}
		receipt, err := privatefiles.CaptureRecord(parent, name, maxRetirementBytes)
		if err != nil {
			return err
		}
		if receipt.Digest != retentionDigest(data) {
			return retentionConflict(name, "retirement journal changed during access")
		}
		if err := s.checkRetirementOwner(parent, journal); err != nil {
			return err
		}
		journals = append(journals, pending{name, journal, receipt})
		claimed[journal.Retired] = true
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), retiredPrefix) && !claimed[entry.Name()] {
			return retentionConflict(entry.Name(), "retired directory has no recognized journal")
		}
	}
	if dry && len(journals) > 0 {
		return retentionConflict(parent.Name(), "retirement recovery requires an explicit collection pass")
	}
	for _, pending := range journals {
		journal := pending.journal
		_, sourceErr := parent.Lstat(journal.Source)
		_, retiredErr := parent.Lstat(journal.Retired)
		if sourceErr == nil {
			if !os.IsNotExist(retiredErr) {
				return retentionConflict(journal.GenerationID, "source and retired paths both exist or cannot be inspected")
			}
			// No rename occurred. A fresh pass must decide whether this generation still expires.
			if err := removeRetirementJournal(ctx, parent, pending.name, pending.receipt); err != nil {
				return err
			}
			continue
		}
		if !os.IsNotExist(sourceErr) {
			return sourceErr
		}
		if required[journal.GenerationID] {
			return retentionConflict(journal.GenerationID, "pending retirement is now required")
		}
		if err := s.cleanRetirement(ctx, parent, pending.name, pending.receipt, journal); err != nil {
			return err
		}
	}
	return nil
}

func (s *Filesystem) checkRetirementOwner(parent *os.Root, journal retirementJournal) error {
	if err := s.validateCommitLock(); err != nil {
		return err
	}
	directory, err := privatefiles.ExistingDirectory(filepath.Join(s.root, "generations"))
	if err != nil {
		return err
	}
	current, err := directory.Open()
	if err != nil {
		return err
	}
	err = privatefiles.CheckEntry(current, ".", true, journal.Parent)
	_ = current.Close()
	if err != nil {
		return err
	}
	if err := privatefiles.CheckEntry(parent, ".", true, journal.Parent); err != nil {
		return err
	}
	writerRoot, err := os.OpenRoot(s.root)
	if err != nil {
		return err
	}
	defer func() { _ = writerRoot.Close() }()
	return privatefiles.CheckEntry(writerRoot, ".commit.lock", false, journal.Writer)
}

func (s *Filesystem) checkRetirementTree(parent *os.Root, name string, journal retirementJournal, partial bool) error {
	if err := s.checkRetirementOwner(parent, journal); err != nil {
		return err
	}
	if err := privatefiles.CheckEntry(parent, name, true, journal.Directory); err != nil {
		return err
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	entries, err := readRetentionEntries(root, maxRetirementRecords)
	if err != nil {
		return err
	}
	want := make(map[string]privatefiles.RecordReceipt, len(journal.Files))
	for _, file := range journal.Files {
		want[file.Name] = file.Receipt
	}
	if !partial && len(entries) != len(want) {
		return retentionConflict(name, "generation contents changed before retirement")
	}
	for _, entry := range entries {
		receipt, found := want[entry.Name()]
		if !found {
			return retentionConflict(entry.Name(), "unrecognized retirement contents")
		}
		if err := privatefiles.CheckRecord(root, entry.Name(), receipt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Filesystem) cleanRetirement(ctx context.Context, parent *os.Root, name string, receipt privatefiles.RecordReceipt, journal retirementJournal) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.checkRetirementOwner(parent, journal); err != nil {
		return err
	}
	if _, err := parent.Lstat(journal.Retired); os.IsNotExist(err) {
		return removeRetirementJournal(ctx, parent, name, receipt)
	} else if err != nil {
		return err
	}
	if err := s.checkRetirementTree(parent, journal.Retired, journal, true); err != nil {
		return err
	}
	for _, file := range journal.Files {
		if err := s.retentionStep(ctx, "remove:"+file.Name, journal.GenerationID); err != nil {
			return err
		}
		if err := s.checkRetirementTree(parent, journal.Retired, journal, true); err != nil {
			return err
		}
		root, err := parent.OpenRoot(journal.Retired)
		if err != nil {
			return err
		}
		if _, err = root.Lstat(file.Name); os.IsNotExist(err) {
			_ = root.Close()
			continue
		}
		if err == nil {
			err = privatefiles.CheckRecord(root, file.Name, file.Receipt)
		}
		if err == nil {
			err = ctx.Err()
		}
		if err == nil {
			err = root.Remove(file.Name)
		}
		if err == nil {
			err = filepublish.SyncDirectory(root)
		}
		_ = root.Close()
		if err != nil {
			return err
		}
		if err := s.retentionStep(ctx, "removed:"+file.Name, journal.GenerationID); err != nil {
			return err
		}
	}
	if err := s.checkRetirementTree(parent, journal.Retired, journal, true); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := parent.Remove(journal.Retired); err != nil {
		return err
	}
	if err := filepublish.SyncDirectory(parent); err != nil {
		return err
	}
	return removeRetirementJournal(ctx, parent, name, receipt)
}

func removeRetirementJournal(ctx context.Context, parent *os.Root, name string, receipt privatefiles.RecordReceipt) error {
	if err := privatefiles.CheckRecord(parent, name, receipt); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := parent.Remove(name); err != nil {
		return err
	}
	return filepublish.SyncDirectory(parent)
}

func (s *Filesystem) retentionStep(ctx context.Context, phase, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.beforeRetentionStep != nil {
		return s.beforeRetentionStep(phase, id)
	}
	return nil
}

func decodeRetirement(name string, data []byte) (retirementJournal, error) {
	var journal retirementJournal
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&journal); err != nil {
		return journal, retentionConflict(name, "retirement journal has invalid JSON or unknown fields")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return journal, retentionConflict(name, "retirement journal contains trailing data")
	}
	canonical, err := json.Marshal(journal)
	if err != nil || !bytes.Equal(canonical, data) {
		return journal, retentionConflict(name, "retirement journal is not canonical JSON")
	}
	token := strings.TrimSuffix(strings.TrimPrefix(name, retirementPrefix), ".json")
	if !retirementToken(token) || journal.Version != retirementVersion || journal.Retired != retiredPrefix+token ||
		journal.GenerationID == "" || journal.Source != generationDirectoryID(journal.GenerationID) || len(journal.Files) < 2 || len(journal.Files) > maxRetirementRecords {
		return journal, retentionConflict(name, "unrecognized retirement journal")
	}
	seen := make(map[string]bool)
	for _, file := range journal.Files {
		limit, err := retentionRecordLimit(file.Name)
		if err != nil || seen[file.Name] || file.Receipt.Size < 0 || file.Receipt.Size > limit || file.Receipt.Entry.Identity == "" || file.Receipt.Entry.Access == "" ||
			len(file.Receipt.Digest) != sha256.Size*2 {
			return journal, retentionConflict(name, "invalid retirement file receipt")
		}
		seen[file.Name] = true
	}
	if !seen[manifestFilename] || !seen[payloadFilename] || journal.Parent.Identity == "" || journal.Writer.Identity == "" || journal.Directory.Identity == "" {
		return journal, retentionConflict(name, "incomplete retirement identity")
	}
	return journal, nil
}

func retentionDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func retirementToken(token string) bool {
	if len(token) < 26 || len(token) > 128 {
		return false
	}
	for _, char := range token {
		if (char < 'A' || char > 'Z') && (char < '2' || char > '7') {
			return false
		}
	}
	return true
}
