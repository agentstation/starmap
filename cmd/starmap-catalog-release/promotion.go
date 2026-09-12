package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/bootstrap/manifest"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/errors"
)

type promotionReport struct {
	GenerationID     string `json:"generation_id"`
	SemanticChecksum string `json:"semantic_checksum"`
	PayloadChecksum  string `json:"payload_checksum"`
	ArchiveChecksum  string `json:"archive_checksum"`
	CatalogDirectory string `json:"catalog_directory"`
	ReleaseDirectory string `json:"release_directory"`
}

// verifyPromotionDirectory checks local integrity and exact embedded input.
// The publisher must separately verify provenance and the merged source revision.
func verifyPromotionDirectory(catalogPath, releasePath string) (promotionReport, error) {
	if catalogPath == "" || releasePath == "" {
		return promotionReport{}, &errors.ValidationError{
			Field: "catalog_release.promotion", Message: "catalog and release directories are required",
		}
	}
	release, err := readReleaseDirectory(releasePath)
	if err != nil {
		return promotionReport{}, err
	}
	generation, err := artifact.Open(release.archive, release.statement)
	if err != nil {
		return promotionReport{}, err
	}
	var expected catalogs.BootstrapManifest
	err = workspace.Read(context.Background(), catalogPath, func(workspace.InputExpectation) error {
		builder, err := catalogs.NewFromPath(catalogPath)
		if err != nil {
			return err
		}
		if err := builder.LoadReport().Err(); err != nil {
			return err
		}
		catalog, err := builder.Build()
		if err != nil {
			return err
		}
		expected, _, err = manifest.DeriveCommitted(catalog, generation, nil)
		if err != nil {
			return err
		}
		return verifyPromotionMetadata(catalogPath, catalog, expected)
	})
	if err != nil {
		return promotionReport{}, err
	}
	return promotionReport{
		GenerationID: expected.GenerationID, SemanticChecksum: expected.SemanticChecksum,
		PayloadChecksum: expected.Payload.Checksum, ArchiveChecksum: release.archiveChecksum,
		CatalogDirectory: catalogPath, ReleaseDirectory: release.directory,
	}, nil
}

func verifyPromotionMetadata(path string, catalog *catalogs.Catalog, expected catalogs.BootstrapManifest) error {
	manifestPath := filepath.Join(path, "generation.json")
	data, err := os.ReadFile(manifestPath) //nolint:gosec // Explicit repository input under the workspace read guard.
	if err != nil {
		return errors.WrapIO("read", manifestPath, err)
	}
	actual, err := catalogs.ParseBootstrapManifestJSON(data)
	if err != nil {
		return err
	}
	if actual.ManifestVersion != expected.ManifestVersion || actual.GenerationID != expected.GenerationID ||
		!actual.GeneratedAt.Equal(expected.GeneratedAt) || actual.SchemaVersion != expected.SchemaVersion ||
		actual.SemanticChecksum != expected.SemanticChecksum || actual.Payload != expected.Payload {
		return &errors.ValidationError{
			Field: "catalog_release.promotion_manifest", Message: "embedded manifest does not identify the exact published generation",
		}
	}
	endpoints, err := workspace.EncodeEndpointProjection(catalog, workspace.Identity{
		GenerationID: expected.GenerationID, PayloadChecksum: expected.Payload.Checksum,
	})
	if err != nil {
		return err
	}
	endpointPath := filepath.Join(path, "endpoints.yaml")
	data, err = os.ReadFile(endpointPath) //nolint:gosec // Explicit repository input under the workspace read guard.
	if err != nil {
		return errors.WrapIO("read", endpointPath, err)
	}
	if !bytes.Equal(data, endpoints) {
		return &errors.ValidationError{
			Field:   "catalog_release.promotion_endpoints",
			Message: fmt.Sprintf("endpoint projection does not match published generation %s", expected.GenerationID),
		}
	}
	return nil
}
