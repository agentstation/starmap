package catalogs

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// CurrentBootstrapManifestVersion is the embedded-bootstrap metadata format.
const CurrentBootstrapManifestVersion uint64 = 2

// BootstrapManifest binds the offline embedded catalog to exact canonical
// catalog bytes and a generation time.
type BootstrapManifest struct {
	ManifestVersion  uint64            `json:"manifest_version" yaml:"manifest_version"`
	GenerationID     string            `json:"generation_id" yaml:"generation_id"`
	GeneratedAt      time.Time         `json:"generated_at" yaml:"generated_at"`
	SchemaVersion    uint64            `json:"schema_version" yaml:"schema_version"`
	SemanticChecksum string            `json:"semantic_checksum" yaml:"semantic_checksum"`
	Payload          PayloadDescriptor `json:"payload" yaml:"payload"`
}

// ValidateEnvelope checks schema-independent embedded-bootstrap metadata.
func (m BootstrapManifest) ValidateEnvelope() error {
	if m.ManifestVersion != CurrentBootstrapManifestVersion {
		return bootstrapValidation("manifest_version", m.ManifestVersion, "is not supported")
	}
	if strings.TrimSpace(m.GenerationID) == "" {
		return bootstrapValidation("generation_id", m.GenerationID, "is required")
	}
	if m.GeneratedAt.IsZero() {
		return bootstrapValidation("generated_at", m.GeneratedAt, "is required")
	}
	_, offset := m.GeneratedAt.Zone()
	if offset != 0 {
		return bootstrapValidation("generated_at", m.GeneratedAt, "must be UTC")
	}
	if m.SchemaVersion == 0 {
		return bootstrapValidation("schema_version", m.SchemaVersion, "must be greater than zero")
	}
	if !strings.HasPrefix(m.SemanticChecksum, checksumAlgorithmPrefix) ||
		len(m.SemanticChecksum) != len(checksumAlgorithmPrefix)+64 {
		return bootstrapValidation(
			"semantic_checksum",
			m.SemanticChecksum,
			"must contain a canonical SHA-256 catalog checksum",
		)
	}
	if m.Payload.MediaType != CatalogPayloadMediaType || m.Payload.SizeBytes <= 0 ||
		!strings.HasPrefix(m.Payload.Checksum, checksumAlgorithmPrefix) || len(m.Payload.Checksum) != len(checksumAlgorithmPrefix)+64 {
		return bootstrapValidation("payload", m.Payload, "must contain a canonical SHA-256 catalog descriptor")
	}
	return nil
}

// Validate checks the embedded-bootstrap metadata contract and requires the
// current catalog schema.
func (m BootstrapManifest) Validate() error {
	if err := m.ValidateEnvelope(); err != nil {
		return err
	}
	if m.SchemaVersion != CurrentCatalogSchemaVersion {
		return bootstrapValidation("schema_version", m.SchemaVersion, "does not match the current catalog schema")
	}
	return nil
}

// ParseBootstrapManifestJSON strictly parses embedded-bootstrap metadata.
func ParseBootstrapManifestJSON(data []byte) (BootstrapManifest, error) {
	return parseBootstrapManifestJSON(data, BootstrapManifest.Validate)
}

// ParseBootstrapManifestEnvelopeJSON strictly parses bootstrap metadata
// without requiring the current catalog schema. Catalog refresh tooling uses
// it to replace a valid manifest from the previous schema.
func ParseBootstrapManifestEnvelopeJSON(data []byte) (BootstrapManifest, error) {
	return parseBootstrapManifestJSON(data, BootstrapManifest.ValidateEnvelope)
}

func parseBootstrapManifestJSON(
	data []byte,
	validate func(BootstrapManifest) error,
) (BootstrapManifest, error) {
	var required map[string]json.RawMessage
	if err := json.Unmarshal(data, &required); err != nil {
		return BootstrapManifest{}, &errors.ParseError{Format: "json", File: "embedded bootstrap manifest", Message: err.Error(), Err: err}
	}
	for _, field := range []string{
		"manifest_version", "generation_id", "generated_at", "schema_version",
		"semantic_checksum", "payload",
	} {
		if _, exists := required[field]; !exists {
			return BootstrapManifest{}, bootstrapValidation(field, nil, "is required")
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest BootstrapManifest
	if err := decoder.Decode(&manifest); err != nil {
		return BootstrapManifest{}, &errors.ParseError{Format: "json", File: "embedded bootstrap manifest", Message: err.Error(), Err: err}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return BootstrapManifest{}, &errors.ParseError{Format: "json", File: "embedded bootstrap manifest", Message: "invalid trailing JSON", Err: err}
	}
	if err := validate(manifest); err != nil {
		return BootstrapManifest{}, err
	}
	return manifest, nil
}

func bootstrapValidation(field string, value any, message string) error {
	return &errors.ValidationError{Field: "bootstrap_manifest." + field, Value: value, Message: message}
}
