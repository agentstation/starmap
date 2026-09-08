//go:build darwin || linux || windows

package filepublish

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func publicationRoot(t *testing.T) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func stageDirectory(t *testing.T, root *os.Root, name string) {
	t.Helper()
	if err := root.Mkdir(name, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile(filepath.Join(name, "payload"), []byte(name), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDirectoryNoReplacePreservesExistingDestination(t *testing.T) {
	for _, kind := range []string{"empty", "populated", "file"} {
		t.Run(kind, func(t *testing.T) {
			root := publicationRoot(t)
			stageDirectory(t, root, "staged")
			switch kind {
			case "empty":
				if err := root.Mkdir("target", 0o700); err != nil {
					t.Fatal(err)
				}
			case "populated":
				stageDirectory(t, root, "target")
			case "file":
				if err := root.WriteFile("target", []byte("preserve"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := root.Lstat("target")
			if err != nil {
				t.Fatal(err)
			}
			if err := DirectoryNoReplace(root, "staged", "target"); !os.IsExist(err) {
				t.Fatalf("collision error = %v; want an existing destination", err)
			}
			after, err := root.Lstat("target")
			if err != nil || !os.SameFile(before, after) {
				t.Fatal("destination identity changed", err)
			}
			payload, err := root.ReadFile("staged/payload")
			if err != nil || string(payload) != "staged" {
				t.Fatal("staging contents changed", err)
			}
			if kind != "empty" {
				name, expected := "target/payload", "target"
				if kind == "file" {
					name, expected = "target", "preserve"
				}
				payload, err := root.ReadFile(name)
				if err != nil || string(payload) != expected {
					t.Fatal("destination contents changed", err)
				}
			}
		})
	}
}

func TestDirectoryNoReplaceUsesOpenRootAfterRename(t *testing.T) {
	container := t.TempDir()
	original, moved := filepath.Join(container, "original"), filepath.Join(container, "moved")
	if err := os.Mkdir(original, 0o700); err != nil {
		t.Fatal(err)
	}
	anchor, err := os.OpenRoot(container)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = anchor.Close() }()
	root, err := anchor.OpenRoot("original")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	stageDirectory(t, root, "staged-星")
	if err := os.Rename(original, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(original, 0o700); err != nil {
		t.Fatal(err)
	}
	decoy, err := os.OpenRoot(original)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = decoy.Close() }()
	stageDirectory(t, decoy, "staged-星")
	if err := DirectoryNoReplace(root, "staged-星", "published-🚀"); err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(filepath.Join(moved, "published-🚀", "payload"))
	if err != nil || string(payload) != "staged-星" {
		t.Fatal("publication did not use the open root", err)
	}
	if _, err := decoy.Lstat("published-🚀"); !os.IsNotExist(err) {
		t.Fatal("publication changed the replacement root", err)
	}
	if _, err := root.Lstat("staged-星"); !os.IsNotExist(err) {
		t.Fatal("publication retained its source name", err)
	}
}

func TestDirectoryNoReplaceConcurrentPublishers(t *testing.T) {
	root := publicationRoot(t)
	const writers = 16
	for i := range writers {
		stageDirectory(t, root, fmt.Sprint(i))
	}
	errors := make([]error, writers)
	start := make(chan struct{})
	var group sync.WaitGroup
	for i := range writers {
		group.Go(func() { <-start; errors[i] = DirectoryNoReplace(root, fmt.Sprint(i), "target") })
	}
	close(start)
	group.Wait()
	winner := -1
	for i, err := range errors {
		if err == nil {
			if winner >= 0 {
				t.Fatal("more than one publisher succeeded")
			}
			winner = i
		} else if !os.IsExist(err) {
			t.Fatalf("publisher %d: %v", i, err)
		}
	}
	if winner < 0 {
		t.Fatal("no publisher succeeded")
	}
	payload, err := root.ReadFile("target/payload")
	if err != nil || string(payload) != fmt.Sprint(winner) {
		t.Fatal("published payload differs from the winner", err)
	}
	for i := range writers {
		if i == winner {
			continue
		}
		payload, err := root.ReadFile(filepath.Join(fmt.Sprint(i), "payload"))
		if err != nil || string(payload) != fmt.Sprint(i) {
			t.Fatal("failed publisher lost its stage", err)
		}
	}
}

func TestDirectoryNoReplaceRejectsInvalidInputs(t *testing.T) {
	root := publicationRoot(t)
	stageDirectory(t, root, "staged")
	for _, name := range []string{"", ".", "..", "../outside", "/absolute", "nested/name", "nested\\name", "stream:name", "trailing.", "trailing ", "nul\x00name"} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			if err := DirectoryNoReplace(root, "staged", name); err == nil {
				t.Fatal("invalid target accepted")
			}
			if err := DirectoryNoReplace(root, name, "target"); err == nil {
				t.Fatal("invalid source accepted")
			}
		})
	}
	if err := DirectoryNoReplace(nil, "staged", "target"); err == nil {
		t.Fatal("nil root accepted")
	}
	if err := DirectoryNoReplace(root, "staged", "staged"); err == nil {
		t.Fatal("identical names accepted")
	}
	if err := root.WriteFile("file", []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := DirectoryNoReplace(root, "file", "target"); err == nil {
		t.Fatal("file source accepted")
	}
	if _, err := root.Lstat("target"); !os.IsNotExist(err) {
		t.Fatal("invalid publication created a target", err)
	}
}

func TestDirectoryNoReplacePreservesSymlinks(t *testing.T) {
	for _, role := range []string{"source", "target"} {
		t.Run(role, func(t *testing.T) {
			root := publicationRoot(t)
			stageDirectory(t, root, "outside")
			if err := root.Symlink("outside", role); err != nil {
				t.Fatal(err)
			}
			if role == "target" {
				stageDirectory(t, root, "source")
			}
			if err := DirectoryNoReplace(root, "source", "target"); err == nil {
				t.Fatal("publication accepted a symlink")
			}
			link, err := root.Readlink(role)
			if err != nil || link != "outside" {
				t.Fatal("publication changed the link", err)
			}
			payload, err := root.ReadFile("outside/payload")
			if err != nil || string(payload) != "outside" {
				t.Fatal("publication changed the link destination", err)
			}
		})
	}
}
