package runtime

import (
	"context"
	stderrors "errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type exitAfterRuntimeRecordStage struct {
	context.Context
	directory string
}

func (c exitAfterRuntimeRecordStage) Err() error {
	for _, relative := range []string{layerDirectoryName, layerDirectoryName + "/providers", layerDirectoryName + "/providers/bindings", layerDirectoryName + "/publication-inputs"} {
		entries, _ := os.ReadDir(filepath.Join(c.directory, relative))
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".layer-") || strings.HasPrefix(entry.Name(), ".input-") {
				info, err := entry.Info()
				if err == nil && info.Size() > 0 {
					os.Exit(88)
				}
			}
		}
	}
	return c.Context.Err()
}

func TestRuntimeRecoversRecordPublicationAfterProcessExit(t *testing.T) {
	if path := os.Getenv("STARMAP_TEST_RUNTIME_RECORD_EXIT"); path != "" {
		var store *layerStore
		var err error
		if os.Getenv("STARMAP_TEST_RUNTIME_RECORD_LEGACY") == "1" {
			store, err = newLayerStore(path)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			owner := openTestRuntime(t, WithStateDirectory(path), WithCatalogSource("embedded"), WithSourceRefreshMode("manual"))
			store = owner.store
		}
		ctx := exitAfterRuntimeRecordStage{Context: t.Context(), directory: path}
		at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
		switch os.Getenv("STARMAP_TEST_RUNTIME_RECORD_KIND") {
		case "source":
			payload := testCatalogPayload(t, "source", "model", "Model")
			err = store.saveSource(ctx, sourceLayer{Identity: "test-source", GenerationID: "candidate", Payload: payload, Checksum: catalogs.DescribeCatalogPayload(payload).Checksum, PublishedAt: at, ObservedAt: at})
		case "provider":
			err = store.saveProvider(ctx, testProviderLayer(t, "provider", "model", "Model", at))
		case "binding":
			err = store.saveProvider(ctx, scopedProviderLayer(t, "account-a", "1", at))
		case "input":
			_, err = store.stageInput(ctx, map[string]string{"generation": "candidate"})
		case "pin":
			err = store.savePinRecord(ctx, generationPinRecord{Version: generationPinRecordVersion, Binding: strings.Repeat("0", 64), Phase: pinPrepared, Receipt: GenerationPinAcceptance{OperationID: "operation", SelectedGenerationID: "candidate", AcceptedGenerationID: "candidate", PayloadChecksum: "sha256:" + strings.Repeat("0", 64), RequestedAt: at}})
		default:
			t.Fatal("unknown child record")
		}
		t.Fatalf("writer did not exit: %v", err)
	}
	for _, kind := range []string{"source", "provider", "binding", "input", "pin"} {
		t.Run(kind, func(t *testing.T) {
			directory := privateRuntimeDirectory(t)
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRuntimeRecoversRecordPublicationAfterProcessExit$")
			command.Env = append(os.Environ(), "STARMAP_TEST_RUNTIME_RECORD_EXIT="+directory, "STARMAP_TEST_RUNTIME_RECORD_KIND="+kind)
			output, err := command.CombinedOutput()
			if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
				t.Fatalf("child exit: %v %s", err, output)
			}
			runtime := openTestRuntime(t, WithStateDirectory(directory), WithCatalogSource("embedded"), WithSourceRefreshMode("manual"))
			if !runtime.Status().Usable || runtime.Catalog() == nil {
				t.Fatal("recovery lost the embedded baseline")
			}
			for _, relative := range []string{layerDirectoryName, layerDirectoryName + "/providers", layerDirectoryName + "/providers/bindings", layerDirectoryName + "/publication-inputs"} {
				entries, err := os.ReadDir(filepath.Join(directory, relative))
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), ".layer-") || strings.HasPrefix(entry.Name(), ".input-") {
						t.Errorf("startup retained abandoned record %s/%s", relative, entry.Name())
					}
				}
			}
		})
	}
}

func TestRuntimeMigrationPreservesPendingRecordReceipts(t *testing.T) {
	request := directoryMigrationRequestFixture(t)
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRuntimeRecoversRecordPublicationAfterProcessExit$")
	command.Env = append(os.Environ(), "STARMAP_TEST_RUNTIME_RECORD_EXIT="+request.SourceDirectory, "STARMAP_TEST_RUNTIME_RECORD_KIND=input", "STARMAP_TEST_RUNTIME_RECORD_LEGACY=1")
	output, err := command.CombinedOutput()
	if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
		t.Fatalf("child exit: %v %s", err, output)
	}
	before := map[string][]byte{}
	err = filepath.WalkDir(request.SourceDirectory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		before[path] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = PrepareDirectoryMigration(t.Context(), request)
	var conflict *pkgerrors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("migration adopted pending native receipts: %v", err)
	}
	for path, data := range before {
		current, err := os.ReadFile(path)
		if err != nil || string(current) != string(data) {
			t.Fatalf("migration changed source %s: %v", path, err)
		}
	}
	inputs, err := privatefiles.ExistingDirectory(filepath.Join(request.SourceDirectory, layerDirectoryName, inputPublicationDirectory))
	if err != nil {
		t.Fatal(err)
	}
	if err := inputs.RecoverPublications(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	stage, err := StageDirectoryMigration(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	copied, err := newLayerStore(stage.StageDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if err := copied.recoverRecordPublications(t.Context()); err != nil {
		t.Fatalf("copied inactive metadata cannot reopen: %v", err)
	}
}
