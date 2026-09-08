package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths"
)

const (
	ownerRecordName      = "owner.json"
	ownerSchemaVersion   = 1
	ownerRecordMaxBytes  = 4096
	deploymentIDMaxBytes = 256
)

// DirectoryOwner binds persistent runtime state to one product, deployment, and instance.
// This record does not replace the live directory lock or a shared identity fence.
type DirectoryOwner struct {
	Product    string `json:"product"`
	Deployment string `json:"deployment"`
	Instance   string `json:"instance"`
}

// Validate checks the ownership identity without creating files.
func (o DirectoryOwner) Validate() error {
	if o.Product != "starmap" && o.Product != "starport" {
		return &errors.ValidationError{Field: "directory_owner.product", Message: "must be starmap or starport"}
	}
	if o.Deployment == "" || strings.TrimSpace(o.Deployment) != o.Deployment || len(o.Deployment) > deploymentIDMaxBytes || !utf8.ValidString(o.Deployment) || strings.ContainsFunc(o.Deployment, unicode.IsControl) {
		return &errors.ValidationError{Field: "directory_owner.deployment", Message: "must be a nonempty deployment identity with at most 256 UTF-8 bytes and no control characters"}
	}
	return productpaths.ValidateInstanceID(o.Instance)
}

// WithDirectoryOwner selects the identity recorded for persistent runtime state.
// A different recorded identity requires an explicit migration before startup.
func WithDirectoryOwner(owner DirectoryOwner) Option {
	return func(r *options) error {
		if err := owner.Validate(); err != nil {
			return err
		}
		r.directoryOwner = owner
		return nil
	}
}

type ownerRecord struct {
	SchemaVersion           int    `json:"schema_version"`
	SchedulerIdentitySHA256 string `json:"scheduler_identity_sha256,omitempty"`
	DirectoryOwner
}

// bindDirectoryOwner runs only while Open holds the directory lock.
func bindDirectoryOwner(ctx context.Context, directory string, owner DirectoryOwner, schedulerIdentity string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if directory == "" {
		return nil
	}
	if err := owner.Validate(); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	encoded, err := encodeOwnerRecord(owner, schedulerIdentity)
	if err != nil {
		return err
	}
	if _, err := root.Lstat(ownerRecordName); err == nil {
		return verifyOwnerRecord(root, encoded)
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, name := range []string{layerDirectoryName, "github-catalog-source", instanceSeedFileName} {
		if _, err := root.Lstat(name); err == nil {
			return &errors.ConflictError{Resource: "runtime directory owner", Message: "existing state has no ownership record and requires explicit migration"}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := writeOwnerFile(ctx, root, ownerRecordName, encoded); err != nil && !os.IsExist(err) {
		return err
	}
	return verifyOwnerRecord(root, encoded)
}

func encodeOwnerRecord(owner DirectoryOwner, schedulerIdentity string) ([]byte, error) {
	record := ownerRecord{SchemaVersion: ownerSchemaVersion, DirectoryOwner: owner}
	if identity := strings.TrimSpace(schedulerIdentity); identity != "" {
		digest := sha256.Sum256([]byte(identity))
		record.SchedulerIdentitySHA256 = hex.EncodeToString(digest[:])
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// writeOwnerFile publishes complete private bytes without replacing an existing file.
func writeOwnerFile(ctx context.Context, root *os.Root, name string, encoded []byte) error {
	stage := ".owner-" + rand.Text()
	file, err := createPrivateRuntimeFile(root, stage)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close(); _ = root.Remove(stage) }()
	if _, err := file.Write(encoded); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return root.Link(stage, name)
}

func verifyOwnerRecord(root *os.Root, expected []byte) error {
	conflict := func() error {
		return &errors.ConflictError{Resource: "runtime directory owner", Message: "record is invalid or belongs to another product, deployment, instance, or schema"}
	}
	info, err := root.Lstat(ownerRecordName)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > ownerRecordMaxBytes {
		return conflict()
	}
	file, err := root.Open(ownerRecordName)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(io.LimitReader(file, ownerRecordMaxBytes+1))
	if err != nil {
		return err
	}
	// Product-owned records use one canonical encoding, including the schema version.
	if !bytes.Equal(raw, expected) {
		return conflict()
	}
	return filepublish.SyncDirectory(root)
}
