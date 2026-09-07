package workspace

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
)

type replacementRecord struct {
	Version   int              `json:"version"`
	Target    string           `json:"target"`
	Candidate string           `json:"candidate"`
	Backup    string           `json:"backup"`
	Old       treeSnapshot     `json:"old"`
	New       treeSnapshot     `json:"new"`
	Marker    projectionMarker `json:"marker"`
}

func replacementJournalPath(target string) string {
	return filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".starmap-replacement.json")
}

func pendingReplacement(target string) error {
	_, err := os.Lstat(replacementJournalPath(target))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return errors.WrapIO("inspect", replacementJournalPath(target), err)
	}
	return readConflict(target, "workspace replacement requires recovery before another read")
}

func (r replacementRecord) validate(target string) error {
	if r.Version == 1 {
		return &errors.ValidationError{Field: "workspace_replacement.version", Message: "journal predates access snapshots. Preserve the workspace, candidate, and backup for explicit recovery"}
	}
	base := filepath.Base(target)
	prefix := "." + base + ".candidate-"
	suffix := strings.TrimPrefix(r.Candidate, prefix)
	if r.Version != replacementVersion || r.Target != target || suffix == "" || suffix == r.Candidate ||
		!replacementChildName(r.Candidate) || r.Backup != "."+base+".backup-"+suffix ||
		!replacementChildName(r.Backup) || r.Old.ID == r.New.ID {
		return invalidReplacement("record")
	}
	if err := r.Old.validate(); err != nil {
		return err
	}
	if err := r.New.validate(); err != nil {
		return err
	}
	if r.Marker.Version != markerVersion || strings.TrimSpace(r.Marker.GenerationID) == "" ||
		!replacementCatalogDigest(r.Marker.PayloadChecksum) || !replacementCatalogDigest(r.Marker.WorkspaceChecksum) ||
		!replacementCatalogDigest(r.Marker.EndpointChecksum) {
		return invalidReplacement("marker")
	}
	return nil
}

func (s treeSnapshot) validate() error {
	if s.ID == "" || len(s.ID) > replacementIdentityMax || !replacementDigest(s.Digest) ||
		len(s.Entries) == 0 || len(s.Entries) > replacementMaxEntries {
		return invalidReplacement("inventory")
	}
	parents := make(map[string]bool, len(s.Entries))
	var bytes, nameBytes int64
	previous := ""
	for i, entry := range s.Entries {
		if !fs.ValidPath(entry.Path) || !filepath.IsLocal(filepath.FromSlash(entry.Path)) ||
			strings.ContainsAny(entry.Path, "\\:\x00") || entry.Path <= previous || entry.Mode&^uint32(workspaceAccessMode) != 0 ||
			!replacementDigest(entry.AccessSHA256) {
			return invalidReplacement("entry")
		}
		if i == 0 {
			if entry.Path != "." || !entry.Directory {
				return invalidReplacement("root")
			}
		} else if !parents[path.Dir(entry.Path)] {
			return invalidReplacement("parent")
		}
		if entry.Directory {
			if entry.Size != 0 || entry.SHA256 != "" {
				return invalidReplacement("directory")
			}
		} else if entry.Size < 0 || entry.Size > replacementMaxBytes-bytes || !replacementDigest(entry.SHA256) {
			return invalidReplacement("file")
		}
		bytes += entry.Size
		nameBytes += int64(len(entry.Path))
		if nameBytes > replacementMaxNameBytes {
			return replacementLimit("names")
		}
		parents[entry.Path] = entry.Directory
		previous = entry.Path
	}
	digest, err := treeEntriesDigest(s.Entries)
	if err != nil {
		return err
	}
	if digest != s.Digest {
		return invalidReplacement("digest")
	}
	return nil
}

func replacementChildName(name string) bool {
	return filepath.IsLocal(name) && filepath.Base(name) == name &&
		!strings.ContainsAny(name, "/\\:\x00") && strings.TrimRight(name, " .") == name
}

func replacementDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func replacementCatalogDigest(value string) bool {
	digest, ok := strings.CutPrefix(value, "sha256:")
	return ok && replacementDigest(digest)
}

func invalidReplacement(field string) error {
	return &errors.ValidationError{Field: "workspace_replacement." + field, Message: "invalid replacement journal"}
}

func writeReplacementRecord(root *os.Root, record replacementRecord) (bool, error) {
	if err := record.validate(record.Target); err != nil {
		return false, err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return false, err
	}
	if len(data)+1 > replacementJournalMax {
		return false, replacementLimit("journal")
	}
	data = append(data, '\n')
	name := filepath.Base(replacementJournalPath(record.Target))
	temporary := "." + name + "." + rand.Text()
	if err := writeReplacementBytes(root, temporary, data); err != nil {
		return false, err
	}
	defer func() { _ = root.Remove(temporary) }()
	if err := root.Link(temporary, name); err != nil {
		return false, err
	}
	return true, filepublish.SyncDirectory(root)
}

func writeReplacementBytes(root *os.Root, name string, data []byte) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	complete := false
	defer func() {
		if !complete {
			_ = root.Remove(name)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}

func readReplacementRecord(root *os.Root, target string) (replacementRecord, error) {
	name := filepath.Base(replacementJournalPath(target))
	info, err := root.Lstat(name)
	if err != nil {
		return replacementRecord{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > replacementJournalMax {
		return replacementRecord{}, invalidReplacement("file")
	}
	file, err := root.Open(name)
	if err != nil {
		return replacementRecord{}, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return replacementRecord{}, err
	}
	if !os.SameFile(info, opened) {
		return replacementRecord{}, invalidReplacement("identity")
	}
	data, err := io.ReadAll(io.LimitReader(file, replacementJournalMax+1))
	if err != nil {
		return replacementRecord{}, err
	}
	if len(data) > replacementJournalMax {
		return replacementRecord{}, replacementLimit("journal")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record replacementRecord
	if err := decoder.Decode(&record); err != nil {
		return replacementRecord{}, &errors.ParseError{Format: "json", File: name, Message: "invalid replacement journal", Err: err}
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return replacementRecord{}, invalidReplacement("trailing_data")
	}
	return record, record.validate(target)
}
