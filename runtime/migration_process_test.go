package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestMigrationJournalRecoversAfterProcessExit(t *testing.T) {
	t.Parallel()
	for _, stop := range []string{"manifest", "prepared", "copied", "verified", "promoted", "completed", "partial", "archived"} {
		t.Run(stop, func(t *testing.T) {
			t.Parallel()
			root, manifest := migrationJournalFixture(t)
			encoded, err := manifest.encode()
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(filepath.Dir(root), "intent.json")
			if err := os.WriteFile(input, encoded, ownerRecordMode); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestMigrationJournalCrashChild$")
			command.Env = append(os.Environ(), "STARMAP_MIGRATION_CRASH_INPUT="+input, "STARMAP_MIGRATION_CRASH_STOP="+stop)
			output, err := command.CombinedOutput()
			var exit *exec.ExitError
			if !stderrors.As(err, &exit) || exit.ExitCode() != 86 {
				t.Fatalf("journal crash exit = %v: %s", err, output)
			}
			journal, err := openDirectoryMigrationJournal(t.Context(), root, manifest)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = journal.Close() }()
			want := migrationPhase(stop)
			if stop == "manifest" || stop == "partial" || stop == "archived" {
				want = migrationPrepared
			}
			if journal.phase != want {
				t.Fatalf("resumed phase = %q, want %q", journal.phase, want)
			}
			for _, phase := range migrationPhases()[journal.sequence:] {
				if err := journal.advance(t.Context(), phase); err != nil {
					t.Fatal(err)
				}
			}
			actual, err := os.ReadFile(filepath.Join(journal.directory, migrationManifestName))
			if err != nil || !bytes.Equal(actual, encoded) {
				t.Fatal("process recovery changed migration intent")
			}
			for _, path := range []string{manifest.SourceDirectory, manifest.TargetDirectory} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("journal recovery changed runtime directories")
				}
			}
		})
	}
}

func TestMigrationJournalCrashChild(t *testing.T) {
	input := os.Getenv("STARMAP_MIGRATION_CRASH_INPUT")
	if input == "" {
		return
	}
	encoded, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	var manifest directoryMigrationManifest
	if err := json.Unmarshal(encoded, &manifest); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(filepath.Dir(input), "journals")
	stop := os.Getenv("STARMAP_MIGRATION_CRASH_STOP")
	if stop == "manifest" {
		digest := sha256.Sum256([]byte(manifest.OperationID))
		directory := filepath.Join(root, hex.EncodeToString(digest[:]))
		lock, err := acquireDirectory(t.Context(), directory)
		if err != nil || lock == nil {
			t.Fatalf("journal lock = %v", err)
		}
		anchored, err := os.OpenRoot(directory)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeOwnerFile(t.Context(), anchored, migrationManifestName, encoded); err != nil {
			t.Fatal(err)
		}
		if err := syncMigrationDirectory(anchored); err != nil {
			t.Fatal(err)
		}
		os.Exit(86)
	}
	journal, err := openDirectoryMigrationJournal(t.Context(), root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if stop == "partial" || stop == "archived" {
		next, err := journal.nextEvent(migrationCopied)
		if err != nil {
			t.Fatal(err)
		}
		tail := next[:len(next)/2]
		file, err := journal.root.OpenFile(migrationJournalName, os.O_APPEND|os.O_WRONLY, ownerRecordMode)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(tail); err != nil {
			t.Fatal(err)
		}
		if err := file.Sync(); err != nil {
			t.Fatal(err)
		}
		if stop == "archived" {
			digest := sha256.Sum256(tail)
			if err := writeOwnerFile(t.Context(), journal.root, "journal.partial-"+hex.EncodeToString(digest[:]), tail); err != nil {
				t.Fatal(err)
			}
			if err := syncMigrationDirectory(journal.root); err != nil {
				t.Fatal(err)
			}
		}
		os.Exit(86)
	}
	for _, phase := range migrationPhases()[1:] {
		if journal.phase == migrationPhase(stop) {
			break
		}
		if err := journal.advance(t.Context(), phase); err != nil {
			t.Fatal(err)
		}
	}
	os.Exit(86)
}

func TestMigrationJournalConcurrentIdempotentAdvance(t *testing.T) {
	t.Parallel()
	root, manifest := migrationJournalFixture(t)
	journal, err := openDirectoryMigrationJournal(t.Context(), root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = journal.Close() }()
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			if err := journal.advance(t.Context(), migrationCopied); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	data, err := os.ReadFile(filepath.Join(journal.directory, migrationJournalName))
	if err != nil || bytes.Count(data, []byte{'\n'}) != 2 {
		t.Fatal("concurrent retries duplicated a journal event")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := journal.advance(ctx, migrationVerified); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("cancelled advance = %v", err)
	}
	after, err := os.ReadFile(filepath.Join(journal.directory, migrationJournalName))
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("cancelled advance changed the journal")
	}
	if err := journal.Close(); err != nil {
		t.Fatal(err)
	}
	if err := journal.advance(t.Context(), migrationVerified); err == nil {
		t.Fatal("closed journal advanced")
	}
}
