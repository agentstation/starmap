package privatefiles

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// This test needs a root container or native privileged test job to change fixture ownership.
func TestServiceConfigurationAdministratorOwnedRead(t *testing.T) {
	const marker = "CSP_SERVICE_CONFIGURATION_CHILD"
	if directory := os.Getenv(marker); directory != "" {
		if os.Geteuid() == 0 {
			t.Fatal("child did not drop privileges")
		}
		root, err := os.OpenRoot(directory)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = root.Close() }()
		for _, test := range []struct {
			name    string
			allowed bool
		}{
			{"readable.yaml", true}, {"denied.yaml", false}, {"writable.yaml", false}, {"foreign.yaml", false},
		} {
			path := filepath.Join(directory, test.name)
			if err := ValidateAncestors(path); err != nil {
				t.Fatal(err)
			}
			data, err := ReadServiceConfiguration(root, test.name, 100)
			if (err == nil) != test.allowed {
				t.Fatalf("%s: data=%q error=%v", test.name, data, err)
			}
			if test.allowed && string(data) != "catalog_source: embedded\n" {
				t.Fatal("wrong bytes")
			}
			if _, err := ReadFile(root, test.name, 100); err == nil {
				t.Fatalf("private reader accepted %s", test.name)
			}
		}
		return
	}
	if os.Geteuid() != 0 {
		t.Skip("requires an isolated root job for native owner fixtures")
	}
	directory, err := os.MkdirTemp("/tmp", "starmap-service-config-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(directory) }()
	if err := os.Chmod(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		mode     os.FileMode
		uid, gid int
	}{
		{"readable.yaml", 0o640, 0, 65534}, {"denied.yaml", 0o600, 0, 0},
		{"writable.yaml", 0o660, 0, 65534}, {"foreign.yaml", 0o644, 65533, 65534},
	} {
		path := filepath.Join(directory, test.name)
		if err := os.WriteFile(path, []byte("catalog_source: embedded\n"), test.mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(path, test.uid, test.gid); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, test.mode); err != nil {
			t.Fatal(err)
		}
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Go's build directory can be private. Copy only this test executable into the fixture.
	bytes, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(directory, "check")
	if err := os.WriteFile(child, bytes, 0o755); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(child, "-test.run=^TestServiceConfigurationAdministratorOwnedRead$", "-test.v")
	command.Env = append(os.Environ(), marker+"="+directory)
	command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("native service read: %v: %s", err, output)
	} else {
		t.Log(string(output))
	}
}
