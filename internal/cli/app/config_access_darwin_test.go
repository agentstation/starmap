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

func TestServiceConfigurationDarwinGrants(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("catalog_source: embedded\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		grant   string
		allowed bool
	}{
		{"everyone allow read,readattr,readextattr,readsecurity", true},
		{"everyone allow write", false},
		{"everyone allow append", false},
		{"everyone allow writesecurity", false},
		{"everyone allow chown", false},
	} {
		t.Run(test.grant, func(t *testing.T) {
			if output, err := exec.Command("/bin/chmod", "-N", path).CombinedOutput(); err != nil {
				t.Fatalf("reset ACL: %v: %s", err, output)
			}
			if output, err := exec.Command("/bin/chmod", "+a", test.grant, path).CombinedOutput(); err != nil {
				t.Fatalf("set ACL: %v: %s", err, output)
			}
			_, err := readSelectedConfiguration(path, "service-managed")
			if (err == nil) != test.allowed {
				t.Fatalf("allowed=%v, error=%v", test.allowed, err)
			}
			if _, err := readConfigurationFile(path); err == nil {
				t.Fatal("private reader accepted public grant")
			}
		})
	}
}
