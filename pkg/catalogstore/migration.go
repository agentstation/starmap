package catalogstore

import (
	"context"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// LegacyMigrationOptions supplies deterministic identity and time metadata that
// did not exist in the legacy multi-file YAML catalog format.
type LegacyMigrationOptions struct {
	GenerationID     string
	SyncRunID        string
	ObservationID    string
	GeneratedAt      time.Time
	ValidatorVersion string
}

// MigrateLegacyDirectory reads the pre-generation multi-file YAML format and
// converts it into a validated schema-v1 generation without mutating the source.
func MigrateLegacyDirectory(ctx context.Context, path string, options LegacyMigrationOptions) (Generation, error) {
	if err := ctx.Err(); err != nil {
		return Generation{}, err
	}
	for _, required := range []struct {
		field string
		value string
	}{
		{field: "generation_id", value: options.GenerationID},
		{field: "sync_run_id", value: options.SyncRunID},
		{field: "source_observations[0].observation_id", value: options.ObservationID},
		{field: "validation.validator_version", value: options.ValidatorVersion},
	} {
		if strings.TrimSpace(required.value) == "" {
			return Generation{}, &errors.ValidationError{Field: required.field, Message: "is required"}
		}
	}
	if options.GeneratedAt.IsZero() {
		return Generation{}, &errors.ValidationError{Field: "generated_at", Message: "is required"}
	}

	builder, err := catalogs.NewFromPath(path)
	if err != nil {
		return Generation{}, errors.WrapResource("migrate", "legacy catalog", path, err)
	}
	catalog, err := builder.Build()
	if err != nil {
		return Generation{}, errors.WrapResource("publish", "legacy catalog", path, err)
	}
	payload, err := EncodeCatalogPayload(catalog)
	if err != nil {
		return Generation{}, errors.WrapResource("encode", "legacy catalog", path, err)
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)
	generation := Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    options.GenerationID,
			GeneratedAt:     options.GeneratedAt,
			Payload:         descriptor,
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: options.ValidatorVersion,
				ValidatedAt:      options.GeneratedAt,
				Status:           catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{
					{Name: "legacy_yaml_parse", Status: catalogs.GenerationValidationCheckPassed},
					{Name: "catalog_publication", Status: catalogs.GenerationValidationCheckPassed},
					{Name: "schema_v1_encode", Status: catalogs.GenerationValidationCheckPassed},
				},
			},
			SyncRunID: options.SyncRunID,
			SourceObservations: []catalogs.SourceObservationLink{
				{
					Source:        catalogmeta.LocalCatalogID,
					ObservationID: options.ObservationID,
					ObservedAt:    options.GeneratedAt,
					Revision: catalogmeta.ObservationRevision{
						Kind:  catalogmeta.ObservationRevisionKindContentDigest,
						Value: descriptor.Checksum,
					},
					Completeness:     catalogmeta.ObservationCompletenessComplete,
					Status:           catalogmeta.ObservationStatusSucceeded,
					EvidenceChecksum: descriptor.Checksum,
				},
			},
			Completeness: catalogs.GenerationCompletenessComplete,
			ConsumerCompatibility: catalogs.ConsumerCompatibility{
				MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
				MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			},
		},
		Payload: payload,
	}
	if err := generation.Validate(); err != nil {
		return Generation{}, err
	}
	return generation, nil
}
