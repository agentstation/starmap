package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
)

func TestArtifactReleaseCommandVerifiesExactPromotion(t *testing.T) {
	catalogPath, releasePath, generation := promotionFixture(t)
	var output bytes.Buffer
	args := []string{"--verify-promotion-dir", catalogPath, "--promotion-release-dir", releasePath}
	if err := run(args, &output); err != nil {
		t.Fatalf("verify exact promotion: %v", err)
	}
	var report map[string]string
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report["generation_id"] != generation.Manifest.GenerationID || report["payload_checksum"] != generation.Manifest.Payload.Checksum {
		t.Fatalf("promotion report does not identify the exact release: %v", report)
	}
	var retry bytes.Buffer
	if err := run(args, &retry); err != nil {
		t.Fatalf("verify promotion again: %v", err)
	}
	if !bytes.Equal(output.Bytes(), retry.Bytes()) {
		t.Fatal("unchanged promotion produced a different report")
	}
}

func TestArtifactReleaseCommandRejectsPromotionMismatch(t *testing.T) {
	for _, kind := range []string{"generation", "timestamp", "semantic", "payload", "schema", "manifest_missing", "facts", "endpoints", "endpoints_missing", "release"} {
		t.Run(kind, func(t *testing.T) {
			catalogPath, releasePath, _ := promotionFixture(t)
			manifestPath := filepath.Join(catalogPath, "generation.json")
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			var bootstrap catalogs.BootstrapManifest
			if err := json.Unmarshal(data, &bootstrap); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "generation":
				bootstrap.GenerationID = "different-generation"
			case "timestamp":
				bootstrap.GeneratedAt = bootstrap.GeneratedAt.Add(time.Second)
			case "semantic":
				bootstrap.SemanticChecksum = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			case "payload":
				bootstrap.Payload.SizeBytes++
			case "schema":
				bootstrap.SchemaVersion++
			case "manifest_missing":
				if err := os.Remove(manifestPath); err != nil {
					t.Fatal(err)
				}
			case "facts":
				path := filepath.Join(catalogPath, "authors", "fixture", "models", "one.yaml")
				before, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				after := bytes.Replace(before, []byte("name: One"), []byte("name: Changed"), 1)
				if bytes.Equal(before, after) {
					t.Fatal("fixture mutation did not change a model fact")
				}
				writePromotionFile(t, path, after)
			case "endpoints":
				writePromotionFile(t, filepath.Join(catalogPath, "endpoints.yaml"), []byte("schema_version: 2\nmodels: []\n"))
			case "endpoints_missing":
				if err := os.Remove(filepath.Join(catalogPath, "endpoints.yaml")); err != nil {
					t.Fatal(err)
				}
			case "release":
				writePromotionFile(t, filepath.Join(releasePath, artifact.Filename), []byte("changed archive"))
			}
			if kind == "generation" || kind == "timestamp" || kind == "semantic" || kind == "payload" || kind == "schema" {
				data, err = json.Marshal(bootstrap)
				if err != nil {
					t.Fatal(err)
				}
				writePromotionFile(t, manifestPath, data)
			}
			var output bytes.Buffer
			if err := run([]string{"--verify-promotion-dir", catalogPath, "--promotion-release-dir", releasePath}, &output); err == nil {
				t.Fatal("promotion accepted changed or missing input")
			}
			if output.Len() != 0 {
				t.Fatal("rejected promotion emitted a success report")
			}
		})
	}
}

func TestArtifactReleaseCommandRequiresPromotionPair(t *testing.T) {
	catalogPath, releasePath, _ := promotionFixture(t)
	for _, args := range [][]string{
		{"--stage-promotion-dir", catalogPath},
		{"--stage-promotion-dir", catalogPath, "--promotion-release-dir", releasePath, "--verify-promotion-dir", catalogPath},
		{"--stage-promotion-dir", catalogPath, "--promotion-release-dir", releasePath, "--output-dir", t.TempDir()},
		{"--stage-promotion-dir", catalogPath, "--promotion-release-dir", releasePath, "--generation-store", t.TempDir()},
		{"--verify-promotion-dir", catalogPath},
		{"--promotion-release-dir", releasePath},
		{"--verify-dir", releasePath, "--promotion-release-dir", releasePath},
		{"--verify-promotion-dir", catalogPath, "--promotion-release-dir", releasePath, "--output-dir", t.TempDir()},
		{"--verify-promotion-dir", catalogPath, "--promotion-release-dir", releasePath, "--inspect-dir", releasePath},
	} {
		if err := run(args, io.Discard); err == nil {
			t.Fatalf("accepted incompatible or incomplete promotion flags: %v", args)
		}
	}
}

func promotionFixture(t *testing.T) (string, string, catalogs.Generation) {
	t.Helper()
	_, generation := releaseFixtureStore(t)
	builder := catalogs.NewEmpty()
	author := catalogs.Author{ID: "fixture", Name: "Fixture"}
	if err := builder.SetAuthor(author); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetAuthorModel("fixture", catalogs.Model{ID: "one", Name: "One", Authors: []catalogs.Author{author}}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider", Models: map[string]*catalogs.Model{
		"exact/ID": {ID: "exact/ID", Name: "One", ModelRef: "fixture/one"},
	}}); err != nil {
		t.Fatal(err)
	}
	observation := generation.Manifest.SourceObservations[0]
	if err := builder.SetMembershipScopes([]catalogs.ProviderMembershipScope{{
		PublisherID: "public-catalog", BindingID: "public-provider", BindingRevision: "1",
		ProviderID: "provider", Region: "default-endpoint", APISurface: "models", Public: true,
		Authority: catalogs.MembershipScopeAuthority,
		Inventory: &catalogs.MembershipInventory{ObservationID: observation.ObservationID, ObservedAt: observation.ObservedAt, ModelIDs: []string{"exact/ID"}},
	}}); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	generation.Payload, err = catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	generation.Manifest.Payload = catalogs.DescribeCatalogPayload(generation.Payload)
	catalogPath := filepath.Join(t.TempDir(), "catalog")
	if _, err := workspace.Project(t.Context(), catalogPath, catalog, workspace.Identity{
		GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum,
	}); err != nil {
		t.Fatal(err)
	}
	projected, err := catalogs.NewFromPath(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := projected.LoadReport().Err(); err != nil {
		t.Fatal(err)
	}
	catalog, err = projected.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(catalog.MembershipScopes(), builder.MembershipScopes()) {
		t.Fatal("workspace changed published membership records")
	}
	generation.Payload, err = catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	generation.Manifest.Payload = catalogs.DescribeCatalogPayload(generation.Payload)
	endpoints, err := workspace.EncodeEndpointProjection(catalog, workspace.Identity{
		GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum,
	})
	if err != nil {
		t.Fatal(err)
	}
	writePromotionFile(t, filepath.Join(catalogPath, "endpoints.yaml"), endpoints)
	bundle, err := artifact.Build(generation)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := artifact.StageReleaseAssets(t.TempDir(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := catalogs.BootstrapManifest{
		ManifestVersion: catalogs.CurrentBootstrapManifestVersion, GenerationID: generation.Manifest.GenerationID,
		GeneratedAt: generation.Manifest.GeneratedAt, SchemaVersion: generation.Manifest.SchemaVersion,
		Payload: generation.Manifest.Payload, SemanticChecksum: semantic,
	}
	data, err := json.Marshal(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	writePromotionFile(t, filepath.Join(catalogPath, "generation.json"), data)
	return catalogPath, assets.Directory, generation
}

func writePromotionFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, constants.FilePermissions); err != nil {
		t.Fatal(err)
	}
}
