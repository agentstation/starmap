package runtime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationJournalRecoversEveryPartialEventBoundary(t *testing.T) {
	t.Parallel()
	for _, phase := range migrationPhases() {
		t.Run(string(phase), func(t *testing.T) {
			root, manifest := migrationJournalFixture(t)
			journal, err := openDirectoryMigrationJournal(t.Context(), root, manifest)
			if err != nil {
				t.Fatal(err)
			}
			for _, next := range migrationPhases()[1:] {
				if journal.phase == phase {
					break
				}
				if err := journal.advance(t.Context(), next); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(journal.directory, migrationJournalName)
			if err := journal.Close(); err != nil {
				t.Fatal(err)
			}
			complete, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			start := bytes.LastIndexByte(complete[:len(complete)-1], '\n') + 1
			line := complete[start:]
			for length := 1; length < len(line); length++ {
				tail := line[:length]
				if err := os.WriteFile(path, complete[:start+length], ownerRecordMode); err != nil {
					t.Fatal(err)
				}
				resumed, err := openDirectoryMigrationJournal(t.Context(), root, manifest)
				if err != nil {
					t.Fatalf("resume at byte %d: %v", length, err)
				}
				if err := resumed.advance(t.Context(), phase); err != nil {
					t.Fatal(err)
				}
				if err := resumed.Close(); err != nil {
					t.Fatal(err)
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(after, complete) {
					t.Fatalf("resume changed the complete chain at byte %d", length)
				}
				digest := sha256.Sum256(tail)
				saved, err := os.ReadFile(filepath.Join(filepath.Dir(path), "journal.partial-"+hex.EncodeToString(digest[:])))
				if err != nil || !bytes.Equal(saved, tail) {
					t.Fatalf("resume lost the partial write at byte %d", length)
				}
			}
		})
	}
}

func TestMigrationJournalRefusesCorruptionWithoutRepair(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"garbage tail", "wrong chain", "duplicate event", "extra event", "symlink", "archive conflict"} {
		t.Run(change, func(t *testing.T) {
			root, manifest := migrationJournalFixture(t)
			journal, err := openDirectoryMigrationJournal(t.Context(), root, manifest)
			if err != nil {
				t.Fatal(err)
			}
			if change == "extra event" {
				for _, phase := range migrationPhases()[1:] {
					if err := journal.advance(t.Context(), phase); err != nil {
						t.Fatal(err)
					}
				}
			}
			next, err := journal.nextEvent(migrationCopied)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(journal.directory, migrationJournalName)
			if err := journal.Close(); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "garbage tail":
				before = append(before, "not an event"...)
			case "wrong chain":
				before = bytes.Replace(before, []byte(`"sequence":1`), []byte(`"sequence":9`), 1)
			case "duplicate event", "extra event":
				before = append(before, before...)
			case "archive conflict":
				tail := next[:len(next)/2]
				digest := sha256.Sum256(tail)
				if err := os.WriteFile(filepath.Join(journal.directory, "journal.partial-"+hex.EncodeToString(digest[:])), []byte("different"), ownerRecordMode); err != nil {
					t.Fatal(err)
				}
				before = append(before, tail...)
			case "symlink":
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".original", path); err != nil {
					t.Fatal(err)
				}
			}
			if change != "symlink" {
				if err := os.WriteFile(path, before, ownerRecordMode); err != nil {
					t.Fatal(err)
				}
			}
			if resumed, err := openDirectoryMigrationJournal(t.Context(), root, manifest); err == nil {
				_ = resumed.Close()
				t.Fatal("corrupt journal was accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("refusal changed the journal")
			}
		})
	}
}

func TestMigrationJournalRefusesChangesWhileOpen(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"removed", "truncated", "same length", "symlink", "manifest"} {
		t.Run(change, func(t *testing.T) {
			root, manifest := migrationJournalFixture(t)
			journal, err := openDirectoryMigrationJournal(t.Context(), root, manifest)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = journal.Close() }()
			path := filepath.Join(journal.directory, migrationJournalName)
			if change == "manifest" {
				path = filepath.Join(journal.directory, migrationManifestName)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "removed":
				err = os.Remove(path)
			case "truncated":
				before = before[:len(before)/2]
				err = os.WriteFile(path, before, ownerRecordMode)
			case "same length", "manifest":
				before[0] = '!'
				err = os.WriteFile(path, before, ownerRecordMode)
			case "symlink":
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(path+".original", path)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := journal.advance(t.Context(), migrationCopied); err == nil {
				t.Fatal("journal appended after its accepted input changed")
			}
			after, err := os.ReadFile(path)
			if change == "removed" {
				if !os.IsNotExist(err) {
					t.Fatal("journal recreated a removed input")
				}
			} else if err != nil || !bytes.Equal(before, after) {
				t.Fatal("journal changed the evidence after refusal")
			}
		})
	}
}
