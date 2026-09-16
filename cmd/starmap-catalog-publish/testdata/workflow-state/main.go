// Command workflow-state emits a small publisher checkpoint for workflow tests.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agentstation/starmap/internal/catalog/publication"
	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(catalogs.Author{ID: "fixture", Name: "Fixture"}); err != nil {
		return err
	}
	if err := builder.SetAuthorModel("fixture", catalogs.Model{ID: "one", Name: "One", Authors: []catalogs.Author{{ID: "fixture", Name: "Fixture"}}}); err != nil {
		return err
	}
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider", Models: map[string]*catalogs.Model{
		"exact/ID": {ID: "exact/ID", Name: "One", ModelRef: "fixture/one"},
	}}); err != nil {
		return err
	}
	catalog, err := builder.Build()
	if err != nil {
		return err
	}
	data, err := os.ReadFile("pkg/catalogs/testdata/generation/manifest.json")
	if err != nil {
		return err
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(data)
	if err != nil {
		return err
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return err
	}
	manifest.GenerationID = "workflow-fixture"
	manifest.SchemaVersion = catalogs.CatalogPayloadSchemaVersion(catalog)
	manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{MinSchemaVersion: manifest.SchemaVersion, MaxSchemaVersion: manifest.SchemaVersion}
	root, err := os.MkdirTemp("", "starmap-workflow-catalog-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(root) }()
	catalogPath := filepath.Join(root, "catalog")
	if _, err := workspace.Project(context.Background(), catalogPath, catalog, workspace.Identity{
		GenerationID: manifest.GenerationID, PayloadChecksum: catalogs.DescribeCatalogPayload(payload).Checksum,
	}); err != nil {
		return err
	}
	projected, err := catalogs.NewFromPath(catalogPath)
	if err != nil {
		return err
	}
	if err := projected.LoadReport().Err(); err != nil {
		return err
	}
	catalog, err = projected.Build()
	if err != nil {
		return err
	}
	payload, err = catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return err
	}
	manifest.Payload = catalogs.DescribeCatalogPayload(payload)
	state, err := publication.NewState(catalogs.Generation{Manifest: manifest, Payload: payload}, "starmap-public")
	if err != nil {
		return err
	}
	record, err := publication.EncodeState(state)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(record)
}
