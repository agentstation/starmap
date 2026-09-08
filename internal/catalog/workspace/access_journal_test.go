package workspace

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestJournalRecoveryRefusesUnboundAccessMetadata(t *testing.T) {
	for _, field := range []string{"version", "access_digest"} {
		t.Run(field, func(t *testing.T) {
			f := interruptedReplacement(t, replacementJournalSaved)
			if field == "version" {
				f.record.Version = 1
			} else {
				f.record.Old.Entries[0].AccessSHA256 = ""
			}
			data, err := json.Marshal(f.record)
			if err != nil {
				t.Fatal(err)
			}
			journal := replacementJournalPath(f.path)
			if err := os.WriteFile(journal, data, fileMode); err != nil {
				t.Fatal(err)
			}
			_, err = Repair(t.Context(), f.path, f.catalog, f.identity)
			var invalid *errors.ValidationError
			if !stderrors.As(err, &invalid) {
				t.Fatalf("unbound access metadata: %v", err)
			}
			actual, err := os.ReadFile(journal)
			if err != nil || !bytes.Equal(actual, data) {
				t.Fatal("recovery changed the refused journal", err)
			}
			assertWorkspaceModel(t, f.path, "old", "Old Model")
			assertWorkspaceModel(t, filepath.Join(filepath.Dir(f.path), f.record.Candidate), "new", "New Model")
			if _, err := os.Lstat(filepath.Join(filepath.Dir(f.path), f.record.Backup)); !os.IsNotExist(err) {
				t.Fatal("refused journal moved the workspace", err)
			}
		})
	}
}
