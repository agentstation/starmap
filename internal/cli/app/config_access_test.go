package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
)

func TestConfigurationReadByteLimitAndMissingFile(t *testing.T) {
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	file, err := privatefiles.CreateFile(root, "config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	contents := bytes.Repeat([]byte("#"), int(maxConfigurationFileBytes))
	if _, err := file.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "config.yaml")
	got, err := readConfigurationFile(path)
	if err != nil || !bytes.Equal(got, contents) {
		t.Fatalf("exact byte limit: %v", err)
	}
	appendFile, err := root.OpenFile("config.yaml", os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appendFile.Write([]byte("#")); err != nil {
		t.Fatal(err)
	}
	if err := appendFile.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfigurationFile(path); err == nil {
		t.Fatal("configuration accepted a file over its byte limit")
	}
	if _, err := readConfigurationFile(filepath.Join(directory, "absent.yaml")); !os.IsNotExist(err) {
		t.Fatalf("missing file classification: %v", err)
	}
	if _, err := readConfigurationFile(directory); err == nil {
		t.Fatal("configuration accepted a directory")
	}
}
