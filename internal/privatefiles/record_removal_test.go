package privatefiles

import (
	"context"
	stderrors "errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

type conditionalRecordRemover interface {
	CompareAndRemoveFileContext(context.Context, string, []byte) (bool, error)
}

func TestPrivateRecordRemovalRefusesActivePublisher(t *testing.T) {
	directory, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("retained")
	if err := directory.PublishFileContext(t.Context(), "record", data, ".record-"); err != nil {
		t.Fatal(err)
	}
	writer, err := directory.acquirePublicationWriter(t.Context(), true)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.close()
	if removed, err := directory.CompareAndRemoveFileContext(t.Context(), "record", data); removed || !pkgerrors.IsConflict(err) {
		t.Fatalf("active publisher did not exclude removal: %v, %v", removed, err)
	}
	got, err := directory.ReadFile("record", 32)
	if err != nil || string(got) != string(data) {
		t.Fatalf("active publication lost content: %q, %v", got, err)
	}
}

func TestPrivateRecordRemovalReportsVisibleOutcome(t *testing.T) {
	directory, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("retired")
	if err := directory.PublishFileContext(t.Context(), "record", data, ".record-"); err != nil {
		t.Fatal(err)
	}
	removed, err := directory.compareAndRemoveFileContext(t.Context(), "record", data, func(root *os.Root) error {
		if _, err := root.Lstat("record"); !os.IsNotExist(err) {
			t.Fatalf("flush preceded removal: %v", err)
		}
		return fs.ErrInvalid
	})
	var publication *pkgerrors.PublicationError
	if !removed || !stderrors.As(err, &publication) || !stderrors.Is(err, fs.ErrInvalid) {
		t.Fatalf("ambiguous removal outcome: %v, %v", removed, err)
	}
	reopened, err := ExistingDirectory(directory.path)
	if err != nil {
		t.Fatal(err)
	}
	if removed, err := reopened.CompareAndRemoveFileContext(t.Context(), "record", data); removed || err != nil {
		t.Fatalf("retry after reopen: %v, %v", removed, err)
	}
}

func TestPrivateRecordRemovalValidatesSelection(t *testing.T) {
	for _, name := range []string{"empty", "nil expectation", "canceled", "parent traversal", "metadata", "directory", "symlink", "replaced parent"} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			directory, err := NewDirectory(filepath.Join(parent, "records"))
			if err != nil {
				t.Fatal(err)
			}
			data := []byte{}
			if err := directory.PublishFileContext(t.Context(), "record", data, ".record-"); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			selected := "record"
			switch name {
			case "nil expectation":
				data = nil
			case "canceled":
				cancel()
			case "parent traversal":
				selected = "../records/record"
			case "metadata":
				selected = publicationDirectory
			case "directory":
				selected = "child"
				if _, err := directory.Child(selected); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				selected = "link"
				if err := os.Symlink("record", filepath.Join(directory.path, selected)); err != nil {
					t.Skipf("symlink creation unavailable: %v", err)
				}
			case "replaced parent":
				if err := os.Rename(directory.path, filepath.Join(parent, "original")); err != nil {
					t.Fatal(err)
				}
				replacement, err := NewDirectory(directory.path)
				if err != nil {
					t.Fatal(err)
				}
				if err := replacement.PublishFileContext(t.Context(), "record", data, ".record-"); err != nil {
					t.Fatal(err)
				}
			}
			removed, err := directory.CompareAndRemoveFileContext(ctx, selected, data)
			if name == "empty" {
				if err != nil || !removed {
					t.Fatalf("empty record removal: %v, %v", removed, err)
				}
				return
			}
			if err == nil || removed {
				t.Fatalf("invalid selection removed a record: %v, %v", removed, err)
			}
			if name == "canceled" && !stderrors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
			if _, err := os.Stat(filepath.Join(directory.path, "record")); err != nil {
				t.Fatalf("refusal lost the retained record: %v", err)
			}
		})
	}
}

func TestPrivateRecordRemovalPreservesChangedContent(t *testing.T) {
	directory, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
	if err != nil {
		t.Fatal(err)
	}
	remover, ok := any(directory).(conditionalRecordRemover)
	if !ok {
		t.Fatal("private records lack checked removal under publication ownership")
	}
	if err := directory.PublishFileContext(t.Context(), "record.json", []byte("newer"), ".record-"); err != nil {
		t.Fatal(err)
	}
	removed, err := remover.CompareAndRemoveFileContext(t.Context(), "record.json", []byte("older"))
	if err == nil || removed {
		t.Fatalf("changed record removed: %v, %v", removed, err)
	}
	data, err := directory.ReadFile("record.json", 32)
	if err != nil || string(data) != "newer" {
		t.Fatalf("changed record lost: %q, %v", data, err)
	}
	removed, err = remover.CompareAndRemoveFileContext(t.Context(), "record.json", []byte("newer"))
	if err != nil || !removed {
		t.Fatalf("matching removal: %v, %v", removed, err)
	}
	if _, err := directory.ReadFile("record.json", 32); !os.IsNotExist(err) {
		t.Fatalf("removed record remains: %v", err)
	}
	removed, err = remover.CompareAndRemoveFileContext(t.Context(), "record.json", []byte("newer"))
	if err != nil || removed {
		t.Fatalf("idempotent retry: %v, %v", removed, err)
	}
}
