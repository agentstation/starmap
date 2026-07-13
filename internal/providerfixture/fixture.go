// Package providerfixture owns integrity-bound raw provider fixture refreshes.
package providerfixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

// DefaultMaxAge is the reviewed freshness budget for provider fixtures.
const DefaultMaxAge = 365 * 24 * time.Hour

// Metadata binds a provider fixture to source revision and freshness evidence.
type Metadata struct {
	Version        uint64                          `json:"version"`
	Provider       string                          `json:"provider"`
	FetchedAt      time.Time                       `json:"fetched_at"`
	SourceRevision catalogmeta.ObservationRevision `json:"source_revision"`
	Payload        Payload                         `json:"payload"`
	MaxAge         string                          `json:"max_age"`
}

// Payload identifies the exact governed fixture bytes.
type Payload struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
}

// FetchResult is the secret-free response material eligible for promotion.
type FetchResult struct {
	Payload  []byte
	Revision catalogmeta.ObservationRevision
}

// RefreshOptions contains explicit refresh dependencies for deterministic tests.
type RefreshOptions struct {
	Provider       string
	FixturePath    string
	Now            time.Time
	ForbiddenBytes [][]byte
	Fetch          func(context.Context) (FetchResult, error)
	Validate       func(context.Context, []byte) error
}

// RefreshResult reports promoted fixture identity without exposing payload data.
type RefreshResult struct {
	Provider string
	Checksum string
	Bytes    int
}

// Refresh fetches, validates, and atomically promotes a changed fixture pair.
func Refresh(ctx context.Context, options RefreshOptions) (RefreshResult, error) {
	if strings.TrimSpace(options.Provider) == "" || strings.TrimSpace(options.FixturePath) == "" || options.Fetch == nil || options.Validate == nil {
		return RefreshResult{}, &errors.ValidationError{Field: "provider_fixture.options", Message: "provider, fixture path, fetch, and validation are required"}
	}
	fetched, err := options.Fetch(ctx)
	if err != nil {
		return RefreshResult{}, errors.WrapResource("fetch", "provider fixture", options.Provider, err)
	}
	if len(fetched.Payload) == 0 || len(fetched.Payload) > constants.MaxSourcePayloadBytes {
		return RefreshResult{}, &errors.ValidationError{Field: "provider_fixture.payload", Value: len(fetched.Payload), Message: "must be non-empty and within the source payload limit"}
	}
	for _, forbidden := range options.ForbiddenBytes {
		if len(forbidden) > 0 && bytes.Contains(fetched.Payload, forbidden) {
			return RefreshResult{}, &errors.ValidationError{Field: "provider_fixture.payload", Message: "contains configured secret material"}
		}
	}
	if err := options.Validate(ctx, fetched.Payload); err != nil {
		return RefreshResult{}, errors.WrapResource("validate", "provider fixture", options.Provider, err)
	}
	checksum := Checksum(fetched.Payload)
	if current, readErr := os.ReadFile(options.FixturePath); readErr == nil && Checksum(current) == checksum {
		return RefreshResult{}, &errors.ConflictError{Resource: "provider fixture", Expected: "changed payload", Actual: checksum, Message: "refresh was a no-op"}
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return RefreshResult{}, errors.WrapIO("read", options.FixturePath, readErr)
	}
	now := options.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	revision := fetched.Revision
	if revision.Kind == "" || strings.TrimSpace(revision.Value) == "" {
		revision = catalogmeta.ObservationRevision{Kind: catalogmeta.ObservationRevisionKindContentDigest, Value: checksum}
	}
	metadata := Metadata{
		Version: 1, Provider: options.Provider, FetchedAt: now, SourceRevision: revision,
		Payload: Payload{Path: filepath.Base(options.FixturePath), Checksum: checksum}, MaxAge: DefaultMaxAge.String(),
	}
	metadataData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return RefreshResult{}, errors.WrapParse("json", "provider fixture metadata", err)
	}
	metadataData = append(metadataData, '\n')
	if err := promotePair(options.FixturePath, fetched.Payload, MetadataPath(options.FixturePath), metadataData); err != nil {
		return RefreshResult{}, err
	}
	if err := Verify(options.FixturePath, now); err != nil {
		return RefreshResult{}, errors.WrapResource("verify", "provider fixture", options.Provider, err)
	}
	return RefreshResult{Provider: options.Provider, Checksum: checksum, Bytes: len(fetched.Payload)}, nil
}

// Verify validates payload identity, source revision, and freshness.
func Verify(fixturePath string, now time.Time) error {
	payload, err := os.ReadFile(fixturePath) //nolint:gosec // Operator-selected fixture path.
	if err != nil {
		return errors.WrapIO("read", fixturePath, err)
	}
	metadataData, err := os.ReadFile(MetadataPath(fixturePath)) //nolint:gosec // Adjacent controlled metadata.
	if err != nil {
		return errors.WrapIO("read", MetadataPath(fixturePath), err)
	}
	var metadata Metadata
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		return &errors.ValidationError{Field: "fixture.metadata", Value: MetadataPath(fixturePath), Message: err.Error()}
	}
	if metadata.Version != 1 || strings.TrimSpace(metadata.Provider) == "" {
		return &errors.ValidationError{Field: "fixture.metadata", Value: metadata.Version, Message: "version 1 and provider are required"}
	}
	expectedProvider := filepath.Base(filepath.Dir(filepath.Dir(fixturePath)))
	if metadata.Provider != expectedProvider || metadata.Payload.Path != filepath.Base(fixturePath) {
		return &errors.ValidationError{Field: "fixture.identity", Value: metadata.Provider, Message: "must match provider directory and adjacent payload"}
	}
	checksum := Checksum(payload)
	if metadata.Payload.Checksum != checksum {
		return &errors.ValidationError{Field: "fixture.payload.checksum", Value: metadata.Payload.Checksum, Message: "does not match fixture bytes"}
	}
	if metadata.SourceRevision.Kind == "" || strings.TrimSpace(metadata.SourceRevision.Value) == "" {
		return &errors.ValidationError{Field: "fixture.source_revision", Value: metadata.SourceRevision, Message: "kind and value are required"}
	}
	switch metadata.SourceRevision.Kind {
	case catalogmeta.ObservationRevisionKindETag, catalogmeta.ObservationRevisionKindLastModified,
		catalogmeta.ObservationRevisionKindGitCommit, catalogmeta.ObservationRevisionKindSourceVersion,
		catalogmeta.ObservationRevisionKindContentDigest:
	default:
		return &errors.ValidationError{Field: "fixture.source_revision.kind", Value: metadata.SourceRevision.Kind, Message: "is not supported"}
	}
	if metadata.SourceRevision.Kind == catalogmeta.ObservationRevisionKindContentDigest && metadata.SourceRevision.Value != checksum {
		return &errors.ValidationError{Field: "fixture.source_revision", Value: metadata.SourceRevision, Message: "content revision must match fixture bytes"}
	}
	if metadata.FetchedAt.IsZero() || metadata.FetchedAt.After(now.Add(5*time.Minute)) {
		return &errors.ValidationError{Field: "fixture.fetched_at", Value: metadata.FetchedAt, Message: "must be a non-future capture time"}
	}
	maxAge, err := time.ParseDuration(metadata.MaxAge)
	if err != nil || maxAge <= 0 || now.Sub(metadata.FetchedAt) > maxAge {
		return &errors.ValidationError{Field: "fixture.fetched_at", Value: metadata.FetchedAt, Message: "fixture freshness policy is invalid or stale"}
	}
	return nil
}

// MetadataPath returns the adjacent metadata path for a fixture.
func MetadataPath(fixturePath string) string {
	return strings.TrimSuffix(fixturePath, filepath.Ext(fixturePath)) + ".metadata.json"
}

// Checksum returns the fixture content digest.
func Checksum(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func promotePair(payloadPath string, payload []byte, metadataPath string, metadata []byte) error {
	if filepath.Dir(payloadPath) != filepath.Dir(metadataPath) {
		return &errors.ValidationError{Field: "provider_fixture.path", Message: "payload and metadata must be adjacent"}
	}
	if err := os.MkdirAll(filepath.Dir(payloadPath), constants.DirPermissions); err != nil {
		return errors.WrapIO("create", filepath.Dir(payloadPath), err)
	}
	previousPayload, payloadExisted, err := readOptional(payloadPath)
	if err != nil {
		return err
	}
	payloadTemp, err := writeTemp(payloadPath, payload)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(payloadTemp) }()
	metadataTemp, err := writeTemp(metadataPath, metadata)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(metadataTemp) }()
	if err := os.Rename(payloadTemp, payloadPath); err != nil {
		return errors.WrapIO("rename", payloadPath, err)
	}
	if err := os.Rename(metadataTemp, metadataPath); err != nil {
		if rollbackErr := restoreFile(payloadPath, previousPayload, payloadExisted); rollbackErr != nil {
			return errors.WrapResource("rollback", "provider fixture", payloadPath, rollbackErr)
		}
		return errors.WrapIO("rename", metadataPath, err)
	}
	return nil
}

func readOptional(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path) //nolint:gosec // Internal promotion target.
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, errors.WrapIO("read", path, err)
	}
	return data, true, nil
}

func restoreFile(path string, data []byte, existed bool) error {
	if !existed {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return errors.WrapIO("remove", path, err)
		}
		return nil
	}
	temporary, err := writeTemp(path, data)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporary) }()
	if err := os.Rename(temporary, path); err != nil {
		return errors.WrapIO("rename", path, err)
	}
	return nil
}

func writeTemp(target string, data []byte) (string, error) {
	temporary, err := os.CreateTemp(filepath.Dir(target), ".provider-fixture-*.tmp")
	if err != nil {
		return "", errors.WrapIO("create", target, err)
	}
	path := temporary.Name()
	if err := temporary.Chmod(constants.FilePermissions); err != nil {
		_ = temporary.Close()
		_ = os.Remove(path)
		return "", errors.WrapIO("chmod", path, err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		_ = os.Remove(path)
		return "", errors.WrapIO("write", path, err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		_ = os.Remove(path)
		return "", errors.WrapIO("sync", path, err)
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(path)
		return "", errors.WrapIO("close", path, err)
	}
	return path, nil
}
