//go:build darwin || linux

package privatefiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateCreationPreservesTrustedAncestorLayouts(t *testing.T) {
	for _, mode := range []os.FileMode{0o700, 0o755, os.ModeSticky | 0o777} {
		t.Run(mode.String(), func(t *testing.T) {
			parent := t.TempDir()
			if err := os.Chmod(parent, mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(parent, DirectoryMode) })
			path := filepath.Join(parent, "first", "records")
			if _, err := NewDirectory(path); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(parent)
			if err != nil || info.Mode().Perm() != mode.Perm() || info.Mode()&os.ModeSticky != mode&os.ModeSticky {
				t.Fatal("creation changed ancestor permissions", err)
			}
		})
	}
}

func TestPrivateCreationChecksSymlinkTargetAncestors(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, DirectoryMode); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDirectory(filepath.Join(alias, "records")); err != nil {
		t.Fatal("trusted alias refused", err)
	}
	redirect := filepath.Join(base, "redirect")
	if err := os.Mkdir(redirect, DirectoryMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(redirect, "hop")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(redirect, 0o777); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(redirect, DirectoryMode) })
	unsafe := filepath.Join(base, "unsafe")
	if err := os.Symlink(filepath.Join(redirect, "hop"), unsafe); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDirectory(filepath.Join(unsafe, "new-records")); err == nil {
		t.Fatal("creation accepted an unsafe intermediate symlink directory")
	}
	if _, err := os.Stat(filepath.Join(target, "new-records")); !os.IsNotExist(err) {
		t.Fatal("unsafe alias created target files")
	}
}

func TestPrivateCreationRefusesReplacedChildBeforeDescent(t *testing.T) {
	base, outside := t.TempDir(), t.TempDir()
	err := createDirectory(filepath.Join(base, "first", "records"), func(root *os.Root, name string) error {
		if name != "first" {
			return nil
		}
		if err := root.Rename(name, "preserved"); err != nil {
			return err
		}
		return os.Symlink(outside, filepath.Join(base, name))
	})
	if err == nil {
		t.Fatal("creation followed a replacement symlink")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatal("creation wrote beyond its directory binding", err)
	}
	if _, err := os.Lstat(filepath.Join(base, "preserved")); err != nil {
		t.Fatal("refusal removed the preserved child", err)
	}
}

func TestPrivatePublicationRechecksAncestorAccess(t *testing.T) {
	ancestor := t.TempDir()
	directory, err := NewDirectory(filepath.Join(ancestor, "records"))
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.WriteFile("record.json", []byte("original"), ".stage-"); err != nil {
		t.Fatal(err)
	}
	directory.beforePublish = func(string) error { return os.Chmod(ancestor, 0o777) }
	t.Cleanup(func() { _ = os.Chmod(ancestor, DirectoryMode) })
	if err := directory.WriteFile("record.json", []byte("replacement"), ".stage-"); err == nil {
		t.Fatal("publication ignored an ancestor access change")
	}
	data, err := os.ReadFile(filepath.Join(ancestor, "records", "record.json"))
	if err != nil || string(data) != "original" {
		t.Fatal("refused publication changed the record", err)
	}
}

func TestPrivateRecordsRefuseWritableAncestorsBeforeAccess(t *testing.T) {
	for _, operation := range []string{"create", "bind", "read", "write"} {
		t.Run(operation, func(t *testing.T) {
			ancestor := t.TempDir()
			path := filepath.Join(ancestor, "parent", "records")
			directory, err := NewDirectory(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := directory.WriteFile("record.json", []byte("original"), ".stage-"); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(ancestor, 0o777); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(ancestor, DirectoryMode) })
			switch operation {
			case "create":
				_, err = NewDirectory(filepath.Join(ancestor, "missing", "records"))
			case "bind":
				_, err = ExistingDirectory(path)
			case "read":
				_, err = directory.ReadFile("record.json", 100)
			case "write":
				err = directory.WriteFile("record.json", []byte("replacement"), ".stage-")
			}
			if err == nil {
				t.Error("private operation accepted an exposed ancestor")
			}
			if _, err := os.Stat(filepath.Join(ancestor, "missing")); !os.IsNotExist(err) {
				t.Error("refused creation wrote under the exposed ancestor")
			}
			data, err := os.ReadFile(filepath.Join(path, "record.json"))
			if err != nil || string(data) != "original" {
				t.Error("refused operation changed retained bytes", err)
			}
		})
	}
}
