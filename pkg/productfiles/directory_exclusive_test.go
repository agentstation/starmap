package productfiles_test

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/productfiles"
)

func TestPrivateDirectoryExclusiveCreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scratch")
	directory, err := productfiles.CreateDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.CompareAndPublish(t.Context(), "owned", nil, []byte("preserved")); err != nil {
		t.Fatal(err)
	}
	before, err := directory.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := productfiles.CreateDirectory(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing target = %v, want existence error", err)
	}
	after, err := directory.Identity()
	if err != nil || before != after {
		t.Fatalf("existing identity changed: %v", err)
	}
	data, err := directory.ReadFile("owned", 64)
	if err != nil || string(data) != "preserved" {
		t.Fatalf("existing content = %q, %v", data, err)
	}
	if _, err := productfiles.CreateDirectory(filepath.Join(path, "owned")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing file = %v, want existence error", err)
	}
	if _, err := productfiles.CreateDirectory(filepath.Join(path, "missing", "child")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing parent = %v, want missing error", err)
	}
	if _, err := productfiles.CreateDirectory("relative"); err == nil {
		t.Fatal("accepted a relative path")
	}
}

func TestPrivateDirectoryExclusiveConcurrentCreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scratch")
	const creators = 8
	results := make(chan error, creators)
	var group sync.WaitGroup
	for range creators {
		group.Go(func() {
			_, err := productfiles.CreateDirectory(path)
			results <- err
		})
	}
	group.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, os.ErrExist) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("successful creators = %d, want 1", winners)
	}
}
