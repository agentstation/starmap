// Package bootstrapmanifest derives embedded generation identity from canonical
// catalog bytes without rewriting unchanged generations.
package bootstrapmanifest

import (
	"fmt"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
)

// Report describes whether canonical catalog bytes require a new embedded generation.
type Report struct {
	Changed              bool      `json:"changed"`
	PreviousGenerationID string    `json:"previous_generation_id,omitempty"`
	GenerationID         string    `json:"generation_id"`
	GeneratedAt          time.Time `json:"generated_at"`
	PayloadChecksum      string    `json:"payload_checksum"`
	SemanticChecksum     string    `json:"semantic_checksum"`
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
	semanticChecksum, err := catalogs.CatalogSemanticChecksum(reader)
	if err != nil {
		return catalogs.BootstrapManifest{}, Report{}, err
	}
	if current != nil &&
		current.SchemaVersion == catalogs.CurrentCatalogSchemaVersion &&
		current.SemanticChecksum == semanticChecksum {
		return *current, Report{
			Changed: false, PreviousGenerationID: current.GenerationID,
			GenerationID: current.GenerationID, GeneratedAt: current.GeneratedAt,
			PayloadChecksum:  current.Payload.Checksum,
			SemanticChecksum: current.SemanticChecksum,
			PayloadSizeBytes: current.Payload.SizeBytes,
		}, nil
	}
	if generatedAt.IsZero() {
		return catalogs.BootstrapManifest{}, Report{}, &errors.ValidationError{Field: "bootstrap_manifest.generated_at", Message: "is required for a changed catalog"}
	}
	generatedAt = generatedAt.UTC()
	digest := strings.TrimPrefix(semanticChecksum, "sha256:")
	if len(digest) < 12 {
		return catalogs.BootstrapManifest{}, Report{}, &errors.ValidationError{
			Field: "bootstrap_manifest.semantic_checksum", Value: semanticChecksum,
			Message: "is not a complete SHA-256 digest",
		}
	}
	manifest := catalogs.BootstrapManifest{
		ManifestVersion: catalogs.CurrentBootstrapManifestVersion,
		GenerationID:    fmt.Sprintf("catalog-%s-%s", generatedAt.Format("20060102T150405Z"), digest[:12]),
		GeneratedAt:     generatedAt, SchemaVersion: catalogs.CurrentCatalogSchemaVersion,
		SemanticChecksum: semanticChecksum, Payload: descriptor,
	}
	if err := manifest.Validate(); err != nil {
		return catalogs.BootstrapManifest{}, Report{}, err
	}
	report := Report{
		Changed: true, GenerationID: manifest.GenerationID, GeneratedAt: generatedAt,
		PayloadChecksum: descriptor.Checksum, SemanticChecksum: semanticChecksum,
		PayloadSizeBytes: descriptor.SizeBytes,
	}
	if current != nil {
		report.PreviousGenerationID = current.GenerationID
	}
	return manifest, report, nil
}

// DeriveCommitted binds changed embedded bytes to the identity of the exact
// durable generation that produced them. It never reconstructs source
// observations; release staging reads the same generation from its store.
func DeriveCommitted(
	reader catalogs.Reader,
	generation catalogstore.Generation,
	current *catalogs.BootstrapManifest,
) (catalogs.BootstrapManifest, Report, error) {
	if reader == nil {
		return catalogs.BootstrapManifest{}, Report{}, &errors.ValidationError{
			Field: "bootstrap_manifest.catalog", Message: "is required",
		}
	}
	if err := generation.Validate(); err != nil {
		return catalogs.BootstrapManifest{}, Report{}, errors.WrapResource(
			"validate",
			"committed catalog generation",
			generation.Manifest.GenerationID,
			err,
		)
	}
	payload, err := catalogs.EncodeCatalogPayload(reader)
	if err != nil {
		return catalogs.BootstrapManifest{}, Report{}, err
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)
	semanticChecksum, err := catalogs.CatalogSemanticChecksum(reader)
	if err != nil {
		return catalogs.BootstrapManifest{}, Report{}, err
	}
	if descriptor != generation.Manifest.Payload {
		return catalogs.BootstrapManifest{}, Report{}, &errors.ValidationError{
			Field:   "bootstrap_manifest.committed_payload",
			Value:   descriptor.Checksum,
			Message: "embedded catalog bytes do not match the committed generation",
		}
	}
	if current != nil &&
		current.SchemaVersion == catalogs.CurrentCatalogSchemaVersion &&
		current.SemanticChecksum == semanticChecksum {
		return *current, unchangedReport(*current), nil
	}

	manifest := catalogs.BootstrapManifest{
		ManifestVersion:  catalogs.CurrentBootstrapManifestVersion,
		GenerationID:     generation.Manifest.GenerationID,
		GeneratedAt:      generation.Manifest.GeneratedAt,
		SchemaVersion:    generation.Manifest.SchemaVersion,
		SemanticChecksum: semanticChecksum,
		Payload:          descriptor,
	}
	if err := manifest.Validate(); err != nil {
		return catalogs.BootstrapManifest{}, Report{}, err
	}
	report := Report{
		Changed:          true,
		GenerationID:     manifest.GenerationID,
		GeneratedAt:      manifest.GeneratedAt,
		PayloadChecksum:  descriptor.Checksum,
		SemanticChecksum: semanticChecksum,
		PayloadSizeBytes: descriptor.SizeBytes,
	}
	if current != nil {
		report.PreviousGenerationID = current.GenerationID
	}
	return manifest, report, nil
}

func unchangedReport(current catalogs.BootstrapManifest) Report {
	return Report{
		Changed:              false,
		PreviousGenerationID: current.GenerationID,
		GenerationID:         current.GenerationID,
		GeneratedAt:          current.GeneratedAt,
		PayloadChecksum:      current.Payload.Checksum,
		SemanticChecksum:     current.SemanticChecksum,
		PayloadSizeBytes:     current.Payload.SizeBytes,
	}
}
