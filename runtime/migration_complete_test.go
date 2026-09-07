package runtime

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCompleteDirectoryMigrationRequiresSelectedRuntime(t *testing.T) {
	t.Parallel()
	request := migrationPublicationFixture(t)
	published, err := PublishDirectoryMigration(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(published.JournalDirectory, migrationJournalName)
	before, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	unrelated := openTestRuntime(t)
	if _, err := unrelated.CompleteDirectoryMigration(t.Context(), request); err == nil {
		t.Fatal("unrelated runtime completed a root switch")
	}
	after, err := os.ReadFile(journalPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("refused completion changed migration progress")
	}
	selected := openTestRuntime(t, WithStateDirectory(request.TargetDirectory), WithSchedulerIdentity(request.SourceIdentity), WithDirectoryOwner(request.Owner), WithSource(newStubSource("migration-source")))
	wrong := request
	wrong.SourceIdentity = "another-identity"
	if _, err := selected.CompleteDirectoryMigration(t.Context(), wrong); err == nil {
		t.Fatal("runtime completed a different retained identity")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := selected.CompleteDirectoryMigration(ctx, request); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("cancelled completion = %v", err)
	}
	completed, err := selected.CompleteDirectoryMigration(t.Context(), request)
	if err != nil || completed.Phase != "completed" {
		t.Fatalf("completion = %+v, %v", completed, err)
	}
	repeated, err := selected.CompleteDirectoryMigration(t.Context(), request)
	if err != nil || repeated != completed {
		t.Fatalf("completion retry = %+v, %v", repeated, err)
	}
	data, err := os.ReadFile(journalPath)
	if err != nil || bytes.Count(data, []byte{'\n'}) != len(migrationPhases()) {
		t.Fatal("completion duplicated a journal event")
	}
	if err := selected.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := selected.CompleteDirectoryMigration(t.Context(), request); err == nil {
		t.Fatal("closed runtime completed a root switch")
	}
}
