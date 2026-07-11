package catalogstore

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestCatalogGenerationMigrationFromLegacyV0Fixture(t *testing.T) {
	generatedAt := time.Date(2026, time.July, 9, 20, 0, 0, 0, time.UTC)
	options := LegacyMigrationOptions{
		GenerationID:     "legacy-v0-generation",
		SyncRunID:        "legacy-v0-migration-run",
		ObservationID:    "legacy-v0-fixture",
		GeneratedAt:      generatedAt,
		ValidatorVersion: "legacy-migrator/v1",
	}
	generation, err := MigrateLegacyDirectory(context.Background(), "testdata/legacy-v0", options)
	if err != nil {
		t.Fatalf("MigrateLegacyDirectory: %v", err)
	}
	if err := generation.Validate(); err != nil {
		t.Fatalf("Validate generation: %v", err)
	}
	if generation.Manifest.GeneratedAt != generatedAt {
		t.Fatalf("generated_at = %s", generation.Manifest.GeneratedAt)
	}
	if len(generation.Manifest.SourceObservations) != 1 ||
		generation.Manifest.SourceObservations[0].ObservationID != "legacy-v0-fixture" {
		t.Fatalf("source observations = %#v", generation.Manifest.SourceObservations)
	}
	repeated, err := MigrateLegacyDirectory(context.Background(), "testdata/legacy-v0", options)
	if err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	if !bytes.Equal(generation.Payload, repeated.Payload) ||
		generation.Manifest.Payload.Checksum != repeated.Manifest.Payload.Checksum {
		t.Fatal("same legacy fixture and metadata did not produce identical payload bytes/checksum")
	}

	catalog, err := DecodeCatalogPayload(generation.Payload)
	if err != nil {
		t.Fatalf("DecodeCatalogPayload: %v", err)
	}
	providerModel, err := catalog.ProviderModel("acme", "acme-chat")
	if err != nil {
		t.Fatalf("ProviderModel: %v", err)
	}
	if providerModel.Description != "Legacy provider model fixture" {
		t.Fatalf("provider description = %q", providerModel.Description)
	}
	author, err := catalog.Author("acme")
	if err != nil {
		t.Fatalf("Author: %v", err)
	}
	if _, found := author.Models["acme-chat"]; !found {
		t.Fatal("author model was not migrated")
	}
}

func TestCatalogGenerationMigrationRejectsCorruptLegacyFixture(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "providers.yaml"),
		[]byte("- id: broken\n  name: [unterminated\n"),
		constants.FilePermissions,
	); err != nil {
		t.Fatalf("Write corrupt fixture: %v", err)
	}
	_, err := MigrateLegacyDirectory(context.Background(), root, LegacyMigrationOptions{
		GenerationID:     "corrupt",
		SyncRunID:        "corrupt-run",
		ObservationID:    "corrupt-observation",
		GeneratedAt:      time.Date(2026, time.July, 9, 20, 0, 0, 0, time.UTC),
		ValidatorVersion: "legacy-migrator/v1",
	})
	var parseErr *pkgerrors.ParseError
	if !stderrors.As(err, &parseErr) {
		t.Fatalf("error = %T: %v, want *errors.ParseError", err, err)
	}
}

func TestCatalogGenerationMigrationRequiresDeterministicMetadata(t *testing.T) {
	_, err := MigrateLegacyDirectory(context.Background(), "testdata/legacy-v0", LegacyMigrationOptions{})
	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error = %T: %v, want *errors.ValidationError", err, err)
	}
	if validationErr.Field == "" {
		t.Fatal("validation error has no field")
	}
}
