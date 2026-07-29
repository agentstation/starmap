package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogartifact"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

func TestArtifactReleaseCommandStagesExactCommittedGeneration(t *testing.T) {
	root := t.TempDir()
	storePath, committed := releaseFixtureStore(t)
	var firstOutput bytes.Buffer
	if err := run([]string{
		"--generation-store", storePath,
		"--output-dir", root,
	}, &firstOutput); err != nil {
		t.Fatalf("run first: %v", err)
	}
	var first releaseReport
	if err := json.Unmarshal(firstOutput.Bytes(), &first); err != nil {
		t.Fatalf("Unmarshal report: %v", err)
	}
	if first.GenerationID != committed.Manifest.GenerationID ||
		len(first.Files) != 3 ||
		len(first.ArchiveChecksum) != len("sha256:")+64 {
		t.Fatalf("report = %#v", first)
	}
	for _, path := range first.Files {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("release asset %q: %v", path, err)
		}
	}
	archive, err := os.ReadFile(filepath.Join(first.Directory, catalogartifact.Filename))
	if err != nil {
		t.Fatalf("ReadFile archive: %v", err)
	}
	statement, err := os.ReadFile(
		filepath.Join(first.Directory, catalogartifact.AttestationFilename),
	)
	if err != nil {
		t.Fatalf("ReadFile statement: %v", err)
	}
	staged, err := catalogartifact.Open(archive, statement)
	if err != nil {
		t.Fatalf("Open staged artifact: %v", err)
	}
	if !reflect.DeepEqual(staged.Manifest, committed.Manifest) {
		t.Fatalf(
			"staged manifest = %#v, want %#v",
			staged.Manifest,
			committed.Manifest,
		)
	}

	var secondOutput bytes.Buffer
	if err := run([]string{
		"--generation-store", storePath,
		"--output-dir", root,
	}, &secondOutput); err != nil {
		t.Fatalf("run idempotent retry: %v", err)
	}
	if secondOutput.String() != firstOutput.String() {
		t.Fatalf("retry report changed:\nfirst %s\nsecond %s", firstOutput.String(), secondOutput.String())
	}
}

func TestArtifactReleaseCommandVerifiesDownloadedReleaseSet(t *testing.T) {
	root := t.TempDir()
	storePath, _ := releaseFixtureStore(t)
	var stagedOutput bytes.Buffer
	if err := run([]string{
		"--generation-store", storePath,
		"--output-dir", root,
	}, &stagedOutput); err != nil {
		t.Fatalf("stage release: %v", err)
	}
	var staged releaseReport
	if err := json.Unmarshal(stagedOutput.Bytes(), &staged); err != nil {
		t.Fatalf("Unmarshal staged report: %v", err)
	}

	var verifiedOutput bytes.Buffer
	if err := run([]string{"--verify-dir", staged.Directory}, &verifiedOutput); err != nil {
		t.Fatalf("verify release: %v", err)
	}
	var verified releaseReport
	if err := json.Unmarshal(verifiedOutput.Bytes(), &verified); err != nil {
		t.Fatalf("Unmarshal verified report: %v", err)
	}
	if verified.GenerationID != staged.GenerationID || verified.ArchiveChecksum != staged.ArchiveChecksum || len(verified.Files) != 3 {
		t.Fatalf("verified report = %#v, staged = %#v", verified, staged)
	}
}

func TestArtifactReleaseCommandRejectsTamperedReleaseSet(t *testing.T) {
	root := t.TempDir()
	storePath, _ := releaseFixtureStore(t)
	var output bytes.Buffer
	if err := run([]string{
		"--generation-store", storePath,
		"--output-dir", root,
	}, &output); err != nil {
		t.Fatalf("stage release: %v", err)
	}
	var staged releaseReport
	if err := json.Unmarshal(output.Bytes(), &staged); err != nil {
		t.Fatalf("Unmarshal staged report: %v", err)
	}
	archivePath := filepath.Join(staged.Directory, catalogartifact.Filename)
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Read archive: %v", err)
	}
	if err := os.WriteFile(archivePath, append(archive, 'x'), constants.FilePermissions); err != nil {
		t.Fatalf("Tamper archive: %v", err)
	}
	if err := run([]string{"--verify-dir", staged.Directory}, io.Discard); err == nil {
		t.Fatal("verification accepted a tampered archive")
	}
}

func TestArtifactReleaseCommandRejectsTamperedDetachedStatement(t *testing.T) {
	root := t.TempDir()
	storePath, _ := releaseFixtureStore(t)
	var output bytes.Buffer
	if err := run([]string{
		"--generation-store", storePath,
		"--output-dir", root,
	}, &output); err != nil {
		t.Fatalf("stage release: %v", err)
	}
	var staged releaseReport
	if err := json.Unmarshal(output.Bytes(), &staged); err != nil {
		t.Fatalf("Unmarshal staged report: %v", err)
	}
	statementPath := filepath.Join(staged.Directory, catalogartifact.AttestationFilename)
	statement, err := os.ReadFile(statementPath)
	if err != nil {
		t.Fatalf("Read detached statement: %v", err)
	}
	if err := os.WriteFile(statementPath, append(statement, 'x'), constants.FilePermissions); err != nil {
		t.Fatalf("Tamper detached statement: %v", err)
	}
	if err := run([]string{"--verify-dir", staged.Directory}, io.Discard); err == nil {
		t.Fatal("verification accepted a tampered detached statement")
	}
}

func TestArtifactReleaseCommandRequiresCommittedGenerationStore(t *testing.T) {
	if err := run([]string{"--output-dir", t.TempDir()}, io.Discard); err == nil {
		t.Fatal("staging accepted a missing generation store")
	}
	storePath, _ := releaseFixtureStore(t)
	if err := run([]string{
		"--verify-dir", t.TempDir(),
		"--generation-store", storePath,
	}, io.Discard); err == nil {
		t.Fatal("verification accepted a staging-only generation store")
	}
}

func releaseFixtureStore(
	t *testing.T,
) (string, catalogstore.Generation) {
	t.Helper()
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build empty catalog: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)
	generatedAt := time.Date(2026, time.July, 29, 22, 0, 0, 0, time.UTC)
	links := []catalogs.SourceObservationLink{
		releaseObservation(
			catalogmeta.ProvidersID,
			"providers-observation",
			descriptor.Checksum,
			generatedAt,
		),
		releaseObservation(
			catalogmeta.ModelsDevGitID,
			"modelsdev-observation",
			descriptor.Checksum,
			generatedAt,
		),
	}
	generation := catalogstore.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    "exact-release-generation",
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
			SyncRunID:          "sync-exact-release",
			SourceObservations: links,
			Completeness:       catalogs.GenerationCompletenessComplete,
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
	return storePath, generation
}

func releaseObservation(
	source catalogmeta.SourceID,
	id string,
	checksum string,
	observedAt time.Time,
) catalogs.SourceObservationLink {
	return catalogs.SourceObservationLink{
		Source:        source,
		ObservationID: id,
		ObservedAt:    observedAt,
		Revision: catalogmeta.ObservationRevision{
			Kind:  catalogmeta.ObservationRevisionKindContentDigest,
			Value: checksum,
		},
		Completeness:     catalogmeta.ObservationCompletenessComplete,
		Status:           catalogmeta.ObservationStatusSucceeded,
		EvidenceChecksum: checksum,
	}
}
