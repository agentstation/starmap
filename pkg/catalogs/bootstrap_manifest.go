package catalogs

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

// CurrentBootstrapManifestVersion is the embedded-bootstrap metadata format.
const CurrentBootstrapManifestVersion uint64 = 1

// BootstrapManifest binds the offline embedded catalog to exact canonical
// catalog bytes and a generation time.
type BootstrapManifest struct {
	ManifestVersion uint64            `json:"manifest_version" yaml:"manifest_version"`
	GenerationID    string            `json:"generation_id" yaml:"generation_id"`
	GeneratedAt     time.Time         `json:"generated_at" yaml:"generated_at"`
	SchemaVersion   uint64            `json:"schema_version" yaml:"schema_version"`
	Payload         PayloadDescriptor `json:"payload" yaml:"payload"`
}

// Validate checks the embedded-bootstrap metadata contract.
func (m BootstrapManifest) Validate() error {
	if m.ManifestVersion != CurrentBootstrapManifestVersion {
		return bootstrapValidation("manifest_version", m.ManifestVersion, validationMessageIsNotSupported)
	}
	if strings.TrimSpace(m.GenerationID) == "" {
		return bootstrapValidation("generation_id", m.GenerationID, validationMessageIsRequired)
	}
	if m.GeneratedAt.IsZero() {
		return bootstrapValidation("generated_at", m.GeneratedAt, validationMessageIsRequired)
	}
	_, offset := m.GeneratedAt.Zone()
	if offset != 0 {
		return bootstrapValidation("generated_at", m.GeneratedAt, "must be UTC")
	}
	if m.SchemaVersion != CurrentCatalogSchemaVersion {
		return bootstrapValidation("schema_version", m.SchemaVersion, "must match the exact current catalog schema")
	}
	if m.Payload.MediaType != CatalogPayloadMediaType || m.Payload.SizeBytes <= 0 ||
		!strings.HasPrefix(m.Payload.Checksum, checksumAlgorithmPrefix) || len(m.Payload.Checksum) != len(checksumAlgorithmPrefix)+64 {
		return bootstrapValidation("payload", m.Payload, "must contain a canonical SHA-256 catalog descriptor")
	}
	return nil
}

// ParseBootstrapManifestJSON strictly parses embedded-bootstrap metadata.
func ParseBootstrapManifestJSON(data []byte) (BootstrapManifest, error) {
	if err := sourcepayload.ValidateExactJSON(data); err != nil {
		return BootstrapManifest{}, err
	}
	var required map[string]json.RawMessage
	if err := json.Unmarshal(data, &required); err != nil {
		return BootstrapManifest{}, &errors.ParseError{Format: string(ModelResponseFormatJSON), File: catalogBootstrapManifestFile, Message: err.Error(), Err: err}
	}
	for _, field := range []string{"manifest_version", "generation_id", "generated_at", "schema_version", "payload"} {
		if _, exists := required[field]; !exists {
			return BootstrapManifest{}, bootstrapValidation(field, nil, validationMessageIsRequired)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest BootstrapManifest
	if err := decoder.Decode(&manifest); err != nil {
		return BootstrapManifest{}, &errors.ParseError{Format: string(ModelResponseFormatJSON), File: catalogBootstrapManifestFile, Message: err.Error(), Err: err}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return BootstrapManifest{}, &errors.ParseError{Format: string(ModelResponseFormatJSON), File: catalogBootstrapManifestFile, Message: "invalid trailing JSON", Err: err}
	}
	if err := manifest.Validate(); err != nil {
		return BootstrapManifest{}, err
	}
	return manifest, nil
}

func bootstrapValidation(field string, value any, message string) error {
	return &errors.ValidationError{Field: "bootstrap_manifest." + field, Value: value, Message: message}
}
