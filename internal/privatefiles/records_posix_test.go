//go:build darwin || linux

package privatefiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateRecordsRefuseExposedFilesWithoutRepair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records")
	directory, err := NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.WriteFile("source.json", []byte("retained"), ".layer-"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(path, "source.json")
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := directory.ReadFile("source.json", 100); err == nil {
		t.Fatal("exposed record accepted for reading")
	}
	if err := directory.WriteFile("source.json", []byte("new"), ".layer-"); err == nil {
		t.Fatal("exposed record accepted for writing")
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatal("refusal changed the file mode", err)
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "retained" {
		t.Fatal("refusal changed retained bytes", err)
	}
	if err := os.Chmod(file, FileMode); err != nil {
		t.Fatal(err)
	}
	if err := directory.WriteFile("source.json", []byte("recovered"), ".layer-"); err != nil {
		t.Fatal("explicit mode correction did not restore writes", err)
	}
}

func TestPrivateRecordsRefuseLinksAndSpecialFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records")
	directory, err := NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "operator.json")
	if err := os.WriteFile(outside, []byte("preserve"), FileMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(path, "source.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := directory.ReadFile("source.json", 100); err == nil {
		t.Fatal("linked record accepted")
	}
	if err := directory.WriteFile("source.json", []byte("new"), ".layer-"); err == nil {
		t.Fatal("linked destination accepted")
	}
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != "preserve" {
		t.Fatal("linked target changed", err)
	}
	if err := os.Mkdir(filepath.Join(path, "directory.json"), DirectoryMode); err != nil {
		t.Fatal(err)
	}
	if _, err := directory.ReadFile("directory.json", 100); err == nil {
		t.Fatal("directory record accepted")
	}
}
