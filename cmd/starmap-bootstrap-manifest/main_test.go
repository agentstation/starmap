package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap/manifest"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

func TestScheduledGenerationManifestCommandWritesChangedOnceAndPreservesUnchangedBytes(t *testing.T) {
	catalogDir := filepath.Join("..", "..", "internal", "embedded", "catalog")
	outputDir := t.TempDir()
	manifestPath := filepath.Join(outputDir, "generation.json")
	endpointsPath := filepath.Join(outputDir, "endpoints.yaml")
	now := time.Date(2026, time.July, 10, 16, 0, 0, 0, time.UTC)
	var firstOutput bytes.Buffer
	if err := run([]string{
		"--catalog-dir", catalogDir,
		"--output", manifestPath,
		"--endpoints-output", endpointsPath,
	}, &firstOutput, now); err != nil {
		t.Fatalf("run first: %v", err)
	}
	var first manifest.Report
	if err := json.Unmarshal(firstOutput.Bytes(), &first); err != nil {
		t.Fatalf("Unmarshal first: %v", err)
	}
	if !first.Changed || first.GenerationID == "" || first.PayloadChecksum == "" {
		t.Fatalf("first report = %#v", first)
	}
	firstBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile first manifest: %v", err)
	}
	if info, err := os.Stat(manifestPath); err != nil || info.Mode().Perm() != constants.FilePermissions {
		t.Fatalf("manifest permissions = %v, %v", info, err)
	}
	firstEndpoints, err := os.ReadFile(endpointsPath)
	if err != nil || len(firstEndpoints) == 0 {
		t.Fatalf("endpoint projection = %d bytes, %v", len(firstEndpoints), err)
	}

	var secondOutput bytes.Buffer
	if err := run([]string{
		"--catalog-dir", catalogDir,
		"--output", manifestPath,
		"--endpoints-output", endpointsPath,
	}, &secondOutput, now.Add(24*time.Hour)); err != nil {
		t.Fatalf("run second: %v", err)
	}
	var second manifest.Report
	if err := json.Unmarshal(secondOutput.Bytes(), &second); err != nil {
		t.Fatalf("Unmarshal second: %v", err)
	}
	secondBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile second manifest: %v", err)
	}
	if second.Changed || second.GenerationID != first.GenerationID || !bytes.Equal(secondBytes, firstBytes) {
		t.Fatalf("unchanged rerun report/bytes = %#v/%v", second, bytes.Equal(secondBytes, firstBytes))
	}
	secondEndpoints, err := os.ReadFile(endpointsPath)
	if err != nil || !bytes.Equal(secondEndpoints, firstEndpoints) {
		t.Fatalf("unchanged endpoint bytes = %v, %v", bytes.Equal(secondEndpoints, firstEndpoints), err)
	}
}

func TestScheduledGenerationManifestReplacesPriorSchemaManifest(t *testing.T) {
	catalogDir := filepath.Join("..", "..", "internal", "embedded", "catalog")
	manifestPath := filepath.Join(t.TempDir(), "generation.json")
	now := time.Date(2026, time.July, 10, 17, 0, 0, 0, time.UTC)
	if err := run([]string{
		"--catalog-dir", catalogDir,
		"--output", manifestPath,
	}, &bytes.Buffer{}, now); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var prior catalogs.BootstrapManifest
	if err := json.Unmarshal(data, &prior); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	prior.SchemaVersion = catalogs.CurrentCatalogSchemaVersion - 1
	data, err = json.MarshalIndent(prior, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(manifestPath, data, constants.FilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var output bytes.Buffer
	if err := run([]string{
		"--catalog-dir", catalogDir,
		"--output", manifestPath,
	}, &output, now.Add(time.Hour)); err != nil {
		t.Fatalf("replace prior schema: %v", err)
	}
	var report manifest.Report
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("Unmarshal report: %v", err)
	}
	if !report.Changed {
		t.Fatalf("report = %#v, want changed schema publication", report)
	}
	currentData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile current: %v", err)
	}
	current, err := catalogs.ParseBootstrapManifestJSON(currentData)
	if err != nil {
		t.Fatalf("ParseBootstrapManifestJSON: %v", err)
	}
	if current.SchemaVersion != catalogs.CurrentCatalogSchemaVersion {
		t.Fatalf("schema version = %d", current.SchemaVersion)
	}
}

func TestScheduledGenerationManifestUsesExactCommittedIdentity(t *testing.T) {
	catalogDir := filepath.Join("..", "..", "internal", "embedded", "catalog")
	builder, err := catalogs.NewFromPath(catalogDir)
	if err != nil {
		t.Fatalf("NewFromPath: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)
	generatedAt := time.Date(2026, time.July, 29, 21, 0, 0, 0, time.UTC)
	generation := catalogstore.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    "exact-committed-generation",
			GeneratedAt:     generatedAt,
			Payload:         descriptor,
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "test/v1",
				ValidatedAt:      generatedAt,
				Status:           catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{{
					Name: "catalog", Status: catalogs.GenerationValidationCheckPassed,
				}},
			},
			SyncRunID: "sync-exact",
			SourceObservations: []catalogs.SourceObservationLink{
				{
					Source:        catalogmeta.ProvidersID,
					ObservationID: "providers-exact",
					ObservedAt:    generatedAt,
					Revision: catalogmeta.ObservationRevision{
						Kind:  catalogmeta.ObservationRevisionKindContentDigest,
						Value: descriptor.Checksum,
					},
					Completeness:     catalogmeta.ObservationCompletenessComplete,
					Status:           catalogmeta.ObservationStatusSucceeded,
					EvidenceChecksum: descriptor.Checksum,
				},
			},
			ReviewCandidates: []catalogmeta.ReviewCandidate{},
			Completeness:     catalogs.GenerationCompletenessComplete,
			ConsumerCompatibility: catalogs.ConsumerCompatibility{
				MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
				MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			},
		},
		Payload: payload,
	}
	storePath := filepath.Join(t.TempDir(), "store")
	store, err := catalogstore.NewFilesystem(storePath)
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	manifestPath := filepath.Join(t.TempDir(), "generation.json")
	var output bytes.Buffer
	if err := run([]string{
		"--catalog-dir", catalogDir,
		"--output", manifestPath,
		"--generation-store", storePath,
	}, &output, generatedAt.Add(time.Hour)); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile manifest: %v", err)
	}
	manifest, err := catalogs.ParseBootstrapManifestJSON(data)
	if err != nil {
		t.Fatalf("ParseBootstrapManifestJSON: %v", err)
	}
	if manifest.GenerationID != generation.Manifest.GenerationID ||
		manifest.GeneratedAt != generation.Manifest.GeneratedAt ||
		manifest.Payload != generation.Manifest.Payload {
		t.Fatalf("manifest = %#v, generation = %#v", manifest, generation.Manifest)
	}
}

func TestScheduledGenerationManifestAllowsEmptyStoreOnlyWhenUnchanged(t *testing.T) {
	catalogDir := filepath.Join("..", "..", "internal", "embedded", "catalog")
	outputDir := t.TempDir()
	manifestPath := filepath.Join(outputDir, "generation.json")
	now := time.Date(2026, time.July, 29, 22, 0, 0, 0, time.UTC)

	if err := run([]string{
		"--catalog-dir", catalogDir,
		"--output", manifestPath,
	}, &bytes.Buffer{}, now); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}

	emptyStore := filepath.Join(t.TempDir(), "store")
	var output bytes.Buffer
	if err := run([]string{
		"--catalog-dir", catalogDir,
		"--output", manifestPath,
		"--generation-store", emptyStore,
	}, &output, now.Add(time.Hour)); err != nil {
		t.Fatalf("unchanged run with empty store: %v", err)
	}
	var report manifest.Report
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("Unmarshal report: %v", err)
	}
	if report.Changed {
		t.Fatalf("report.Changed = true, want false: %#v", report)
	}
}

func TestScheduledGenerationManifestRejectsChangedCatalogWithoutCommittedGeneration(t *testing.T) {
	err := run([]string{
		"--catalog-dir", filepath.Join("..", "..", "internal", "embedded", "catalog"),
		"--output", filepath.Join(t.TempDir(), "generation.json"),
		"--generation-store", filepath.Join(t.TempDir(), "store"),
	}, &bytes.Buffer{}, time.Date(2026, time.July, 29, 23, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("run error = nil, want missing committed generation")
	}
}
