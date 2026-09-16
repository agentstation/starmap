package productfiles_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/productfiles"
)

func TestPrivateRecordLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private")
	if _, err := productfiles.ExistingDirectory(path); !os.IsNotExist(err) {
		t.Fatalf("missing directory: %v", err)
	}
	directory, err := productfiles.NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	first, second := []byte("first"), []byte("second")
	if err := directory.CompareAndPublish(t.Context(), "config.env", nil, first); err != nil {
		t.Fatal(err)
	}
	if err := directory.CompareAndPublish(t.Context(), "config.env", nil, second); err == nil {
		t.Fatal("replaced an existing record")
	}
	if _, err := directory.ReadFile("config.env", 4); err == nil {
		t.Fatal("read exceeded its bound")
	}
	if err := directory.CompareAndPublish(t.Context(), "config.env", first, second); err != nil {
		t.Fatal(err)
	}
	if err := directory.RecoverPublications(t.Context()); err != nil {
		t.Fatal(err)
	}
	if removed, err := directory.CompareAndRemove(t.Context(), "config.env", first); err == nil || removed {
		t.Fatalf("removed changed bytes: %t, %v", removed, err)
	}
	got, err := directory.ReadFile("config.env", 16)
	if err != nil || string(got) != string(second) {
		t.Fatalf("record = %q, %v", got, err)
	}
	if removed, err := directory.CompareAndRemove(t.Context(), "config.env", second); err != nil || !removed {
		t.Fatalf("remove = %t, %v", removed, err)
	}
	if removed, err := directory.CompareAndRemove(t.Context(), "config.env", second); err != nil || removed {
		t.Fatalf("repeated remove = %t, %v", removed, err)
	}
}

func TestDirectoryPublicationDoesNotReplace(t *testing.T) {
	source := newDirectory(t, filepath.Join(t.TempDir(), "source"))
	target := newDirectory(t, filepath.Join(t.TempDir(), "target"))
	stage, err := source.Child("stage")
	if err != nil {
		t.Fatal(err)
	}
	if err := stage.CompareAndPublish(t.Context(), "record", nil, []byte("owned")); err != nil {
		t.Fatal(err)
	}
	from, err := source.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer from.Close()
	to, err := target.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer to.Close()
	if err := productfiles.PublishDirectory(from, "stage", to, "installed"); err != nil {
		t.Fatal(err)
	}
	for _, root := range []*os.Root{from, to} {
		if err := productfiles.SyncDirectory(root); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stage.Open(); err == nil {
		t.Fatal("old directory binding survived relocation")
	}
	installed, err := target.ExistingChild("installed")
	if err != nil {
		t.Fatal(err)
	}
	got, err := installed.ReadFile("record", 32)
	if err != nil || string(got) != "owned" {
		t.Fatalf("installed bytes = %q, %v", got, err)
	}
	if _, err := source.Child("second"); err != nil {
		t.Fatal(err)
	}
	if err := productfiles.PublishDirectory(from, "second", to, "installed"); !os.IsExist(err) {
		t.Fatalf("replacement error = %v", err)
	}
	if err := productfiles.PublishDirectory(from, "../second", to, "escape"); err == nil {
		t.Fatal("accepted a parent traversal")
	}
}

func TestDirectoryBindingRejectsReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private")
	directory := newDirectory(t, path)
	if err := os.Rename(path, path+"-old"); err != nil {
		t.Fatal(err)
	}
	newDirectory(t, path)
	if _, err := directory.Open(); err == nil {
		t.Fatal("accepted a replaced directory")
	}
}

func TestPrivateOperationsRejectCancellation(t *testing.T) {
	directory := newDirectory(t, filepath.Join(t.TempDir(), "private"))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := directory.CompareAndPublish(ctx, "record", nil, []byte("secret")); !errors.Is(err, context.Canceled) {
		t.Fatalf("publication = %v", err)
	}
	if _, err := directory.ReadFile("record", 32); !os.IsNotExist(err) {
		t.Fatalf("canceled publication created a record: %v", err)
	}
	if err := directory.CompareAndPublish(nil, "record", nil, nil); err == nil {
		t.Fatal("accepted a nil context")
	}
	if _, err := directory.CompareAndRemove(nil, "record", []byte{}); err == nil {
		t.Fatal("accepted a nil removal context")
	}
	if err := directory.RecoverPublications(nil); err == nil {
		t.Fatal("accepted a nil recovery context")
	}
}

func TestZeroDirectoryRefusesAccess(t *testing.T) {
	for _, directory := range []*productfiles.Directory{nil, {}} {
		if _, err := directory.Open(); err == nil {
			t.Fatal("opened an unbound directory")
		}
		if _, err := directory.Child("child"); err == nil {
			t.Fatal("created an unbound child")
		}
		if _, err := directory.ExistingChild("child"); err == nil {
			t.Fatal("bound a child without a parent")
		}
		if _, err := directory.ReadFile("record", 32); err == nil {
			t.Fatal("read an unbound directory")
		}
		if err := directory.CompareAndPublish(t.Context(), "record", nil, nil); err == nil {
			t.Fatal("published to an unbound directory")
		}
		if _, err := directory.CompareAndRemove(t.Context(), "record", []byte{}); err == nil {
			t.Fatal("removed from an unbound directory")
		}
		if err := directory.RecoverPublications(t.Context()); err == nil {
			t.Fatal("recovered an unbound directory")
		}
	}
}

func newDirectory(t *testing.T, path string) *productfiles.Directory {
	t.Helper()
	directory, err := productfiles.NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	return directory
}
