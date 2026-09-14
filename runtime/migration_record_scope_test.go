package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeMigrationPreservesUnrelatedRecordDirectory(t *testing.T) {
	for _, parent := range []string{"operator-data", "catalog-runtime/operator-data", "github-catalog-source/operator-data"} {
		t.Run(parent, func(t *testing.T) {
			request := directoryMigrationRequestFixture(t)
			relative := filepath.Join(filepath.FromSlash(parent), ".record-publications", "notes.jsonl")
			path := filepath.Join(request.SourceDirectory, relative)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("operator-owned data"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := PrepareDirectoryMigration(t.Context(), request); err != nil {
				t.Fatalf("migration adopted an unrelated receipt directory: %v", err)
			}
			stage, err := StageDirectoryMigration(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range []string{path, filepath.Join(stage.StageDirectory, relative)} {
				data, err := os.ReadFile(file)
				if err != nil || string(data) != "operator-owned data" {
					t.Fatalf("migration did not preserve operator data: %q %v", data, err)
				}
			}
		})
	}
}
