package gitfixture

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFixtureUsesRegularGlobalGitConfig(t *testing.T) {
	New(t, []byte(`{}`))
	path := os.Getenv("GIT_CONFIG_GLOBAL")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Size() != 0 {
		t.Fatalf("global Git config must be an empty regular file: mode=%s size=%d", info.Mode(), info.Size())
	}
}

func TestFixtureFrozenLockfileMatchesWorkspace(t *testing.T) {
	fixture := New(t, []byte(`{}`))
	lockfile := filepath.Join(fixture.root, "bun.lock")
	original, err := os.ReadFile(lockfile)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "bun", "install", "--frozen-lockfile")
	command.Dir = fixture.root
	output, installErr := command.CombinedOutput()
	if installErr != nil {
		diagnostic := exec.CommandContext(ctx, "bun", "install", "--lockfile-only")
		diagnostic.Dir = fixture.root
		diagnosticOutput, diagnosticErr := diagnostic.CombinedOutput()
		generated, readErr := os.ReadFile(lockfile)
		t.Fatalf("frozen install failed: %v\n%s\noriginal lockfile:\n%s\ndiagnostic generation: %v\n%s\ngenerated lockfile (read error: %v):\n%s",
			installErr, output, original, diagnosticErr, diagnosticOutput, readErr, generated)
	}
	after, err := os.ReadFile(lockfile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, after) {
		t.Fatal("frozen install changed the fixture lockfile")
	}
}
