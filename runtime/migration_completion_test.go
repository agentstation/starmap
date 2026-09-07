package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func completeMigrationFixture(t *testing.T, request DirectoryMigrationRequest) DirectoryMigrationPublication {
	t.Helper()
	if _, err := PublishDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	selected := openTestRuntime(t, WithStateDirectory(request.TargetDirectory), WithSchedulerIdentity(request.SourceIdentity), WithDirectoryOwner(request.Owner), WithSource(newStubSource("migration-source")), WithPublishedDirectoryMigration(request))
	completed, err := selected.CompleteDirectoryMigration(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if err := selected.Close(); err != nil {
		t.Fatal(err)
	}
	return completed
}

func TestDirectoryMigrationCompletionPreservesCurrentCatalog(t *testing.T) {
	t.Parallel()
	request := migrationPublicationFixture(t)
	if _, err := PublishDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDirectoryMigrationCompletion(t.Context(), request.TargetDirectory); err == nil {
		t.Fatal("publication alone acknowledged completion")
	}
	completeMigrationFixture(t, request)
	before, err := ReadDirectoryMigrationCompletion(t.Context(), request.TargetDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if before.OperationID != request.OperationID || before.SourceDirectory != request.SourceDirectory || before.TargetDirectory != request.TargetDirectory || before.Owner != request.Owner || before.SchedulerIdentity != request.SourceIdentity || !migrationDigestValid(before.ManifestSHA256) {
		t.Fatalf("completion = %+v", before)
	}
	source := newStubSource("migration-source")
	source.replies = []SourceRead{testSourceRead(t, "new-generation", testCatalogPayload(t, "new-provider", "new-model", "New Model"), time.Now().UTC())}
	connected := openTestRuntime(t, WithStateDirectory(request.TargetDirectory), WithSchedulerIdentity(request.SourceIdentity), WithDirectoryOwner(request.Owner), WithSource(source), WithCompletedDirectoryMigration(before))
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(request.JournalRoot, request.JournalRoot+"-archived"); err != nil {
		t.Fatal(err)
	}
	after, err := ReadDirectoryMigrationCompletion(t.Context(), request.TargetDirectory)
	if err != nil || before != after {
		t.Fatalf("updated target or archived journal invalidated completion: %+v, %v", after, err)
	}
	reopened := openTestRuntime(t, WithStateDirectory(request.TargetDirectory), WithSchedulerIdentity(request.SourceIdentity), WithDirectoryOwner(request.Owner), WithSource(newStubSource("migration-source")), WithCompletedDirectoryMigration(after))
	if _, err := reopened.Catalog().FindModel("new-model"); err != nil {
		t.Fatal("acknowledgement reverted current catalog", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ReadDirectoryMigrationCompletion(ctx, request.TargetDirectory); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("cancelled read = %v", err)
	}
	if _, err := ReadDirectoryMigrationCompletion(nil, request.TargetDirectory); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestDirectoryMigrationCompletionRefusesChangedEvidence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, relative string
		source         bool
	}{
		{"source-inventory", filepath.Join(layerDirectoryName, sourceLayerFileName), true},
		{"extra-source-file", "untracked-file", true},
		{"source-owner", ownerRecordName, true},
		{"source-seed", instanceSeedFileName, true},
		{"source-retirement", migrationRetiredName, true},
		{"target-owner", ownerRecordName, false},
		{"target-seed", instanceSeedFileName, false},
		{"target-receipt", migrationReceiptName, false},
		{"target-completion", migrationCompletionName, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := migrationPublicationFixture(t)
			completeMigrationFixture(t, request)
			root := request.TargetDirectory
			if tc.source {
				root = request.SourceDirectory
			}
			path := filepath.Join(root, tc.relative)
			changed := []byte("preserve changed evidence")
			if err := os.WriteFile(path, changed, ownerRecordMode); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadDirectoryMigrationCompletion(t.Context(), request.TargetDirectory); err == nil {
				t.Fatal("changed evidence acknowledged")
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, changed) {
				t.Fatal("verification modified evidence")
			}
		})
	}
}

func TestCompletedMigrationStartupRefusesReplacedTarget(t *testing.T) {
	t.Parallel()
	request := migrationPublicationFixture(t)
	completeMigrationFixture(t, request)
	completion, err := ReadDirectoryMigrationCompletion(t.Context(), request.TargetDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(request.TargetDirectory, request.TargetDirectory+"-preserved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(request.TargetDirectory, runtimeDirectoryMode); err != nil {
		t.Fatal(err)
	}
	selected, err := Open(t.Context(), WithStateDirectory(request.TargetDirectory), WithSchedulerIdentity(request.SourceIdentity), WithDirectoryOwner(request.Owner), WithSource(newStubSource("migration-source")), WithCompletedDirectoryMigration(completion))
	if err == nil {
		_ = selected.Close()
		t.Fatal("startup initialized a replaced completed target")
	}
	for _, name := range []string{ownerRecordName, instanceSeedFileName, layerDirectoryName} {
		if _, err := os.Lstat(filepath.Join(request.TargetDirectory, name)); !os.IsNotExist(err) {
			t.Fatalf("startup initialized %s", name)
		}
	}
}

func TestDirectoryMigrationCompletionSupportsSuccessiveMoves(t *testing.T) {
	t.Parallel()
	first := migrationPublicationFixture(t)
	completeMigrationFixture(t, first)
	second := first
	second.OperationID = "second-completed-move"
	second.SourceDirectory = first.TargetDirectory
	second.TargetDirectory = filepath.Join(filepath.Dir(first.TargetDirectory), "third-runtime")
	completeMigrationFixture(t, second)
	if _, err := ReadDirectoryMigrationCompletion(t.Context(), first.TargetDirectory); err == nil {
		t.Fatal("retired target acknowledged")
	}
	if err := os.Rename(first.SourceDirectory, first.SourceDirectory+"-archived"); err != nil {
		t.Fatal(err)
	}
	completion, err := ReadDirectoryMigrationCompletion(t.Context(), second.TargetDirectory)
	if err != nil {
		t.Fatal("second move requires the first source", err)
	}
	selected := openTestRuntime(t, WithStateDirectory(second.TargetDirectory), WithSchedulerIdentity(second.SourceIdentity), WithDirectoryOwner(second.Owner), WithSource(newStubSource("migration-source")), WithCompletedDirectoryMigration(completion))
	if _, err := selected.Catalog().FindModel("retained-model"); err != nil {
		t.Fatal("second move lost catalog", err)
	}
}

func TestDirectoryMigrationCompletionRecoversProcessExit(t *testing.T) {
	t.Parallel()
	for _, stop := range []string{"journal-completed", "completion-recorded"} {
		t.Run(stop, func(t *testing.T) {
			t.Parallel()
			request := migrationPublicationFixture(t)
			published, err := PublishDirectoryMigration(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(filepath.Dir(request.SourceDirectory), "completion-request.json")
			if err := os.WriteFile(input, encoded, ownerRecordMode); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestMigrationCompletionCrashChild$")
			command.Env = append(os.Environ(), "STARMAP_COMPLETION_CRASH_INPUT="+input, "STARMAP_COMPLETION_CRASH_STOP="+stop)
			output, err := command.CombinedOutput()
			var exit *exec.ExitError
			if !stderrors.As(err, &exit) || exit.ExitCode() != 86 {
				t.Fatalf("completion crash = %v: %s", err, output)
			}
			_, err = ReadDirectoryMigrationCompletion(t.Context(), request.TargetDirectory)
			if (err == nil) != (stop == "completion-recorded") {
				t.Fatalf("completion observation after %s = %v", stop, err)
			}
			completeMigrationFixture(t, request)
			if _, err := ReadDirectoryMigrationCompletion(t.Context(), request.TargetDirectory); err != nil {
				t.Fatal(err)
			}
			journal, err := os.ReadFile(filepath.Join(published.JournalDirectory, migrationJournalName))
			if err != nil || bytes.Count(journal, []byte{'\n'}) != len(migrationPhases()) {
				t.Fatal("completion retry duplicated progress")
			}
		})
	}
}

func TestMigrationCompletionCrashChild(t *testing.T) {
	input := os.Getenv("STARMAP_COMPLETION_CRASH_INPUT")
	if input == "" {
		return
	}
	encoded, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	var request DirectoryMigrationRequest
	if err := json.Unmarshal(encoded, &request); err != nil {
		t.Fatal(err)
	}
	selected := openTestRuntime(t, WithStateDirectory(request.TargetDirectory), WithSchedulerIdentity(request.SourceIdentity), WithDirectoryOwner(request.Owner), WithSource(newStubSource("migration-source")), WithPublishedDirectoryMigration(request))
	stop := os.Getenv("STARMAP_COMPLETION_CRASH_STOP")
	_, err = selected.completeDirectoryMigration(t.Context(), request, func(event, _ string) error {
		if event == stop {
			os.Exit(86)
		}
		return nil
	})
	t.Fatalf("completion crash point was not reached: %v", err)
}

func TestDirectoryMigrationCompletionRefusesMissingSourceAndCopiedReceipt(t *testing.T) {
	t.Parallel()
	request := migrationPublicationFixture(t)
	completeMigrationFixture(t, request)
	if err := os.Rename(request.SourceDirectory, request.SourceDirectory+"-preserved"); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDirectoryMigrationCompletion(t.Context(), request.TargetDirectory); err == nil {
		t.Fatal("missing source acknowledged")
	}
	if err := os.Rename(request.SourceDirectory+"-preserved", request.SourceDirectory); err != nil {
		t.Fatal(err)
	}
	moved := request.TargetDirectory + "-moved"
	if err := os.Rename(request.TargetDirectory, moved); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadDirectoryMigrationCompletion(t.Context(), moved); err == nil {
		t.Fatal("receipt accepted at another target")
	}
}

func TestDirectoryMigrationCompletionRefusesJournalRollback(t *testing.T) {
	t.Parallel()
	request := migrationPublicationFixture(t)
	completed := completeMigrationFixture(t, request)
	path := filepath.Join(completed.JournalDirectory, migrationJournalName)
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cut := bytes.LastIndexByte(bytes.TrimSuffix(encoded, []byte{'\n'}), '\n') + 1
	rolledBack := encoded[:cut]
	if err := os.WriteFile(path, rolledBack, ownerRecordMode); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDirectoryMigrationPublication(t.Context(), request); err == nil {
		t.Fatal("completion record accepted before completed journal phase")
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, rolledBack) {
		t.Fatal("verification repaired a conflicting journal")
	}
}
