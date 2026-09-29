package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func migrationRestoreReader(records map[string][]byte) RetainedRecordReader {
	return func(ctx context.Context, name string, limit int64) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		body, found := records[name]
		if !found {
			return nil, fmt.Errorf("missing captured record: %w", os.ErrNotExist)
		}
		if int64(len(body)) > limit {
			return nil, invalidMigrationIntent("test_record_size")
		}
		return bytes.Clone(body), nil
	}
}

func TestRetainedMigrationInspectionPreservesCompletedHistory(t *testing.T) {
	request := migrationPublicationFixture(t)
	if _, err := PublishDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	selected := openTestRuntime(t, WithStateDirectory(request.TargetDirectory), WithSchedulerIdentity(request.SourceIdentity), WithDirectoryOwner(request.Owner), WithSource(newStubSource("migration-source")))
	if _, err := selected.CompleteDirectoryMigration(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if err := selected.Close(); err != nil {
		t.Fatal(err)
	}
	records := make(map[string][]byte)
	for _, name := range []string{migrationReceiptName, migrationCompletionName, instanceSeedFileName} {
		body, err := os.ReadFile(filepath.Join(request.TargetDirectory, name))
		if err != nil {
			t.Fatal(err)
		}
		records[name] = body
	}
	// Recovery must not require the former machine's source or target filesystem.
	if err := os.Rename(request.SourceDirectory, request.SourceDirectory+"-offline"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(request.TargetDirectory, request.TargetDirectory+"-offline"); err != nil {
		t.Fatal(err)
	}
	names, err := InspectRetainedMigration(t.Context(), request.TargetDirectory, request.Owner, request.SourceIdentity, migrationRestoreReader(records))
	if err != nil || !reflect.DeepEqual(names, []string{migrationReceiptName, migrationCompletionName}) {
		t.Fatalf("historical selection = %v, %v", names, err)
	}
}

func TestRetainedMigrationInspectionTreatsForeignPathsAsHistory(t *testing.T) {
	for _, paths := range [][2]string{{"/old/runtime", "/new/runtime"}, {`C:\old\runtime`, `D:\new\runtime`}, {`\\server\share\old`, `\\server\share\new`}} {
		_, manifest := migrationJournalFixture(t)
		manifest.SourceDirectory, manifest.TargetDirectory = paths[0], paths[1]
		encoded, err := manifest.encodeFiles()
		if err != nil {
			t.Fatal(err)
		}
		records := map[string][]byte{migrationReceiptName: encoded, migrationCompletionName: bytes.Clone(encoded), instanceSeedFileName: []byte("0123456789abcdef0123456789abcdef")}
		before, err := json.Marshal(records)
		if err != nil {
			t.Fatal(err)
		}
		names, err := InspectRetainedMigration(t.Context(), paths[1], manifest.Owner, manifest.SourceIdentity, migrationRestoreReader(records))
		if err != nil || len(names) != 2 {
			t.Fatalf("foreign history = %v, %v", names, err)
		}
		after, err := json.Marshal(records)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("inspection changed captured records")
		}
	}
}

func TestRetainedMigrationInspectionRefusesIncompleteOrChangedHistory(t *testing.T) {
	for _, mode := range []string{"schema", "operation", "owner", "identity", "directory", "paths", "seed", "missing-seed", "missing-receipt", "missing-completion", "different-completion", "unknown-member", "duplicate-member", "oversize", "canceled", "nil-context", "nil-reader"} {
		t.Run(mode, func(t *testing.T) {
			_, manifest := migrationJournalFixture(t)
			owner, identity, original := manifest.Owner, manifest.SourceIdentity, manifest.TargetDirectory
			switch mode {
			case "schema":
				manifest.SchemaVersion++
			case "operation":
				manifest.OperationID = ""
			case "owner":
				owner.Instance = "another-replica"
			case "identity":
				identity = "another-identity"
			case "directory":
				original = "another-origin"
			case "paths":
				manifest.SourceDirectory = manifest.TargetDirectory
			}
			encoded, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			encoded = append(encoded, '\n')
			switch mode {
			case "unknown-member":
				encoded = bytes.Replace(encoded, []byte("{\n"), []byte("{\n  \"unknown\": true,\n"), 1)
			case "duplicate-member":
				encoded = bytes.Replace(encoded, []byte("{\n"), []byte("{\n  \"schema_version\": 1,\n"), 1)
			case "oversize":
				encoded = []byte(strings.Repeat("x", migrationManifestMaxBytes+1))
			}
			records := map[string][]byte{migrationReceiptName: encoded, migrationCompletionName: bytes.Clone(encoded), instanceSeedFileName: []byte("0123456789abcdef0123456789abcdef")}
			switch mode {
			case "seed":
				records[instanceSeedFileName] = []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
			case "missing-seed":
				delete(records, instanceSeedFileName)
			case "missing-receipt":
				delete(records, migrationReceiptName)
			case "missing-completion":
				delete(records, migrationCompletionName)
			case "different-completion":
				records[migrationCompletionName] = []byte("changed")
			}
			ctx, read := t.Context(), migrationRestoreReader(records)
			if mode == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if mode == "nil-context" {
				ctx = nil
			}
			if mode == "nil-reader" {
				read = nil
			}
			if names, err := InspectRetainedMigration(ctx, original, owner, identity, read); err == nil || len(names) != 0 {
				t.Fatalf("invalid history selected %v, %v", names, err)
			}
		})
	}
}

func TestRetainedMigrationInspectionAcceptsNoMigration(t *testing.T) {
	_, manifest := migrationJournalFixture(t)
	names, err := InspectRetainedMigration(t.Context(), manifest.TargetDirectory, manifest.Owner, "", migrationRestoreReader(nil))
	if err != nil || len(names) != 0 {
		t.Fatalf("ordinary runtime = %v, %v", names, err)
	}
}
