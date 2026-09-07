package runtime

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyDirectoryMigrationPublicationPreservesState(t *testing.T) {
	t.Parallel()
	request := migrationPublicationFixture(t)
	prepared, err := PrepareDirectoryMigration(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(prepared.JournalDirectory, migrationJournalName)
	before, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyDirectoryMigrationPublication(t.Context(), request); err == nil {
		t.Fatal("verification accepted an unpublished target")
	}
	if _, err := os.Lstat(request.TargetDirectory); !os.IsNotExist(err) {
		t.Fatal("verification created a target")
	}
	after, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("verification advanced an unpublished migration")
	}
	if _, err := PublishDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	selected := openTestRuntime(t, WithStateDirectory(request.TargetDirectory), WithSchedulerIdentity(request.SourceIdentity), WithSource(newStubSource("migration-source")))
	if err := VerifyDirectoryMigrationPublication(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if err := selected.Close(); err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(request.TargetDirectory, migrationReceiptName)
	if err := os.WriteFile(receipt, []byte("conflicting receipt"), ownerRecordMode); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDirectoryMigrationPublication(t.Context(), request); err == nil {
		t.Fatal("verification accepted a conflicting receipt")
	}
	data, err := os.ReadFile(receipt)
	if err != nil || string(data) != "conflicting receipt" {
		t.Fatal("verification rewrote the conflicting receipt")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := VerifyDirectoryMigrationPublication(ctx, request); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("cancelled verification = %v", err)
	}
}

func TestMigrationStartupRefusesReplacedTargetBeforeInitialization(t *testing.T) {
	t.Parallel()
	request := migrationPublicationFixture(t)
	if _, err := PublishDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDirectoryMigrationPublication(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(request.TargetDirectory, request.TargetDirectory+"-preserved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(request.TargetDirectory, runtimeDirectoryMode); err != nil {
		t.Fatal(err)
	}
	connected, err := Open(t.Context(), WithStateDirectory(request.TargetDirectory), WithSchedulerIdentity(request.SourceIdentity), WithDirectoryOwner(request.Owner), WithSource(newStubSource("migration-source")), WithPublishedDirectoryMigration(request))
	if err == nil {
		_ = connected.Close()
		t.Fatal("migration startup initialized a replacement directory after preflight")
	}
	for _, name := range []string{ownerRecordName, instanceSeedFileName, layerDirectoryName} {
		if _, err := os.Lstat(filepath.Join(request.TargetDirectory, name)); !os.IsNotExist(err) {
			t.Fatalf("migration startup initialized %s in the replacement directory", name)
		}
	}
}
