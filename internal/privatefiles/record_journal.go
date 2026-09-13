package privatefiles

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	publicationJournalSuffix    = ".jsonl"
	publicationJournalMaxBytes  = 64 << 10
	publicationJournalMaxEvents = 3
	publicationJournalVersion   = 1
)

type publicationHeader struct {
	Version     int               `json:"version"`
	Stage       string            `json:"stage"`
	Prefix      string            `json:"prefix"`
	Destination string            `json:"destination"`
	Parent      publicationEntry  `json:"parent"`
	Metadata    publicationEntry  `json:"metadata"`
	Owner       publicationRecord `json:"owner"`
	Journal     publicationEntry  `json:"journal"`
}

type publicationEvent struct {
	Header *publicationHeader `json:"header,omitempty"`
	Record *publicationRecord `json:"record,omitempty"`
}

type publicationJournal struct {
	contents []byte
	name     string
	header   publicationHeader
	state    *publicationRecord
	receipt  publicationRecord
	file     *os.File
}

func (w *publicationWriter) newJournal(name, prefix string) (*publicationJournal, error) {
	nonce := rand.Text()
	journal := &publicationJournal{name: nonce + publicationJournalSuffix}
	journal.header = publicationHeader{
		Version: publicationJournalVersion, Stage: prefix + nonce, Prefix: prefix, Destination: name,
		Parent: w.parent, Metadata: w.meta, Owner: w.owner,
	}
	file, err := CreateFile(w.records, journal.name)
	if err != nil {
		return nil, err
	}
	journal.file = file
	entry, err := publicationEntryOf(w.records, journal.name, false)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	journal.header.Journal = entry
	if err := journal.append(w, publicationEvent{Header: &journal.header}); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := filepublish.SyncDirectory(w.records); err != nil {
		_ = file.Close()
		return nil, err
	}
	return journal, nil
}

func (j *publicationJournal) append(w *publicationWriter, event publicationEvent) error {
	if err := w.check(); err != nil {
		return err
	}
	if j.receipt.Entry.Identity != "" {
		if err := checkPublicationRecord(w.records, j.name, j.receipt); err != nil {
			return err
		}
	} else {
		if err := checkPublicationEntry(w.records, j.name, false, j.header.Journal); err != nil {
			return err
		}
		current, err := recordInfo(w.records, j.name)
		if err != nil {
			return err
		}
		if current.Size() != 0 {
			return changed(j.name)
		}
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	info, err := j.file.Stat()
	if err != nil {
		return err
	}
	if info.Size()+int64(len(raw)) > publicationJournalMaxBytes {
		return oversized(j.name, publicationJournalMaxBytes)
	}
	n, err := j.file.Write(raw)
	if err != nil {
		return err
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	if err := j.file.Sync(); err != nil {
		return err
	}
	receipt, err := publicationRecordOf(w.records, j.name, publicationJournalMaxBytes)
	if err != nil {
		return err
	}
	expected := append(bytes.Clone(j.contents), raw...)
	if receipt.Entry != j.header.Journal || receipt.Digest != publicationDigest(expected) || receipt.Size != int64(len(expected)) {
		return changed(j.name)
	}
	j.contents = expected
	j.receipt = receipt
	if event.Record != nil {
		copied := *event.Record
		j.state = &copied
	}
	return nil
}

func (w *publicationWriter) readJournal(name string) (*publicationJournal, error) {
	if err := childName(name); err != nil {
		return nil, err
	}
	nonce, ok := strings.CutSuffix(name, publicationJournalSuffix)
	if !ok || len(nonce) != 26 || strings.Trim(nonce, "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567") != "" {
		return nil, changed(name)
	}
	receipt, err := publicationRecordOf(w.records, name, publicationJournalMaxBytes)
	if err != nil {
		return nil, err
	}
	raw, err := ReadFile(w.records, name, publicationJournalMaxBytes)
	if err != nil {
		return nil, err
	}
	if publicationDigest(raw) != receipt.Digest || len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return nil, changed(name)
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) > publicationJournalMaxEvents {
		return nil, oversized(name, publicationJournalMaxBytes)
	}
	journal := &publicationJournal{name: name, receipt: receipt}
	for index, line := range lines {
		event, err := decodePublicationEvent(name, line)
		if err != nil {
			return nil, err
		}
		if index == 0 {
			if event.Header == nil || event.Record != nil {
				return nil, changed(name)
			}
			journal.header = *event.Header
		} else {
			if event.Header != nil || event.Record == nil {
				return nil, changed(name)
			}
			if !validPublicationRecord(*event.Record) {
				return nil, changed(name)
			}
			if index == 1 && (event.Record.Size != 0 || event.Record.Digest != publicationDigest(nil)) {
				return nil, changed(name)
			}
			if journal.state != nil && journal.state.Entry != event.Record.Entry {
				return nil, changed(name)
			}
			journal.state = event.Record
		}
	}
	if err := journal.validateHeader(w, nonce, receipt.Entry); err != nil {
		return nil, err
	}
	return journal, nil
}

func validPublicationRecord(record publicationRecord) bool {
	return record.Entry.Identity != "" && len(record.Entry.Access) == 64 && record.Size >= 0 && record.Size <= publicationRecordMaxBytes && len(record.Digest) == 64
}

func decodePublicationEvent(name string, line []byte) (publicationEvent, error) {
	var event publicationEvent
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		return publicationEvent{}, errors.NewParseError("json", "private publication receipt", "cannot decode the receipt", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return publicationEvent{}, changed(name)
	}

	canonical, err := json.Marshal(event)
	if err != nil {
		return publicationEvent{}, err
	}
	if !bytes.Equal(canonical, line) {
		return publicationEvent{}, changed(name)
	}
	return event, nil
}

func (j *publicationJournal) validateHeader(w *publicationWriter, nonce string, entry publicationEntry) error {
	h := j.header
	if h.Version != publicationJournalVersion || h.Parent != w.parent || h.Metadata != w.meta || h.Owner != w.owner || h.Journal != entry {
		return changed(j.name)
	}
	if childName(h.Destination) != nil || childName(h.Prefix) != nil || childName(h.Stage) != nil || h.Stage != h.Prefix+nonce || h.Stage == h.Destination || h.Destination == publicationDirectory {
		return changed(j.name)
	}
	return nil
}
