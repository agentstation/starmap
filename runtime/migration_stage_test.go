package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	for _, stop := range []string{"initialization-created", "initialization-intent", "initialization-owner", "initialization-publish", "initialization-verify", "initialization-published", "stage-ready", "copy-chunk", "file-published", "partial-removed", "copied", "verified"} {
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
			entries, err := os.ReadDir(filepath.Dir(request.TargetDirectory))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".migration-build-") {
					t.Error("recovery left its initialized stage behind", entry.Name())
				}
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
	var partialPath string
	_, stageErr := stageDirectoryMigration(t.Context(), request, func(event, directory string) error {
		if event == "stage-ready" {
			partialPath = filepath.Join(directory, filepath.FromSlash(migrationPartialName("catalog-runtime/layers.json")))
			return os.Link(sourcePath, partialPath)
		}
		return nil
	})
	after, err := os.ReadFile(sourcePath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("scratch-file recovery changed its external hard-link target")
	}
	if stageErr == nil {
		t.Fatal("migration accepted an unrecorded partial link")
	}
	retained, err := os.ReadFile(partialPath)
	if err != nil || !bytes.Equal(before, retained) {
		t.Fatal("migration removed the unrecorded partial link")
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

func TestMigrationRecoveryPreservesChangedPartial(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"content", "replacement", "unrecorded", "lock", "record"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			request := directoryMigrationRequestFixture(t)
			var directory string
			interrupted := stderrors.New("interrupt partial copy")
			_, err := stageDirectoryMigration(t.Context(), request, func(event, path string) error {
				if event == "stage-ready" {
					directory = path
				}
				if event == "copy-chunk" && path == "catalog-runtime/layers.json" {
					return interrupted
				}
				return nil
			})
			if !stderrors.Is(err, interrupted) {
				t.Fatalf("partial copy interruption = %v", err)
			}
			partial := filepath.Join(directory, filepath.FromSlash(migrationPartialName("catalog-runtime/layers.json")))
			before, err := os.ReadFile(partial)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "content":
				before = []byte("operator content must survive")
				if err := os.WriteFile(partial, before, ownerRecordMode); err != nil {
					t.Fatal(err)
				}
			case "replacement":
				if err := os.Rename(partial, partial+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(partial, before, ownerRecordMode); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(partial + ".original"); err != nil {
					t.Fatal(err)
				}
			case "lock":
				lock := filepath.Join(directory, directoryLockName)
				if err := os.Rename(lock, filepath.Join(filepath.Dir(directory), "retained-stage-lock")); err != nil {
					t.Fatal(err)
				}
			case "record":
				data, err := os.ReadFile(partial + ".json")
				if err != nil {
					t.Fatal(err)
				}
				var record map[string]any
				if err := json.Unmarshal(data, &record); err != nil {
					t.Fatal(err)
				}
				record["version"] = 999
				data, err = json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(partial+".json", data, ownerRecordMode); err != nil {
					t.Fatal(err)
				}
			case "unrecorded":
				if err := os.Remove(partial + ".json"); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
			}
			identity, err := os.Stat(partial)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := StageDirectoryMigration(t.Context(), request); err == nil {
				t.Error("recovery accepted changed or unrecorded partial content")
			}
			after, readErr := os.ReadFile(partial)
			current, statErr := os.Stat(partial)
			if readErr != nil || statErr != nil || !bytes.Equal(before, after) || !os.SameFile(identity, current) {
				t.Errorf("recovery changed preserved partial: read=%v stat=%v", readErr, statErr)
			}
		})
	}
}

func TestMigrationInitializationPreservesUnknownFiles(t *testing.T) {
	t.Parallel()
	request := directoryMigrationRequestFixture(t)
	var evidence string
	interrupted := stderrors.New("interrupt initialization")
	_, err := stageDirectoryMigration(t.Context(), request, func(event, directory string) error {
		if event != "initialization-created" {
			return nil
		}
		evidence = filepath.Join(directory, "operator-evidence")
		if err := os.WriteFile(evidence, []byte("preserve operator data"), ownerRecordMode); err != nil {
			return err
		}
		return interrupted
	})
	if !stderrors.Is(err, interrupted) {
		t.Fatalf("initialization interruption = %v", err)
	}
	for attempt := range 2 {
		if attempt == 1 {
			if _, err := StageDirectoryMigration(t.Context(), request); err == nil {
				t.Error("initialization accepted unknown stage content")
			}
		}
		data, err := os.ReadFile(evidence)
		if err != nil || string(data) != "preserve operator data" {
			t.Fatalf("initialization removed operator evidence: %v", err)
		}
	}
}

func TestMigrationSourceScanBoundsEmptyDirectories(t *testing.T) {
	request := directoryMigrationRequestFixture(t)
	for index := range 40001 {
		if err := os.Mkdir(filepath.Join(request.SourceDirectory, "empty-"+strconv.Itoa(index)), runtimeDirectoryMode); err != nil {
			t.Fatal(err)
		}
	}
	_, err := PrepareDirectoryMigration(t.Context(), request)
	var validation *errors.ValidationError
	if !stderrors.As(err, &validation) || validation.Field != "migration.scan_entries" {
		t.Fatalf("migration scan refusal = %v", err)
	}
}

func TestMigrationInitializationPreservesChangedOwnership(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"directory", "journal-lock", "intent"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			request := directoryMigrationRequestFixture(t)
			prepared, err := PrepareDirectoryMigration(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			var directory string
			interrupted := stderrors.New("interrupt initialized stage")
			_, err = stageDirectoryMigration(t.Context(), request, func(event, path string) error {
				if event == "initialization-intent" {
					directory = path
					return interrupted
				}
				return nil
			})
			if !stderrors.Is(err, interrupted) {
				t.Fatalf("initialization interruption = %v", err)
			}
			switch change {
			case "directory":
				if err := os.Rename(directory, directory+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(directory, runtimeDirectoryMode); err != nil {
					t.Fatal(err)
				}
			case "journal-lock":
				lock := filepath.Join(prepared.JournalDirectory, directoryLockName)
				if err := os.Rename(lock, filepath.Join(filepath.Dir(directory), "retained-journal-lock")); err != nil {
					t.Fatal(err)
				}
			case "intent":
				if err := os.WriteFile(filepath.Join(directory, migrationPendingName), []byte("preserve changed intent"), ownerRecordMode); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Stat(directory)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := StageDirectoryMigration(t.Context(), request); err == nil {
				t.Error("initialization accepted changed ownership")
			}
			after, err := os.Stat(directory)
			if err != nil || !os.SameFile(before, after) {
				t.Fatal("initialization changed the preserved directory", err)
			}
			if change == "intent" {
				data, err := os.ReadFile(filepath.Join(directory, migrationPendingName))
				if err != nil || string(data) != "preserve changed intent" {
					t.Fatal("initialization changed preserved intent", err)
				}
			}
		})
	}
}

func TestMigrationInitializationPreservesReusedStageName(t *testing.T) {
	t.Parallel()
	request := directoryMigrationRequestFixture(t)
	var original string
	result, err := stageDirectoryMigration(t.Context(), request, func(event, path string) error {
		if event == "initialization-created" {
			original = path
		}
		if event == "initialization-published" {
			if err := os.Mkdir(original, runtimeDirectoryMode); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(original, "operator-content"), []byte("preserve reused path"), ownerRecordMode)
		}
		return nil
	})
	if err != nil || result.Phase != "verified" {
		t.Fatalf("stage publication = %+v, %v", result, err)
	}
	data, err := os.ReadFile(filepath.Join(original, "operator-content"))
	if err != nil || string(data) != "preserve reused path" {
		t.Fatal("publication changed a reused stage path", err)
	}
}

func TestMigrationInitializationExcludesActiveWriter(t *testing.T) {
	t.Parallel()
	request := directoryMigrationRequestFixture(t)
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := stageDirectoryMigration(t.Context(), request, func(event, _ string) error {
			if event == "initialization-created" {
				close(started)
				select {
				case <-release:
				case <-t.Context().Done():
					return t.Context().Err()
				}
			}
			return nil
		})
		done <- err
	}()
	select {
	case <-started:
	case err := <-done:
		t.Fatal("writer did not reach initialization", err)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	_, err := StageDirectoryMigration(t.Context(), request)
	close(release)
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Error("second writer did not receive a conflict", err)
	}
	if err := <-done; err != nil {
		t.Fatal("original writer did not complete", err)
	}
}

func TestMigrationInitializationPreservesMovedStage(t *testing.T) {
	t.Parallel()
	request := directoryMigrationRequestFixture(t)
	var retained string
	_, err := stageDirectoryMigration(t.Context(), request, func(event, path string) error {
		if event == "initialization-verify" {
			retained = path + "-retained"
			return os.Rename(path, retained)
		}
		return nil
	})
	if !stderrors.Is(err, os.ErrNotExist) {
		t.Fatalf("moved initialization stage = %v", err)
	}
	if _, err := os.Stat(filepath.Join(retained, migrationPendingName)); err != nil {
		t.Fatal("initialization did not preserve the moved stage", err)
	}
	if _, err := os.Stat(request.TargetDirectory); !os.IsNotExist(err) {
		t.Fatal("failed initialization published the target", err)
	}
}
