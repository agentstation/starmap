package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

type replacementRecord struct {
	LockIdentity  string            `json:"lock_identity"`
	Version       int               `json:"version"`
	Target        string            `json:"target"`
	Candidate     string            `json:"candidate"`
	Backup        string            `json:"backup"`
	Old           treeSnapshot      `json:"old"`
	New           treeSnapshot      `json:"new"`
	Marker        projectionMarker  `json:"marker"`
	OldIdentities map[string]string `json:"old_identities"`
	NewIdentities map[string]string `json:"new_identities"`
	journal       workspaceRecordState
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
	if r.Version == 2 {
		return &errors.ValidationError{Field: "workspace_replacement.version", Message: "journal predates child identity snapshots. Preserve the workspace, candidate, and backup for explicit recovery"}
	}
	base := filepath.Base(target)
	prefix := "." + base + ".candidate-"
	suffix := strings.TrimPrefix(r.Candidate, prefix)
	if r.Version != replacementVersion || r.Target != target || suffix == "" || suffix == r.Candidate ||
		!replacementChildName(r.Candidate) || r.Backup != "."+base+".backup-"+suffix ||
		!replacementChildName(r.Backup) || r.Old.ID == r.New.ID {
		return invalidReplacement("record")
	}
	if r.LockIdentity == "" || len(r.LockIdentity) > replacementIdentityMax {
		return invalidReplacement("writer_identity")
	}
	if err := r.Old.validate(); err != nil {
		return err
	}
	if err := r.New.validate(); err != nil {
		return err
	}
	if err := validateReplacementIdentities(r.Old, r.OldIdentities); err != nil {
		return err
	}
	if err := validateReplacementIdentities(r.New, r.NewIdentities); err != nil {
		return err
	}
	if r.Marker.Version != markerVersion || strings.TrimSpace(r.Marker.GenerationID) == "" ||
		!replacementCatalogDigest(r.Marker.PayloadChecksum) || !replacementCatalogDigest(r.Marker.WorkspaceChecksum) ||
		!replacementCatalogDigest(r.Marker.EndpointChecksum) {
		return invalidReplacement("marker")
	}
	return nil
}

func validateReplacementIdentities(tree treeSnapshot, identities map[string]string) error {
	if len(identities) != len(tree.Entries) || identities["."] != tree.ID {
		return invalidReplacement("identities")
	}
	for _, entry := range tree.Entries {
		id := identities[entry.Path]
		if id == "" || len(id) > replacementIdentityMax {
			return invalidReplacement("identities")
		}
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

func writeReplacementRecord(ctx context.Context, root *os.Root, record *replacementRecord, hooks workspaceRecordWriter) (bool, error) {
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
	record.journal, err = hooks.publish(ctx, root, name, data, recordPublication{})
	return record.journal.identity != "", err
}

func readReplacementRecord(root *os.Root, target string) (replacementRecord, error) {
	name := filepath.Base(replacementJournalPath(target))
	data, state, err := readWorkspaceRecord(root, name, replacementJournalMax)
	if err != nil {
		return replacementRecord{}, err
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
	if err := record.validate(target); err != nil {
		return replacementRecord{}, err
	}
	record.Old.identities, record.New.identities = record.OldIdentities, record.NewIdentities
	record.journal = state
	return record, nil
}
