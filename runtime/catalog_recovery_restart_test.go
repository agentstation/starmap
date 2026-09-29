package runtime

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type exitAfterCatalogRecoveryStage struct {
	context.Context
	directory, prefix string
}

func (c exitAfterCatalogRecoveryStage) Err() error {
	for _, subdirectory := range []string{generationBaselinesDirectory, generationInputsDirectory} {
		entries, _ := os.ReadDir(filepath.Join(c.directory, layerDirectoryName, subdirectory))
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), c.prefix) {
				info, err := entry.Info()
				if err == nil && info.Size() > 0 {
					os.Exit(88)
				}
			}
		}
	}
	return c.Context.Err()
}

func TestCatalogRecoveryResumesInterruptedOwnerPublication(t *testing.T) {
	if directory := os.Getenv("STARMAP_TEST_CATALOG_RECOVERY_EXIT"); directory != "" {
		snapshot, _ := fleetValidationFixture(t)
		store, err := newLayerStore(directory)
		if err != nil {
			t.Fatal(err)
		}
		attempt := localRecoveryTestAttempt(t, store, snapshot.Publication.Recovery.Data)
		ctx := context.WithValue(t.Context(), localRecoveryContextKey{}, attempt)
		crash := exitAfterCatalogRecoveryStage{Context: ctx, directory: directory, prefix: os.Getenv("STARMAP_TEST_CATALOG_RECOVERY_STAGE")}
		err = retainLocalGeneration(crash, snapshot.Publication.Generation)
		t.Fatalf("writer did not exit after its stage: %v", err)
	}
	for _, stage := range []string{".baseline-", ".generation-input-"} {
		t.Run(stage, func(t *testing.T) {
			directory := privateRuntimeDirectory(t)
			command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestCatalogRecoveryResumesInterruptedOwnerPublication$")
			command.Env = append(os.Environ(), "STARMAP_TEST_CATALOG_RECOVERY_EXIT="+directory, "STARMAP_TEST_CATALOG_RECOVERY_STAGE="+stage)
			output, err := command.CombinedOutput()
			if command.ProcessState == nil || command.ProcessState.ExitCode() != 88 {
				t.Fatalf("child exit: %v %s", err, output)
			}
			store, err := newLayerStore(directory)
			if err != nil {
				t.Fatal(err)
			}
			if err = store.inspectCatalogRecovery(t.Context()); err == nil {
				t.Fatal("inspection accepted an interrupted record publication")
			}
			if err = store.recoverRecordPublications(t.Context()); err != nil {
				t.Fatal(err)
			}
			snapshot, _ := fleetValidationFixture(t)
			attempt := localRecoveryTestAttempt(t, store, snapshot.Publication.Recovery.Data)
			ctx := context.WithValue(t.Context(), localRecoveryContextKey{}, attempt)
			for range 2 {
				if err = retainLocalGeneration(ctx, snapshot.Publication.Generation); err != nil {
					t.Fatal(err)
				}
			}
			inventory, err := store.captureCatalogRecovery(t.Context(), storage.DefaultRetentionScanEntries, storage.DefaultRetentionInputMaxBytes)
			if err != nil || len(inventory.baseline) != 1 || len(inventory.records) != 1 {
				t.Fatalf("recovered inventory: baselines=%d inputs=%d err=%v", len(inventory.baseline), len(inventory.records), err)
			}
			if err = inventory.records[0].record.validate(snapshot.Publication.Generation); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCatalogRecoveryRefusesMissingBaselineAndBoundedCensus(t *testing.T) {
	r := smallCatalogRecoveryRuntime(t, storage.NewMemory())
	inventory, err := r.store.captureCatalogRecovery(t.Context(), storage.DefaultRetentionScanEntries, storage.DefaultRetentionInputMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.store.captureCatalogRecovery(t.Context(), 1, storage.DefaultRetentionInputMaxBytes); err == nil {
		t.Fatal("entry bound accepted an incomplete census")
	}
	if _, err = r.store.captureCatalogRecovery(t.Context(), storage.DefaultRetentionScanEntries, inventory.bytes-1); err == nil {
		t.Fatal("byte bound accepted an incomplete census")
	}
	record := inventory.records[0].record
	baseline := inventory.baseline[record.BaselineChecksum]
	path := filepath.Join(r.config.stateDirectory, layerDirectoryName, generationBaselinesDirectory, record.BaselineChecksum+".json.gz")
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = r.store.inspectCatalogRecovery(t.Context()); err == nil {
		t.Fatal("missing baseline accepted")
	}
	corrupt := bytes.Clone(baseline)
	corrupt[0] ^= 1
	if err = os.WriteFile(path, corrupt, ownerRecordMode); err != nil {
		t.Fatal(err)
	}
	if err = r.store.inspectCatalogRecovery(t.Context()); err == nil {
		t.Fatal("corrupt baseline accepted")
	}
	if err = os.WriteFile(path, baseline, ownerRecordMode); err != nil {
		t.Fatal(err)
	}
	if err = r.store.inspectCatalogRecovery(t.Context()); err != nil {
		t.Fatal(err)
	}
}
