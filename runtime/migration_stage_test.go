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

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestStageDirectoryMigrationCopiesAndVerifies(t *testing.T) {
	t.Parallel()
	request := directoryMigrationRequestFixture(t)
	prepared, err := PrepareDirectoryMigration(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := StageDirectoryMigration(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if staged.Phase != "verified" || staged.JournalDirectory != prepared.JournalDirectory || staged.FileCount != prepared.FileCount {
		t.Fatalf("staging = %+v", staged)
	}
	for source, target := range map[string]string{
		"catalog-runtime/instance-seed": "instance-seed", "catalog-runtime/layers.json": "catalog-runtime/layers.json",
	} {
		before, err := os.ReadFile(filepath.Join(request.SourceDirectory, source))
		if err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(filepath.Join(staged.StageDirectory, target))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("staged bytes differ from source")
		}
	}
	if _, err := os.Stat(request.TargetDirectory); !os.IsNotExist(err) {
		t.Fatal("staging published the target")
	}
	if connected, err := Open(t.Context(), WithStateDirectory(staged.StageDirectory), WithSchedulerIdentity(request.SourceIdentity)); err == nil {
		_ = connected.Close()
		t.Fatal("an incomplete migration started a runtime")
	}
	resumed, err := StageDirectoryMigration(t.Context(), request)
	if err != nil || resumed != staged {
		t.Fatalf("staging retry = %+v, %v", resumed, err)
	}
	root, err := os.OpenRoot(staged.StageDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	if _, err := verifyMigrationSourceIdentity(root, request.SourceIdentity); err != nil {
		t.Fatal("staged ownership does not retain source identity", err)
	}
}

func TestPendingMigrationRefusesBeforeRuntimeInitialization(t *testing.T) {
	t.Parallel()
	directory := privateRuntimeDirectory(t)
	if err := os.WriteFile(filepath.Join(directory, migrationPendingName), []byte("pending"), ownerRecordMode); err != nil {
		t.Fatal(err)
	}
	probe := &ownerProbeStore{Memory: storage.NewMemory()}
	connected, err := Open(t.Context(), WithStateDirectory(directory), WithClientOptions(starmap.WithCatalogStore(probe)))
	if connected != nil {
		_ = connected.Close()
	}
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) || conflict.Resource != "runtime directory migration" || probe.reads != 0 {
		t.Fatalf("pending migration reached runtime initialization: %v, %d store reads", err, probe.reads)
	}
	for _, name := range []string{ownerRecordName, instanceSeedFileName, layerDirectoryName} {
		if _, err := os.Lstat(filepath.Join(directory, name)); !os.IsNotExist(err) {
			t.Fatal("pending migration created runtime state")
		}
	}
}

func TestMigrationDirectoryPublicationNeverReplaces(t *testing.T) {
	t.Parallel()
	for _, existing := range []string{"empty directory", "populated directory", "file"} {
		t.Run(existing, func(t *testing.T) {
			directory := privateRuntimeDirectory(t)
			if err := os.Mkdir(filepath.Join(directory, "source"), runtimeDirectoryMode); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "source", "marker"), []byte("source"), ownerRecordMode); err != nil {
				t.Fatal(err)
			}
			if existing == "file" {
				if err := os.WriteFile(filepath.Join(directory, "target"), []byte("target"), ownerRecordMode); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(filepath.Join(directory, "target"), runtimeDirectoryMode); err != nil {
					t.Fatal(err)
				}
				if existing == "populated directory" {
					if err := os.WriteFile(filepath.Join(directory, "target", "marker"), []byte("target"), ownerRecordMode); err != nil {
						t.Fatal(err)
					}
				}
			}
			parent, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = parent.Close() }()
			if err := publishMigrationDirectory(parent, "source", "target"); err == nil {
				t.Fatal("publication replaced an existing target")
			}
			data, err := os.ReadFile(filepath.Join(directory, "source", "marker"))
			if err != nil || string(data) != "source" {
				t.Fatal("refusal changed source")
			}
			if existing == "empty directory" {
				entries, err := os.ReadDir(filepath.Join(directory, "target"))
				if err != nil || len(entries) != 0 {
					t.Fatal("refusal changed empty target")
				}
			} else {
				name := "target"
				if existing == "populated directory" {
					name = filepath.Join(name, "marker")
				}
				data, err := os.ReadFile(filepath.Join(directory, name))
				if err != nil || string(data) != "target" {
					t.Fatal("refusal changed target")
				}
			}
		})
	}
}

func TestStageDirectoryMigrationRecoversProcessExit(t *testing.T) {
	t.Parallel()
	for _, stop := range []string{"stage-ready", "copy-chunk", "file-published", "copied", "verified"} {
		t.Run(stop, func(t *testing.T) {
			t.Parallel()
			request := directoryMigrationRequestFixture(t)
			content := bytes.Repeat([]byte("retained bytes"), 100000)
			if err := os.WriteFile(filepath.Join(request.SourceDirectory, "catalog-runtime/layers.json"), content, ownerRecordMode); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(filepath.Dir(request.SourceDirectory), "request.json")
			if err := os.WriteFile(input, encoded, ownerRecordMode); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestMigrationStageCrashChild$")
			command.Env = append(os.Environ(), "STARMAP_STAGE_CRASH_INPUT="+input, "STARMAP_STAGE_CRASH_STOP="+stop)
			output, err := command.CombinedOutput()
			var exit *exec.ExitError
			if !stderrors.As(err, &exit) || exit.ExitCode() != 86 {
				t.Fatalf("stage crash = %v: %s", err, output)
			}
			result, err := StageDirectoryMigration(t.Context(), request)
			if err != nil || result.Phase != "verified" {
				t.Fatalf("staging recovery = %+v, %v", result, err)
			}
			for _, directory := range []string{request.SourceDirectory, result.StageDirectory} {
				data, err := os.ReadFile(filepath.Join(directory, "catalog-runtime/layers.json"))
				if err != nil || !bytes.Equal(data, content) {
					t.Fatal("process recovery changed retained bytes")
				}
			}
			if _, err := os.Stat(request.TargetDirectory); !os.IsNotExist(err) {
				t.Fatal("process recovery published the target")
			}
		})
	}
}

func TestMigrationStageCrashChild(t *testing.T) {
	input := os.Getenv("STARMAP_STAGE_CRASH_INPUT")
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
	stop := os.Getenv("STARMAP_STAGE_CRASH_STOP")
	_, err = stageDirectoryMigration(t.Context(), request, func(event, path string) error {
		if event == stop && ((event != "copy-chunk" && event != "file-published") || path == "catalog-runtime/layers.json") {
			os.Exit(86)
		}
		return nil
	})
	t.Fatalf("stage crash point was not reached: %v", err)
}

func TestStageDirectoryMigrationResumesCancelledCopy(t *testing.T) {
	t.Parallel()
	request := directoryMigrationRequestFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, err := stageDirectoryMigration(ctx, request, func(event, _ string) error {
		if event == "copy-chunk" {
			cancel()
			return ctx.Err()
		}
		return nil
	})
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("cancelled copy = %v", err)
	}
	result, err := StageDirectoryMigration(t.Context(), request)
	if err != nil || result.Phase != "verified" {
		t.Fatalf("resumed copy = %+v, %v", result, err)
	}
}

func TestStageDirectoryMigrationDoesNotTruncatePartialLinkTarget(t *testing.T) {
	t.Parallel()
	request := directoryMigrationRequestFixture(t)
	sourcePath := filepath.Join(request.SourceDirectory, "catalog-runtime/layers.json")
	before, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	_, stageErr := stageDirectoryMigration(t.Context(), request, func(event, directory string) error {
		if event == "stage-ready" {
			return os.Link(sourcePath, filepath.Join(directory, filepath.FromSlash(migrationPartialName("catalog-runtime/layers.json"))))
		}
		return nil
	})
	after, err := os.ReadFile(sourcePath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("scratch-file recovery changed its external hard-link target")
	}
	if stageErr != nil {
		t.Fatal(stageErr)
	}
}

func TestStageDirectoryMigrationRefusesChangedState(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"source", "staged file", "extra file", "marker", "staged symlink"} {
		t.Run(change, func(t *testing.T) {
			request := directoryMigrationRequestFixture(t)
			staged, err := StageDirectoryMigration(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(staged.StageDirectory, "catalog-runtime/layers.json")
			switch change {
			case "source":
				path = filepath.Join(request.SourceDirectory, "catalog-runtime/layers.json")
			case "extra file":
				path = filepath.Join(staged.StageDirectory, "unknown-user-file")
			case "marker":
				path = filepath.Join(staged.StageDirectory, migrationPendingName)
			case "staged symlink":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(request.SourceDirectory, "catalog-runtime/layers.json"), path); err != nil {
					t.Fatal(err)
				}
			}
			if change != "staged symlink" {
				if err := os.WriteFile(path, []byte("preserve this evidence"), ownerRecordMode); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := StageDirectoryMigration(t.Context(), request); err == nil {
				t.Fatal("changed migration state was accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("refusal changed the evidence")
			}
		})
	}
}
