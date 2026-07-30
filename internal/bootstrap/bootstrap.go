// Package bootstrap verifies the catalog generation embedded in the binary.
package bootstrap

import (
	"sync"

	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/errors"
)

const manifestPath = "catalog/generation.json"

var (
	embeddedOnce     sync.Once
	embeddedCatalog  *catalogs.Catalog
	embeddedManifest catalogs.BootstrapManifest
	embeddedErr      error
)

// Embedded returns the process-wide verified immutable bootstrap catalog and
// its digest-bound manifest. The catalog is safe to retain across goroutines.
// Verification and YAML decoding run once because the embedded filesystem
// cannot change during a process lifetime.
func Embedded() (*catalogs.Catalog, catalogs.BootstrapManifest, error) {
	embeddedOnce.Do(func() {
		builder, err := catalogs.NewEmbedded()
		if err != nil {
			embeddedErr = errors.WrapResource("load", "embedded bootstrap catalog", "", err)
			return
		}
		embeddedCatalog, err = builder.Build()
		if err != nil {
			embeddedErr = errors.WrapResource("publish", "embedded bootstrap catalog", "", err)
			return
		}
		embeddedManifest, embeddedErr = Load(embeddedCatalog)
	})
	return embeddedCatalog, embeddedManifest, embeddedErr
}

// Load parses embedded generation metadata and verifies it against the exact
// canonical bytes produced by the embedded catalog.
func Load(reader catalogs.Reader) (catalogs.BootstrapManifest, error) {
	data, err := embedded.FS.ReadFile(manifestPath)
	if err != nil {
		return catalogs.BootstrapManifest{}, errors.WrapResource("read", "embedded bootstrap manifest", manifestPath, err)
	}
	manifest, err := catalogs.ParseBootstrapManifestJSON(data)
	if err != nil {
		return catalogs.BootstrapManifest{}, err
	}
	payload, err := catalogs.EncodeCatalogPayload(reader)
	if err != nil {
		return catalogs.BootstrapManifest{}, errors.WrapResource("encode", "embedded bootstrap catalog", manifest.GenerationID, err)
	}
	if err := manifest.Payload.Verify(payload); err != nil {
		return catalogs.BootstrapManifest{}, errors.WrapResource("verify", "embedded bootstrap catalog", manifest.GenerationID, err)
	}
	semanticChecksum, err := catalogs.CatalogSemanticChecksum(reader)
	if err != nil {
		return catalogs.BootstrapManifest{}, errors.WrapResource(
			"encode",
			"embedded bootstrap catalog semantics",
			manifest.GenerationID,
			err,
		)
	}
	if semanticChecksum != manifest.SemanticChecksum {
		return catalogs.BootstrapManifest{}, &errors.ValidationError{
			Field:   "bootstrap_manifest.semantic_checksum",
			Value:   semanticChecksum,
			Message: "does not match the embedded catalog facts",
		}
	}
	return manifest, nil
}

// Generation returns the embedded bootstrap as a complete validated immutable
// generation suitable for deterministic release artifact publication.
func Generation() (catalogstore.Generation, error) {
	catalog, bootstrapManifest, err := Embedded()
	if err != nil {
		return catalogstore.Generation{}, err
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return catalogstore.Generation{}, err
	}
	manifest := catalogs.GenerationManifest{
		ManifestVersion: catalogs.CurrentGenerationManifestVersion,
		SchemaVersion:   bootstrapManifest.SchemaVersion,
		GenerationID:    bootstrapManifest.GenerationID,
		GeneratedAt:     bootstrapManifest.GeneratedAt,
		Payload:         bootstrapManifest.Payload,
		Validation: catalogs.GenerationValidationReport{
			ValidatorVersion: "embedded-bootstrap/v1", ValidatedAt: bootstrapManifest.GeneratedAt,
			Status: catalogs.GenerationValidationPassed,
			Checks: []catalogs.GenerationValidationCheck{
				{Name: "canonical_payload", Status: catalogs.GenerationValidationCheckPassed},
				{Name: "embedded_manifest", Status: catalogs.GenerationValidationCheckPassed},
			},
		},
		SyncRunID: "embedded-bootstrap-build",
		SourceObservations: []catalogs.SourceObservationLink{{
			Source:        catalogmeta.EmbeddedCatalogID,
			ObservationID: "embedded-bootstrap:" + bootstrapManifest.GenerationID,
			ObservedAt:    bootstrapManifest.GeneratedAt,
			Revision: catalogmeta.ObservationRevision{
				Kind:  catalogmeta.ObservationRevisionKindContentDigest,
				Value: bootstrapManifest.Payload.Checksum,
			},
			Completeness:     catalogmeta.ObservationCompletenessComplete,
			Status:           catalogmeta.ObservationStatusSucceeded,
			EvidenceChecksum: bootstrapManifest.Payload.Checksum,
		}},
		Completeness: catalogs.GenerationCompletenessComplete,
		ConsumerCompatibility: catalogs.ConsumerCompatibility{
			MinSchemaVersion: bootstrapManifest.SchemaVersion,
			MaxSchemaVersion: bootstrapManifest.SchemaVersion,
		},
	}
	generation := catalogstore.Generation{Manifest: manifest, Payload: payload}
	if err := generation.Validate(); err != nil {
		return catalogstore.Generation{}, err
	}
	return generation, nil
}
