package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func migrationJournalFixture(t *testing.T) (string, directoryMigrationManifest) {
	t.Helper()
	base := t.TempDir()
	digest := sha256.Sum256([]byte("0123456789abcdef0123456789abcdef"))
	return filepath.Join(base, "journals"), directoryMigrationManifest{
		SchemaVersion: 1, OperationID: "move-runtime-a", SourceDirectory: filepath.Join(base, "old-runtime"), TargetDirectory: filepath.Join(base, "new-runtime"),
		SourceIdentity: "retained-instance", Owner: DirectoryOwner{Product: "starmap", Deployment: "local", Instance: "default"},
		Files: []directoryMigrationFile{{Source: "catalog-runtime/instance-seed", Target: "instance-seed", Size: 32, SHA256: hex.EncodeToString(digest[:])}},
	}
}

func TestMigrationJournalRetainsOrderedIntentAcrossRestart(t *testing.T) {
	t.Parallel()
	root, manifest := migrationJournalFixture(t)
	journal, err := openDirectoryMigrationJournal(t.Context(), root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if journal.phase != migrationPrepared {
		t.Fatal("new journal has no durable prepared state")
	}
	if err := journal.advance(t.Context(), migrationVerified); err == nil {
		t.Fatal("journal skipped copy verification prerequisites")
	}
	if err := journal.advance(t.Context(), migrationCopied); err != nil {
		t.Fatal(err)
	}
	directory := journal.directory
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := openDirectoryMigrationJournal(t.Context(), root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resumed.Close() }()
	if resumed.phase != migrationCopied {
		t.Fatal("resume lost durable progress")
	}
	for _, phase := range []migrationPhase{migrationVerified, migrationPromoted, migrationCompleted} {
		if err := resumed.advance(t.Context(), phase); err != nil {
			t.Fatal(err)
		}
	}
	if err := resumed.advance(t.Context(), migrationCompleted); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("progress changed migration intent")
	}
	for _, path := range []string{manifest.SourceDirectory, manifest.TargetDirectory} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("journal preparation changed runtime directories")
		}
	}
}

func TestMigrationJournalRefusesChangedIntentAndConcurrentOwner(t *testing.T) {
	t.Parallel()
	root, manifest := migrationJournalFixture(t)
	first, err := openDirectoryMigrationJournal(t.Context(), root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := openDirectoryMigrationJournal(t.Context(), root, manifest); err == nil {
		_ = other.Close()
		t.Fatal("two journal writers held the same operation")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	manifest.SourceIdentity = "different-instance"
	if changed, err := openDirectoryMigrationJournal(t.Context(), root, manifest); err == nil {
		_ = changed.Close()
		t.Fatal("operation ID accepted a different intent")
	}
}

func TestMigrationJournalCancellationCreatesNoFiles(t *testing.T) {
	t.Parallel()
	root, manifest := migrationJournalFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if journal, err := openDirectoryMigrationJournal(ctx, root, manifest); err == nil {
		_ = journal.Close()
		t.Fatal("cancelled journal opened")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("cancelled journal created files")
	}
}
