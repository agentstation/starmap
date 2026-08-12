// Package providerfixture owns governed provider-response fixtures for tests.
package providerfixture

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	fixtureMetadataVersion = 1
	fixturePayloadName     = "models_list.json"
	fixtureMetadataName    = "models_list.metadata.json"
	fixtureFutureTolerance = 5 * time.Minute
)

var updateRequested = flag.Bool("update", false, "refresh the selected provider fixture")

// Fixture identifies one provider capture and its adjacent metadata.
type Fixture struct {
	Provider     string
	PayloadPath  string
	MetadataPath string
}

// FixtureMetadata binds a provider fixture to its source revision and
// freshness policy.
type FixtureMetadata struct {
	Version        uint64                          `json:"version"`
	Provider       string                          `json:"provider"`
	FetchedAt      time.Time                       `json:"fetched_at"`
	SourceRevision catalogmeta.ObservationRevision `json:"source_revision"`
	Payload        FixturePayload                  `json:"payload"`
	MaxAge         string                          `json:"max_age"`
}

// FixturePayload identifies the exact fixture bytes governed by metadata.
type FixturePayload struct {
	Path     string `json:"path"`
	Checksum string `json:"checksum"`
}

// UpdateRequested reports whether the caller explicitly selected fixture
// refresh with the test binary's -update flag.
func UpdateRequested() bool { return *updateRequested }

// Discover returns every complete provider fixture below root in provider-ID
// order.
func Discover(root string) ([]Fixture, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, errors.WrapIO("read", root, err)
	}
	fixtures := make([]Fixture, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			return nil, &errors.ValidationError{
				Field: "fixture.entry", Value: entry.Name(), Message: "must be a provider directory",
			}
		}
		fixture, err := Find(root, entry.Name())
		if err != nil {
			return nil, err
		}
		fixtures = append(fixtures, fixture)
	}
	if len(fixtures) == 0 {
		return nil, &errors.ValidationError{
			Field: "fixture.root", Value: root, Message: "contains no provider fixtures",
		}
	}
	return fixtures, nil
}

// Find resolves one explicit provider fixture below root.
func Find(root, provider string) (Fixture, error) {
	if !validProviderDirectory(provider) {
		return Fixture{}, &errors.ValidationError{
			Field: "fixture.provider", Value: provider,
			Message: "must use lowercase letters, digits, or hyphens",
		}
	}
	directory := filepath.Join(root, provider)
	fixture := Fixture{
		Provider:     provider,
		PayloadPath:  filepath.Join(directory, fixturePayloadName),
		MetadataPath: filepath.Join(directory, fixtureMetadataName),
	}
	if err := requireRegularFile(fixture.PayloadPath); err != nil {
		return Fixture{}, err
	}
	if err := requireRegularFile(fixture.MetadataPath); err != nil {
		return Fixture{}, err
	}
	return fixture, nil
}

// Read returns caller-owned fixture payload bytes.
func (f Fixture) Read() ([]byte, error) {
	data, err := os.ReadFile(f.PayloadPath) //nolint:gosec // The caller selects a governed test fixture.
	if err != nil {
		return nil, errors.WrapIO("read", f.PayloadPath, err)
	}
	return data, nil
}

// Decode unmarshals the fixture payload into destination.
func (f Fixture) Decode(destination any) error {
	data, err := f.Read()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return errors.WrapParse("json", f.PayloadPath, err)
	}
	return nil
}

// Verify validates fixture identity, bytes, source revision, and freshness.
func (f Fixture) Verify(now time.Time) error {
	if now.IsZero() {
		return &errors.ValidationError{Field: "fixture.now", Message: "must be set"}
	}
	payload, err := f.Read()
	if err != nil {
		return err
	}
	metadata, err := f.readMetadata()
	if err != nil {
		return err
	}
	maxAge, err := f.validateMetadataIdentity(metadata)
	if err != nil {
		return err
	}
	checksum := fixtureChecksum(payload)
	if metadata.Payload.Checksum != checksum {
		return &errors.ValidationError{
			Field: "fixture.payload.checksum", Value: metadata.Payload.Checksum,
			Message: "does not match fixture bytes",
		}
	}
	if metadata.SourceRevision.Kind != catalogmeta.ObservationRevisionKindContentDigest ||
		metadata.SourceRevision.Value != checksum {
		return &errors.ValidationError{
			Field: "fixture.source_revision", Value: metadata.SourceRevision,
			Message: "must identify the fixture content digest",
		}
	}
	if metadata.FetchedAt.IsZero() || metadata.FetchedAt.After(now.Add(fixtureFutureTolerance)) {
		return &errors.ValidationError{
			Field: "fixture.fetched_at", Value: metadata.FetchedAt,
			Message: "must be a non-future capture time",
		}
	}
	if now.Sub(metadata.FetchedAt) > maxAge {
		return &errors.ValidationError{
			Field: "fixture.fetched_at", Value: metadata.FetchedAt,
			Message: "provider fixture is stale",
		}
	}
	return nil
}

// Capture replaces the payload and metadata after an explicit live refresh.
// It preserves the fixture's reviewed maximum-age policy.
func (f Fixture) Capture(payload []byte, capturedAt time.Time) error {
	if capturedAt.IsZero() {
		return &errors.ValidationError{Field: "fixture.fetched_at", Message: "must be set"}
	}
	metadata, err := f.readMetadata()
	if err != nil {
		return err
	}
	if _, err := f.validateMetadataIdentity(metadata); err != nil {
		return err
	}
	canonical, err := canonicalJSON(payload)
	if err != nil {
		return errors.WrapParse("json", f.PayloadPath, err)
	}
	checksum := fixtureChecksum(canonical)
	metadata.FetchedAt = capturedAt.UTC()
	metadata.SourceRevision = catalogmeta.ObservationRevision{
		Kind: catalogmeta.ObservationRevisionKindContentDigest, Value: checksum,
	}
	metadata.Payload.Checksum = checksum
	metadataData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return errors.WrapParse("json", f.MetadataPath, err)
	}
	metadataData = append(metadataData, '\n')

	payloadStage, err := stageFile(f.PayloadPath, canonical)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(payloadStage) }()
	metadataStage, err := stageFile(f.MetadataPath, metadataData)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(metadataStage) }()
	if err := os.Rename(payloadStage, f.PayloadPath); err != nil {
		return errors.WrapIO("rename", f.PayloadPath, err)
	}
	if err := os.Rename(metadataStage, f.MetadataPath); err != nil {
		return errors.WrapIO("rename", f.MetadataPath, err)
	}
	return f.Verify(capturedAt.UTC())
}

func (f Fixture) readMetadata() (FixtureMetadata, error) {
	data, err := os.ReadFile(f.MetadataPath) //nolint:gosec // The caller selects a governed test fixture.
	if err != nil {
		return FixtureMetadata{}, errors.WrapIO("read", f.MetadataPath, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var metadata FixtureMetadata
	if err := decoder.Decode(&metadata); err != nil {
		return FixtureMetadata{}, errors.WrapParse("json", f.MetadataPath, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = &errors.ValidationError{Field: "fixture.metadata", Message: "contains multiple JSON values"}
		}
		return FixtureMetadata{}, errors.WrapParse("json", f.MetadataPath, err)
	}
	return metadata, nil
}

func (f Fixture) validateMetadataIdentity(metadata FixtureMetadata) (time.Duration, error) {
	if metadata.Version != fixtureMetadataVersion {
		return 0, &errors.ValidationError{
			Field: "fixture.metadata.version", Value: metadata.Version, Message: "must be version 1",
		}
	}
	if metadata.Provider != f.Provider {
		return 0, &errors.ValidationError{
			Field: "fixture.provider", Value: metadata.Provider,
			Message: "does not match the selected provider directory",
		}
	}
	if metadata.Payload.Path != fixturePayloadName {
		return 0, &errors.ValidationError{
			Field: "fixture.payload.path", Value: metadata.Payload.Path,
			Message: "must name the adjacent fixture",
		}
	}
	maxAge, err := time.ParseDuration(metadata.MaxAge)
	if err != nil || maxAge <= 0 {
		return 0, &errors.ValidationError{
			Field: "fixture.max_age", Value: metadata.MaxAge,
			Message: "must be a positive Go duration",
		}
	}
	return maxAge, nil
}

func validProviderDirectory(provider string) bool {
	if provider == "" || strings.TrimSpace(provider) != provider {
		return false
	}
	for _, character := range provider {
		if character == '-' || character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return true
}

func requireRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return errors.WrapIO("stat", path, err)
	}
	if !info.Mode().IsRegular() {
		return &errors.ValidationError{
			Field: "fixture.path", Value: path, Message: "must be a regular file",
		}
	}
	return nil
}

func canonicalJSON(data []byte) ([]byte, error) {
	var output bytes.Buffer
	if err := json.Indent(&output, bytes.TrimSpace(data), "", "  "); err != nil {
		return nil, err
	}
	output.WriteByte('\n')
	return output.Bytes(), nil
}

func stageFile(target string, data []byte) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(target), ".provider-fixture-*.tmp")
	if err != nil {
		return "", errors.WrapIO("create", target, err)
	}
	stagePath := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(stagePath)
	}
	if err := file.Chmod(constants.FilePermissions); err != nil {
		cleanup()
		return "", errors.WrapIO("chmod", target, err)
	}
	if _, err := file.Write(data); err != nil {
		cleanup()
		return "", errors.WrapIO("write", target, err)
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return "", errors.WrapIO("sync", target, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(stagePath)
		return "", errors.WrapIO("close", target, err)
	}
	return stagePath, nil
}

func fixtureChecksum(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}
