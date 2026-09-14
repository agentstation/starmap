package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func relocationFixture(t *testing.T, phase string) (string, string, catalogs.Generation) {
	t.Helper()
	root := t.TempDir()
	legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
	generation := migrationGeneration(t, "relocation-boundary", "model", "Model")
	if err := migrationStore(t, legacy).Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	runLegacyRelocationExit(t, legacy, state, phase)
	return legacy, state, generation
}

func relocationJournalPath(t *testing.T, legacy string) string {
	t.Helper()
	names, err := preparationNames(t.Context(), legacy)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		path := filepath.Join(filepath.Dir(legacy), name, preparationJournalName)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(`"relocation":`)) {
			return path
		}
	}
	t.Fatal("missing relocation journal")
	return ""
}

func TestLegacyRelocationRecoveryPreservesChangedState(t *testing.T) {
	for _, change := range []string{"store-root", "store-content", "generation-identity", "unknown-generation", "workspace-content", "workspace-unknown", "workspace-root", "state-parent", "writer-identity", "journal-identity", "journal-truncated", "inventory-incomplete", "unknown-stage", "wrong-destination"} {
		t.Run(change, func(t *testing.T) {
			legacy, state, generation := relocationFixture(t, "installed")
			journal := relocationJournalPath(t, legacy)
			root := filepath.Dir(legacy)
			write := func(path string, data []byte) {
				t.Helper()
				if err := os.WriteFile(path, data, fileMode); err != nil {
					t.Fatal(err)
				}
			}
			replace := func(path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				write(path, data)
			}
			switch change {
			case "store-root":
				if err := os.Rename(state, state+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(state, directoryMode); err != nil {
					t.Fatal(err)
				}
				write(filepath.Join(state, "operator-file"), []byte("keep"))
			case "store-content":
				entries, err := os.ReadDir(filepath.Join(state, "generations"))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(state, "generations", entries[0].Name(), "catalog.json")
				file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := file.WriteString("\n"); err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			case "generation-identity":
				entries, err := os.ReadDir(filepath.Join(state, "generations"))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(state, "generations", entries[0].Name(), "catalog.json")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(path, filepath.Join(root, "original-payload")); err != nil {
					t.Fatal(err)
				}
				write(path, data)
			case "unknown-generation":
				next := migrationGeneration(t, "unrecorded-update", "later", "Later")
				if err := migrationStore(t, state).Commit(t.Context(), next, generation.Manifest.GenerationID); err != nil {
					t.Fatal(err)
				}
			case "workspace-content":
				write(filepath.Join(legacy, "providers.yaml"), []byte("operator edit"))
			case "workspace-unknown":
				write(filepath.Join(legacy, "operator-note"), []byte("keep"))
			case "workspace-root":
				if err := os.Rename(legacy, legacy+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.CopyFS(legacy, os.DirFS(legacy+".original")); err != nil {
					t.Fatal(err)
				}
			case "state-parent":
				parent := filepath.Dir(state)
				if err := os.Rename(parent, parent+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(parent, directoryMode); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(parent+".original", "catalog"), state); err != nil {
					t.Fatal(err)
				}
			case "writer-identity":
				replace(writerLockPath(legacy))
			case "journal-identity":
				replace(journal)
			case "journal-truncated":
				data, err := os.ReadFile(journal)
				if err != nil {
					t.Fatal(err)
				}
				write(journal, data[:len(data)-1])
			case "inventory-incomplete":
				data, err := os.ReadFile(journal)
				if err != nil {
					t.Fatal(err)
				}
				lines := bytes.Split(data, []byte{'\n'})
				write(journal, append(bytes.Join(lines[:2], []byte{'\n'}), '\n'))
			case "unknown-stage":
				write(filepath.Join(filepath.Dir(journal), "operator-note"), []byte("keep"))
			case "wrong-destination":
				state = filepath.Join(root, "another-state")
			}
			before := migrationTree(t, root)
			if _, err := MigrateLegacyLayout(t.Context(), legacy, state); err == nil {
				t.Fatal("recovery accepted changed state")
			}
			if after := migrationTree(t, root); !reflect.DeepEqual(before, after) {
				t.Fatal("recovery changed rejected state")
			}
		})
	}
}

func TestLegacyRelocationRecoveryExcludesActiveWriter(t *testing.T) {
	legacy, state, _ := relocationFixture(t, "moved")
	release, err := acquireWriterLock(legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	before := migrationTree(t, filepath.Dir(legacy))
	_, err = MigrateLegacyLayout(t.Context(), legacy, state)
	var conflict *errors.ConflictError
	if !stderrors.As(err, &conflict) {
		t.Fatalf("active writer result: %v", err)
	}
	if after := migrationTree(t, filepath.Dir(legacy)); !reflect.DeepEqual(before, after) {
		t.Fatal("active writer state changed")
	}
}

func TestLegacyRelocationRecoveryRetainsCanceledState(t *testing.T) {
	legacy, state, _ := relocationFixture(t, "moved")
	before := migrationTree(t, filepath.Dir(legacy))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := MigrateLegacyLayout(ctx, legacy, state)
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled recovery: %v", err)
	}
	if after := migrationTree(t, filepath.Dir(legacy)); !reflect.DeepEqual(before, after) {
		t.Fatal("canceled recovery changed state")
	}
}

func TestLegacyRelocationRecoveryAfterPartialRollback(t *testing.T) {
	for _, phase := range []string{"partial-workspace", "restored-store"} {
		t.Run(phase, func(t *testing.T) {
			legacy, state, generation := relocationFixture(t, "installed")
			if phase == "partial-workspace" {
				if err := os.Remove(filepath.Join(legacy, "providers.yaml")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.RemoveAll(legacy); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(state, legacy); err != nil {
					t.Fatal(err)
				}
			}
			result, err := MigrateLegacyLayout(t.Context(), legacy, state)
			if err != nil || result.GenerationID != generation.Manifest.GenerationID {
				t.Fatalf("resume partial rollback: %+v, %v", result, err)
			}
			assertWorkspaceModel(t, legacy, "model", "Model")
			assertNoProjectionStaging(t, legacy)
		})
	}
}

func TestLegacyRelocationRequiresExplicitRecovery(t *testing.T) {
	legacy, _, generation := relocationFixture(t, "installed")
	catalog, err := catalogs.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		t.Fatal(err)
	}
	before := migrationTree(t, filepath.Dir(legacy))
	_, err = Project(t.Context(), legacy, catalog, Identity{GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum})
	if err == nil {
		t.Fatal("ordinary projection accepted pending relocation")
	}
	if after := migrationTree(t, filepath.Dir(legacy)); !reflect.DeepEqual(before, after) {
		t.Fatal("ordinary projection changed relocation state")
	}
}

func TestRelocationJournalRejectsOlderVersion(t *testing.T) {
	legacy, _, _ := relocationFixture(t, "moved")
	path := relocationJournalPath(t, legacy)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(data, []byte{'\n'})
	var event preparationEvent
	if err := json.Unmarshal(lines[0], &event); err != nil {
		t.Fatal(err)
	}
	event.Header.Version = 3
	lines[0], err = json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Join(lines, []byte{'\n'})
	if err := os.WriteFile(path, data, fileMode); err != nil {
		t.Fatal(err)
	}
	writer, err := acquireWorkspaceWriter(legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.close()
	stage, err := readPreparation(t.Context(), legacy, filepath.Base(filepath.Dir(path)), writer)
	if stage != nil {
		stage.releaseHandles()
	}
	if err == nil {
		t.Fatal("version 3 accepted relocation events")
	}
}

func TestLegacyRelocationRecoveryBoundsParentScan(t *testing.T) {
	legacy, state, _ := relocationFixture(t, "moved")
	root := filepath.Dir(legacy)
	for i := range preparationScanMax {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("unrelated-%04d", i)), nil, fileMode); err != nil {
			t.Fatal(err)
		}
	}
	before := migrationTree(t, root)
	_, err := MigrateLegacyLayout(t.Context(), legacy, state)
	var limit *errors.ValidationError
	if !stderrors.As(err, &limit) {
		t.Fatalf("scan limit result: %v", err)
	}
	if after := migrationTree(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("scan limit changed pending relocation")
	}
}

func TestLegacyRelocationRecoveryExcludesCommitWriter(t *testing.T) {
	legacy, state, _ := relocationFixture(t, "moved")
	release, err := acquireLegacyStoreLock(t.Context(), state)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	before := migrationTree(t, filepath.Dir(legacy))
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err = MigrateLegacyLayout(ctx, legacy, state)
	if !stderrors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("active commit writer result: %v", err)
	}
	if after := migrationTree(t, filepath.Dir(legacy)); !reflect.DeepEqual(before, after) {
		t.Fatal("active commit writer state changed")
	}
}

func TestLegacyRelocationRecoveryChecksLockAlias(t *testing.T) {
	for _, change := range []string{"retained", "missing", "replaced", "modified"} {
		t.Run(change, func(t *testing.T) {
			legacy, state, _ := relocationFixture(t, "moved")
			journal := relocationJournalPath(t, legacy)
			data, err := os.ReadFile(journal)
			if err != nil {
				t.Fatal(err)
			}
			lines := bytes.Split(data, []byte{'\n'})
			var event preparationEvent
			if err := json.Unmarshal(lines[1], &event); err != nil {
				t.Fatal(err)
			}
			parent, err := os.OpenRoot(filepath.Dir(legacy))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = parent.Close() }()
			record := event.Relocation
			if record.Alias == nil {
				name := ".catalog.starmap-migration-lock-test"
				if err := os.Link(filepath.Join(state, ".commit.lock"), filepath.Join(parent.Name(), name)); err != nil {
					t.Fatal(err)
				}
				receipt, err := optionalWorkspaceRecord(t.Context(), parent, name)
				if err != nil {
					t.Fatal(err)
				}
				record.Alias = &relocationFile{Entry: receipt.entry, Identity: receipt.identity}
				lines[1], err = json.Marshal(event)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(journal, bytes.Join(lines, []byte{'\n'}), fileMode); err != nil {
					t.Fatal(err)
				}
			}
			alias := filepath.Join(parent.Name(), record.Alias.Entry.Path)
			switch change {
			case "missing":
				if err := os.Remove(alias); err != nil {
					t.Fatal(err)
				}
			case "replaced":
				if err := os.Rename(alias, alias+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(alias, nil, fileMode); err != nil {
					t.Fatal(err)
				}
			case "modified":
				if err := os.WriteFile(alias, []byte("operator lock content"), fileMode); err != nil {
					t.Fatal(err)
				}
			}
			before := migrationTree(t, parent.Name())
			_, err = MigrateLegacyLayout(t.Context(), legacy, state)
			if change == "replaced" || change == "modified" {
				if err == nil {
					t.Fatal("recovery accepted changed alias")
				}
				if after := migrationTree(t, parent.Name()); !reflect.DeepEqual(before, after) {
					t.Fatal("recovery changed rejected alias state")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(alias); !os.IsNotExist(err) {
				t.Fatalf("recovery retained alias: %v", err)
			}
			assertWorkspaceModel(t, legacy, "model", "Model")
			assertNoProjectionStaging(t, legacy)
		})
	}
}

func TestLegacyRelocationChecksJournalBeforeWorkspacePublication(t *testing.T) {
	root := t.TempDir()
	legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
	generation := migrationGeneration(t, "relocation-publication", "model", "Model")
	if err := migrationStore(t, legacy).Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	m := legacyLayoutMigrator{projector: projector{beforePromote: func() error {
		path := relocationJournalPath(t, legacy)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, append([]byte{' '}, data...), fileMode)
	}}}
	if _, err := m.migrate(t.Context(), legacy, state); err == nil {
		t.Fatal("publication accepted changed relocation journal")
	}
	if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
		t.Fatalf("workspace was published: %v", err)
	}
	current, err := migrationStore(t, state).Current(t.Context())
	if err != nil || !sameMigrationGeneration(generation, current) {
		t.Fatalf("relocated catalog changed: %v", err)
	}
}

func TestRelocationPreparationClosesHandlesAfterCleanupConflict(t *testing.T) {
	root := t.TempDir()
	legacy, state := filepath.Join(root, "catalog"), filepath.Join(root, "state", "catalog")
	generation := migrationGeneration(t, "relocation-open-handles", "model", "Model")
	if err := migrationStore(t, legacy).Commit(t.Context(), generation, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Dir(state), directoryMode); err != nil {
		t.Fatal(err)
	}
	lease, err := acquireLegacyStoreLease(t.Context(), legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.close()
	writer, err := acquireWorkspaceWriter(legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.close()
	fault := stderrors.New("stop after enclosure creation")
	var captured *workspaceStage
	stage, err := prepareRelocation(t.Context(), legacy, state, generation, 1, writer, lease, func(s *workspaceStage) error {
		captured = s
		if err := os.WriteFile(filepath.Join(root, s.name, "operator-file"), []byte("keep"), fileMode); err != nil {
			return err
		}
		return fault
	})
	if !stderrors.Is(err, fault) || stage != nil || captured == nil {
		t.Fatalf("preparation failure: %v", err)
	}
	var probe [1]byte
	if _, err := captured.journal.file.Read(probe[:]); !stderrors.Is(err, os.ErrClosed) {
		t.Fatalf("journal handle retained: %v", err)
	}
	if _, err := captured.private.Stat("."); !stderrors.Is(err, os.ErrClosed) {
		t.Fatalf("enclosure handle retained: %v", err)
	}
	if _, err := captured.parent.Stat("."); !stderrors.Is(err, os.ErrClosed) {
		t.Fatalf("parent handle retained: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, captured.name, "operator-file"))
	if err != nil || string(data) != "keep" {
		t.Fatalf("operator file changed: %q, %v", data, err)
	}
}
