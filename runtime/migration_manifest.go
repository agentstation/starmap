package runtime

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	migrationManifestName     = "manifest.json"
	migrationJournalName      = "journal.ndjson"
	migrationManifestMaxBytes = 4 << 20
	migrationManifestMaxFiles = 10000
	migrationJournalMaxBytes  = 16 << 10
	migrationIdentityMaxBytes = 256
	migrationPendingName      = ".migration-pending.json"
	migrationWorkDirectory    = ".migration-work"
	migrationRetiredName      = ".migration-retired.json"
	migrationReceiptName      = ".migration-receipt.json"
	migrationCompletionName   = ".migration-completed.json"
)

type directoryMigrationFile struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type directoryMigrationManifest struct {
	SchemaVersion       int                      `json:"schema_version"`
	OperationID         string                   `json:"operation_id"`
	SourceDirectory     string                   `json:"source_directory"`
	TargetDirectory     string                   `json:"target_directory"`
	SourceIdentity      string                   `json:"source_identity"`
	SourceOwnerSHA256   string                   `json:"source_owner_sha256,omitempty"`
	SourceReceiptSHA256 string                   `json:"source_receipt_sha256,omitempty"`
	Owner               DirectoryOwner           `json:"owner"`
	Files               []directoryMigrationFile `json:"files"`
}

func (m directoryMigrationManifest) encode() ([]byte, error) {
	if err := m.validateIntent(); err != nil {
		return nil, err
	}
	if err := validateMigrationFiles(m.Files); err != nil {
		return nil, err
	}
	m.Files = slices.Clone(m.Files)
	slices.SortFunc(m.Files, func(a, b directoryMigrationFile) int { return cmp.Compare(a.Source, b.Source) })
	encoded, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(encoded)+1 > migrationManifestMaxBytes {
		return nil, invalidMigrationIntent("manifest_size")
	}
	return append(encoded, '\n'), nil
}

func invalidMigrationIntent(field string) error {
	return &errors.ValidationError{Field: "migration." + field, Message: "invalid migration intent"}
}

func (m directoryMigrationManifest) validateIntent() error {
	if m.SchemaVersion != 1 {
		return invalidMigrationIntent("schema_version")
	}
	for field, value := range map[string]string{"operation_id": m.OperationID, "source_identity": m.SourceIdentity} {
		if value == "" || len(value) > migrationIdentityMaxBytes || strings.TrimSpace(value) != value || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
			return invalidMigrationIntent(field)
		}
	}
	if !absoluteMigrationPath(m.SourceDirectory) || !absoluteMigrationPath(m.TargetDirectory) || migrationPathsOverlap(m.SourceDirectory, m.TargetDirectory) {
		return invalidMigrationIntent("directories")
	}
	if err := m.Owner.Validate(); err != nil {
		return err
	}
	if m.SourceOwnerSHA256 != "" && !migrationDigestValid(m.SourceOwnerSHA256) {
		return invalidMigrationIntent("source_owner_sha256")
	}
	if m.SourceReceiptSHA256 != "" && !migrationDigestValid(m.SourceReceiptSHA256) {
		return invalidMigrationIntent("source_receipt_sha256")
	}
	return nil
}

func validateMigrationFiles(files []directoryMigrationFile) error {
	if len(files) == 0 || len(files) > migrationManifestMaxFiles {
		return invalidMigrationIntent("files")
	}
	targets := make(map[string]bool, len(files))
	sources := make(map[string]bool, len(files))
	seed := false
	for _, file := range files {
		if !migrationRelativeFile(file.Source) || !migrationRelativeFile(file.Target) || file.Size < 0 || !migrationDigestValid(file.SHA256) || sources[file.Source] || targets[file.Target] {
			return invalidMigrationIntent("files")
		}
		first, _, _ := strings.Cut(file.Target, "/")
		if slices.Contains([]string{ownerRecordName, directoryLockName, migrationManifestName, migrationJournalName, migrationPendingName, migrationWorkDirectory, migrationReceiptName, migrationRetiredName, migrationCompletionName}, first) {
			return invalidMigrationIntent("files.target")
		}
		if file.Target == instanceSeedFileName {
			if file.Size != int64(hex.EncodedLen(instanceSeedBytes)) || (file.Source != instanceSeedFileName && file.Source != layerDirectoryName+"/"+instanceSeedFileName) {
				return invalidMigrationIntent("seed")
			}
			seed = true
		}
		sources[file.Source], targets[file.Target] = true, true
	}
	if !seed {
		return invalidMigrationIntent("seed")
	}
	for _, names := range []map[string]bool{sources, targets} {
		for name := range names {
			for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
				if names[parent] {
					return invalidMigrationIntent("files")
				}
			}
		}
	}
	return nil
}

func absoluteMigrationPath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && !strings.ContainsRune(path, '\x00')
}

func migrationRelativeFile(path string) bool {
	return fs.ValidPath(path) && path != "." && !strings.ContainsAny(path, "\\\x00") && filepath.IsLocal(filepath.FromSlash(path))
}

func migrationPathsOverlap(a, b string) bool {
	within := func(parent, child string) bool {
		relative, err := filepath.Rel(parent, child)
		return err == nil && (relative == "." || filepath.IsLocal(relative))
	}
	return within(a, b) || within(b, a)
}

func migrationDigestValid(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == value
}
