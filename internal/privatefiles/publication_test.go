package privatefiles

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestPrivatePublicationReportsDurabilityFailureAfterVisibleBytes(t *testing.T) {
	for _, mode := range []string{"create", "replace", "if-absent", "canceled-after-publication"} {
		t.Run(mode, func(t *testing.T) {
			directory, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "replace" {
				if err := directory.WriteFile("record", []byte("previous"), ".record-"); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cause := fs.ErrInvalid
			if mode == "canceled-after-publication" {
				cause = context.Canceled
			}
			flush := func(root *os.Root) error {
				data, err := ReadFile(root, "record", 32)
				if err != nil || string(data) != "selected" {
					t.Fatalf("synchronization preceded complete bytes: %q, %v", data, err)
				}
				cancel()
				return cause
			}
			err = directory.writeFileContext(ctx, "record", []byte("selected"), ".record-", mode != "if-absent", flush)
			var publication *pkgerrors.PublicationError
			if !errors.As(err, &publication) || publication.Resource != "private file" || publication.ID != "record" || !errors.Is(err, cause) {
				t.Fatalf("missing publication outcome: %v", err)
			}
			data, err := directory.ReadFile("record", 32)
			if err != nil || string(data) != "selected" {
				t.Fatalf("publication failure restored prior bytes: %q, %v", data, err)
			}
			if err := directory.WriteFileContext(t.Context(), "record", []byte("selected"), ".record-"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPrivatePublicationRefusalKeepsPreviousRecord(t *testing.T) {
	directory, err := NewDirectory(filepath.Join(t.TempDir(), "records"))
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.WriteFile("record", []byte("previous"), ".record-"); err != nil {
		t.Fatal(err)
	}
	directory.beforePublish = func(string) error { return fs.ErrPermission }
	err = directory.WriteFileContextWithSync(t.Context(), "record", []byte("selected"), ".record-", func(*os.Root) error {
		t.Fatal("refused publication reached synchronization")
		return nil
	})
	var publication *pkgerrors.PublicationError
	if !errors.Is(err, fs.ErrPermission) || errors.As(err, &publication) {
		t.Fatalf("refused publication returned an accepted outcome: %v", err)
	}
	data, err := directory.ReadFile("record", 32)
	if err != nil || string(data) != "previous" {
		t.Fatalf("refusal changed the record: %q, %v", data, err)
	}
}
