package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func retainedBaselineInputs(t *testing.T, directory string) (map[string]BaselineRetainedFile, BaselineRecordReader) {
	t.Helper()
	files := make(map[string]BaselineRetainedFile)
	bodies := make(map[string][]byte)
	err := filepath.WalkDir(directory, func(name string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(directory, name)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		sum := sha256.Sum256(body)
		files[relative] = BaselineRetainedFile{Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:])}
		bodies[relative] = body
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files, func(_ context.Context, name string, limit int64) ([]byte, error) {
		if limit != baselineJournalLimit {
			t.Fatalf("unexpected read bound %d", limit)
		}
		body, ok := bodies[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return body, nil
	}
}

func TestRetainedBaselinePublicationsKeepStagingInactive(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "baseline")
	stage, _, _ := recordedBaselineStage(t, directory)
	files, read := retainedBaselineInputs(t, directory)
	for range 2 {
		inactive, err := InspectRetainedBaselinePublications(t.Context(), files, read)
		if err != nil {
			t.Fatal(err)
		}
		expected := []string{filepath.Base(stage) + "/catalog.json", filepath.Base(stage) + "/manifest.json"}
		if !slices.Equal(inactive, expected) {
			t.Fatalf("selection %v, want %v", inactive, expected)
		}
	}
	after, _ := retainedBaselineInputs(t, directory)
	if len(after) != len(files) {
		t.Fatal("changed captured files")
	}
	for name, file := range files {
		if after[name] != file {
			t.Fatal("changed captured bytes")
		}
	}
}

func TestRetainedBaselinePublicationsRefuseUncertainStaging(t *testing.T) {
	for _, mode := range []string{"changed-stage", "missing-journal", "missing-lock", "changed-journal", "unknown-stage-file", "invalid-name", "nil-reader", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "baseline")
			stage, journal, _ := recordedBaselineStage(t, directory)
			files, read := retainedBaselineInputs(t, directory)
			journalName := baselineRecoveryDirectory + "/" + filepath.Base(journal)
			switch mode {
			case "changed-stage":
				name := filepath.Base(stage) + "/catalog.json"
				file := files[name]
				file.Size++
				files[name] = file
			case "missing-journal":
				delete(files, journalName)
			case "missing-lock":
				delete(files, baselineRecoveryDirectory+"/"+baselineRecoveryLock)
			case "changed-journal":
				file := files[journalName]
				file.Size++
				files[journalName] = file
			case "unknown-stage-file":
				files[filepath.Base(stage)+"/operator-note"] = files[filepath.Base(stage)+"/catalog.json"]
			case "invalid-name":
				files["../outside"] = files[journalName]
			case "nil-reader":
				read = nil
			}
			ctx := t.Context()
			if mode == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, err := InspectRetainedBaselinePublications(ctx, files, read); err == nil {
				t.Fatal("accepted uncertain baseline stage")
			}
		})
	}
}

func TestRetainedBaselinePublicationsAcceptHistoricalJournalUpdatesAndCollection(t *testing.T) {
	for _, mode := range []string{"journal-update", "collecting", "promoted"} {
		t.Run(mode, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "baseline")
			stage, journal, record := recordedBaselineStage(t, directory)
			expected := 2
			switch mode {
			case "journal-update":
				raw, err := os.ReadFile(journal)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, baselineRecoveryDirectory, ".record-"+strings.Repeat("B", 26)), raw, 0600); err != nil {
					t.Fatal(err)
				}
				record.Files = nil
			case "collecting":
				record.Phase = baselineJournalCollect
				if err := os.Remove(filepath.Join(stage, baselinePayloadName)); err != nil {
					t.Fatal(err)
				}
				expected = 1
			case "promoted":
				if err := os.Rename(stage, filepath.Join(directory, record.Target)); err != nil {
					t.Fatal(err)
				}
				expected = 0
			}
			raw, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(journal, append(raw, '\n'), 0600); err != nil {
				t.Fatal(err)
			}
			files, read := retainedBaselineInputs(t, directory)
			inactive, err := InspectRetainedBaselinePublications(t.Context(), files, read)
			if err != nil || len(inactive) != expected {
				t.Fatalf("inactive %v; err %v", inactive, err)
			}
		})
	}
}
