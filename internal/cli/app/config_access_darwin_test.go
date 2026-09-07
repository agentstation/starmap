package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestConfigurationRejectsPublicDarwinACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("catalog_source: embedded\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("/bin/chmod", "+a", "everyone allow read,readattr,readextattr,readsecurity", path).CombinedOutput(); err != nil {
		t.Fatalf("fixture ACL: %v: %s", err, output)
	}
	if _, err := readConfigurationFile(path); err == nil {
		t.Error("configuration accepted a public ACL grant")
	}
	if output, err := exec.Command("/bin/chmod", "-N", path).CombinedOutput(); err != nil {
		t.Fatalf("operator ACL correction: %v: %s", err, output)
	}
	if _, err := readConfigurationFile(path); err != nil {
		t.Fatalf("private configuration after explicit ACL correction: %v", err)
	}
}
