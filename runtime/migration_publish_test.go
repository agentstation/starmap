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

func migrationPublicationFixture(t *testing.T) DirectoryMigrationRequest {
	t.Helper()
	root, manifest := migrationJournalFixture(t)
	source := newStubSource("migration-source")
	source.replies = []SourceRead{testSourceRead(t, "retained-generation", testCatalogPayload(t, "retained-provider", "retained-model", "Retained Model"), time.Now().UTC())}
	connected := openTestRuntime(t, WithStateDirectory(manifest.SourceDirectory), WithSource(source))
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	identity := connected.Status().InstanceIdentity
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	return DirectoryMigrationRequest{
		OperationID: manifest.OperationID, SourceDirectory: manifest.SourceDirectory, TargetDirectory: manifest.TargetDirectory,
		JournalRoot: root, SourceIdentity: identity, Owner: manifest.Owner,
	}
}

func TestPublishDirectoryMigrationRecoversProcessExit(t *testing.T) {
	t.Parallel()
	for _, stop := range []string{"renamed", "promoted", "source-retired", "receipt-written", "target-ready"} {
		t.Run(stop, func(t *testing.T) {
			t.Parallel()
			request := migrationPublicationFixture(t)
			encoded, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(filepath.Dir(request.SourceDirectory), "publication-request.json")
			if err := os.WriteFile(input, encoded, ownerRecordMode); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestMigrationPublicationCrashChild$")
			command.Env = append(os.Environ(), "STARMAP_PUBLICATION_CRASH_INPUT="+input, "STARMAP_PUBLICATION_CRASH_STOP="+stop)
			output, err := command.CombinedOutput()
			var exit *exec.ExitError
			if !stderrors.As(err, &exit) || exit.ExitCode() != 86 {
				t.Fatalf("publication crash = %v: %s", err, output)
			}
			result, err := PublishDirectoryMigration(t.Context(), request)
			if err != nil || result.Phase != "promoted" {
				t.Fatalf("publication recovery = %+v, %v", result, err)
			}
			connected := openTestRuntime(t, WithStateDirectory(result.TargetDirectory), WithSchedulerIdentity(result.SchedulerIdentity), WithSource(newStubSource("migration-source")))
			if _, err := connected.Catalog().FindModel("retained-model"); err != nil {
				t.Fatal("recovery lost the retained catalog", err)
			}
			if connected.Status().InstanceIdentity != request.SourceIdentity {
				t.Fatal("recovery changed instance identity")
			}
			if err := refuseRetiredMigration(request.SourceDirectory); err == nil {
				t.Fatal("recovery did not retire the source")
			}
		})
	}
}

func TestMigrationPublicationCrashChild(t *testing.T) {
	input := os.Getenv("STARMAP_PUBLICATION_CRASH_INPUT")
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
	stop := os.Getenv("STARMAP_PUBLICATION_CRASH_STOP")
	_, err = publishDirectoryMigration(t.Context(), request, func(event, _ string) error {
		if event == stop {
			os.Exit(86)
		}
		return nil
	})
	t.Fatalf("publication crash point was not reached: %v", err)
}

func TestPublishDirectoryMigrationRetryPreservesLiveUpdates(t *testing.T) {
	t.Parallel()
	request := migrationPublicationFixture(t)
	result, err := PublishDirectoryMigration(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	source := newStubSource("migration-source")
	source.replies = []SourceRead{testSourceRead(t, "new-generation", testCatalogPayload(t, "new-provider", "new-model", "New Model"), time.Now().UTC())}
	connected := openTestRuntime(t, WithStateDirectory(result.TargetDirectory), WithSchedulerIdentity(result.SchedulerIdentity), WithSource(source))
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(result.TargetDirectory, layerDirectoryName, sourceLayerFileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	retried, err := PublishDirectoryMigration(t.Context(), request)
	if err != nil || retried != result {
		t.Fatalf("live publication retry = %+v, %v", retried, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("publication retry changed new catalog bytes")
	}
	if _, err := connected.Catalog().FindModel("new-model"); err != nil {
		t.Fatal("publication retry reverted live state", err)
	}
}

func TestPublishDirectoryMigrationSupportsSuccessiveMoves(t *testing.T) {
	t.Parallel()
	first := migrationPublicationFixture(t)
	if _, err := PublishDirectoryMigration(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.OperationID = "second-move"
	second.SourceDirectory = first.TargetDirectory
	second.TargetDirectory = filepath.Join(filepath.Dir(first.TargetDirectory), "third-runtime")
	result, err := PublishDirectoryMigration(t.Context(), second)
	if err != nil {
		t.Fatal(err)
	}
	connected := openTestRuntime(t, WithStateDirectory(result.TargetDirectory), WithSchedulerIdentity(result.SchedulerIdentity), WithSource(newStubSource("migration-source")))
	if _, err := connected.Catalog().FindModel("retained-model"); err != nil {
		t.Fatal("successive migration lost retained state", err)
	}
	if _, err := PublishDirectoryMigration(t.Context(), first); err == nil {
		t.Fatal("a retired target was reported as a ready replacement")
	}
}

func TestPublishDirectoryMigrationRefusesChangedReceipts(t *testing.T) {
	t.Parallel()
	for _, record := range []string{migrationRetiredName, migrationReceiptName} {
		t.Run(record, func(t *testing.T) {
			request := migrationPublicationFixture(t)
			if _, err := PublishDirectoryMigration(t.Context(), request); err != nil {
				t.Fatal(err)
			}
			directory := request.TargetDirectory
			if record == migrationRetiredName {
				directory = request.SourceDirectory
			}
			path := filepath.Join(directory, record)
			before := []byte("preserve conflicting receipt")
			if err := os.WriteFile(path, before, ownerRecordMode); err != nil {
				t.Fatal(err)
			}
			if _, err := PublishDirectoryMigration(t.Context(), request); err == nil {
				t.Fatal("publication accepted a conflicting receipt")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("publication replaced conflicting receipt")
			}
		})
	}
}

func TestPublishDirectoryMigrationRetainsCatalogAndRetiresSource(t *testing.T) {
	t.Parallel()
	request := migrationPublicationFixture(t)
	before, err := os.ReadFile(filepath.Join(request.SourceDirectory, layerDirectoryName, sourceLayerFileName))
	if err != nil {
		t.Fatal(err)
	}
	result, err := PublishDirectoryMigration(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Phase != "promoted" || result.TargetDirectory != request.TargetDirectory || result.SchedulerIdentity != request.SourceIdentity {
		t.Fatalf("publication = %+v", result)
	}
	if old, err := Open(t.Context(), WithStateDirectory(request.SourceDirectory), WithSourcePollInterval(0), WithAcquisitionEnabled(false)); err == nil {
		_ = old.Close()
		t.Fatal("retired source started")
	}
	connected := openTestRuntime(t, WithStateDirectory(result.TargetDirectory), WithDirectoryOwner(request.Owner), WithSchedulerIdentity(result.SchedulerIdentity), WithSource(newStubSource("migration-source")))
	if connected.Status().InstanceIdentity != request.SourceIdentity {
		t.Fatal("publication changed runtime identity")
	}
	if _, err := connected.Catalog().FindModel("retained-model"); err != nil {
		t.Fatal("published runtime lost its retained catalog", err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{request.SourceDirectory, request.TargetDirectory} {
		after, err := os.ReadFile(filepath.Join(directory, layerDirectoryName, sourceLayerFileName))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("publication changed retained source bytes")
		}
	}
	repeated, err := PublishDirectoryMigration(t.Context(), request)
	if err != nil || repeated != result {
		t.Fatalf("publication retry = %+v, %v", repeated, err)
	}
}

func TestPublishDirectoryMigrationRefusesUnrelatedTarget(t *testing.T) {
	t.Parallel()
	request := migrationPublicationFixture(t)
	if err := os.Mkdir(request.TargetDirectory, runtimeDirectoryMode); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishDirectoryMigration(t.Context(), request); err == nil {
		t.Fatal("publication adopted an existing empty target")
	}
	entries, err := os.ReadDir(request.TargetDirectory)
	if err != nil || len(entries) != 0 {
		t.Fatal("refused publication changed target")
	}
	if _, err := os.Stat(filepath.Join(request.SourceDirectory, migrationRetiredName)); !os.IsNotExist(err) {
		t.Fatal("refused publication retired the source")
	}
}

func TestPublishDirectoryMigrationRejectsInvalidRetainedCatalog(t *testing.T) {
	t.Parallel()
	request := migrationPublicationFixture(t)
	if err := os.WriteFile(filepath.Join(request.SourceDirectory, layerDirectoryName, sourceLayerFileName), []byte("invalid retained layer"), ownerRecordMode); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishDirectoryMigration(t.Context(), request); err == nil {
		t.Fatal("publication accepted an unreadable retained catalog")
	}
	if _, err := os.Stat(request.TargetDirectory); !os.IsNotExist(err) {
		t.Fatal("invalid retained catalog reached the final target")
	}
}
