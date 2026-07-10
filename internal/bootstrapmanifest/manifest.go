// Package bootstrapmanifest derives embedded generation identity from canonical
// catalog bytes without rewriting unchanged generations.
package bootstrapmanifest

import (
	"fmt"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Report describes whether canonical catalog bytes require a new embedded generation.
type Report struct {
	Changed              bool      `json:"changed"`
	PreviousGenerationID string    `json:"previous_generation_id,omitempty"`
	GenerationID         string    `json:"generation_id"`
	GeneratedAt          time.Time `json:"generated_at"`
	PayloadChecksum      string    `json:"payload_checksum"`
	PayloadSizeBytes     int64     `json:"payload_size_bytes"`
}

// Derive compares canonical reader bytes with the current manifest. Unchanged
// input returns the exact current identity; changed input gets a new logical ID.
func Derive(reader catalogs.Reader, current *catalogs.BootstrapManifest, generatedAt time.Time) (catalogs.BootstrapManifest, Report, error) {
	if reader == nil {
		return catalogs.BootstrapManifest{}, Report{}, &errors.ValidationError{Field: "bootstrap_manifest.catalog", Message: "is required"}
	}
	payload, err := catalogs.EncodeCatalogPayload(reader)
	if err != nil {
		return catalogs.BootstrapManifest{}, Report{}, err
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)
	if current != nil && current.SchemaVersion == catalogs.CurrentCatalogSchemaVersion && current.Payload == descriptor {
		return *current, Report{
			Changed: false, PreviousGenerationID: current.GenerationID,
			GenerationID: current.GenerationID, GeneratedAt: current.GeneratedAt,
			PayloadChecksum: descriptor.Checksum, PayloadSizeBytes: descriptor.SizeBytes,
		}, nil
	}
	if generatedAt.IsZero() {
		return catalogs.BootstrapManifest{}, Report{}, &errors.ValidationError{Field: "bootstrap_manifest.generated_at", Message: "is required for a changed catalog"}
	}
	generatedAt = generatedAt.UTC()
	digest := strings.TrimPrefix(descriptor.Checksum, "sha256:")
	if len(digest) < 12 {
		return catalogs.BootstrapManifest{}, Report{}, &errors.ValidationError{Field: "bootstrap_manifest.payload_checksum", Value: descriptor.Checksum, Message: "is not a complete SHA-256 digest"}
	}
	manifest := catalogs.BootstrapManifest{
		ManifestVersion: catalogs.CurrentBootstrapManifestVersion,
		GenerationID:    fmt.Sprintf("catalog-%s-%s", generatedAt.Format("20060102T150405Z"), digest[:12]),
		GeneratedAt:     generatedAt, SchemaVersion: catalogs.CurrentCatalogSchemaVersion, Payload: descriptor,
	}
	if err := manifest.Validate(); err != nil {
		return catalogs.BootstrapManifest{}, Report{}, err
	}
	report := Report{
		Changed: true, GenerationID: manifest.GenerationID, GeneratedAt: generatedAt,
		PayloadChecksum: descriptor.Checksum, PayloadSizeBytes: descriptor.SizeBytes,
	}
	if current != nil {
		report.PreviousGenerationID = current.GenerationID
	}
	return manifest, report, nil
}
