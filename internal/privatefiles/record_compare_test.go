package privatefiles

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestPrivateRecordCompareAndPublish(t *testing.T) {
	for _, test := range []struct {
		name               string
		existing, previous []byte
		conflict           bool
	}{
		{name: "absent"},
		{name: "empty", existing: []byte{}, previous: []byte{}},
		{name: "same", existing: []byte("accepted"), previous: []byte("accepted")},
		{name: "changed", existing: []byte("newer"), previous: []byte("older"), conflict: true},
		{name: "created", existing: []byte("newer"), conflict: true},
		{name: "removed", previous: []byte("older"), conflict: true},
		{name: "empty requires presence", previous: []byte{}, conflict: true},
		{name: "nil requires absence", existing: []byte{}, conflict: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
			if err != nil {
				t.Fatal(err)
			}
			if test.existing != nil {
				if err := directory.PublishFileContext(t.Context(), "state.json", test.existing, ".state-"); err != nil {
					t.Fatal(err)
				}
			}
			err = directory.CompareAndPublishFileContext(t.Context(), "state.json", test.previous, []byte("candidate"), ".state-")
			var conflict *errors.ConflictError
			if test.conflict {
				if !stderrors.As(err, &conflict) {
					t.Fatalf("comparison result: %v", err)
				}
				raw, readErr := directory.ReadFile("state.json", 128)
				if test.existing == nil {
					if !os.IsNotExist(readErr) {
						t.Fatalf("absent record changed: %q %v", raw, readErr)
					}
				} else if readErr != nil || string(raw) != string(test.existing) {
					t.Fatalf("accepted record changed: %q %v", raw, readErr)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				raw, err := directory.ReadFile("state.json", 128)
				if err != nil || string(raw) != "candidate" {
					t.Fatalf("publication: %q %v", raw, err)
				}
			}
		})
	}
}
