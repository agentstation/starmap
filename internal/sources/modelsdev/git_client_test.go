package modelsdev

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestGitClientPinsCommitAndFrozenLockfile(t *testing.T) {
	remote := t.TempDir()
	runGitTestCommand(t, remote, "init")
	runGitTestCommand(t, remote, "config", "user.email", "starmap@example.test")
	runGitTestCommand(t, remote, "config", "user.name", "Starmap Test")
	lockV1 := []byte("lockfileVersion = 1\n")
	if err := os.WriteFile(filepath.Join(remote, lockfileName), lockV1, constants.FilePermissions); err != nil {
		t.Fatalf("write lock v1: %v", err)
	}
	webDir := filepath.Join(remote, "packages", "web")
	if err := os.MkdirAll(webDir, constants.DirPermissions); err != nil {
		t.Fatalf("create web dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "package.json"), []byte("{}\n"), constants.FilePermissions); err != nil {
		t.Fatalf("write package: %v", err)
	}
	runGitTestCommand(t, remote, "add", lockfileName, "packages/web/package.json")
	runGitTestCommand(t, remote, "commit", "-m", "first")
	commitV1 := strings.TrimSpace(runGitTestCommand(t, remote, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(remote, lockfileName), []byte("lockfileVersion = 2\n"), constants.FilePermissions); err != nil {
		t.Fatalf("write lock v2: %v", err)
	}
	runGitTestCommand(t, remote, "add", lockfileName)
	runGitTestCommand(t, remote, "commit", "-m", "second")

	fakeBin := t.TempDir()
	invocations := filepath.Join(t.TempDir(), "bun-invocations")
	bun := filepath.Join(fakeBin, "bun")
	bunScript := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$BUN_INVOCATIONS\"\n" +
		"if [ \"$1\" = run ]; then mkdir -p dist; printf '%s' \"$MODELS_DEV_API_FIXTURE\" > dist/_api.json; fi\n"
	if err := os.WriteFile(bun, []byte(bunScript), constants.ExecutablePermissions); err != nil {
		t.Fatalf("write fake bun: %v", err)
	}
	t.Setenv("BUN_INVOCATIONS", invocations)
	t.Setenv("MODELS_DEV_API_FIXTURE", mockAPIJSON())
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	client := &GitClient{RepoPath: filepath.Join(t.TempDir(), "checkout"), RepoURL: remote, Commit: commitV1}
	inputs, err := client.PrepareRepository(context.Background())
	if err != nil {
		t.Fatalf("PrepareRepository: %v", err)
	}
	digest := sha256.Sum256(lockV1)
	wantChecksum := "sha256:" + hex.EncodeToString(digest[:])
	if inputs.Commit != commitV1 || inputs.LockfilePath != lockfileName || inputs.LockfileChecksum != wantChecksum {
		t.Fatalf("inputs = %#v, want commit %q lock %q checksum %q", inputs, commitV1, lockfileName, wantChecksum)
	}
	if head := strings.TrimSpace(runGitTestCommand(t, client.RepoPath, "rev-parse", "HEAD")); head != commitV1 {
		t.Fatalf("checkout HEAD = %q, want pinned %q", head, commitV1)
	}
	invocationData, err := os.ReadFile(invocations)
	if err != nil {
		t.Fatalf("read bun invocations: %v", err)
	}
	if strings.TrimSpace(string(invocationData)) != "install --frozen-lockfile" {
		t.Fatalf("bun invocation = %q", invocationData)
	}

	revision := revisionForGitInputs(inputs)
	if revision.Kind != sources.RevisionKindGitCommit || revision.Value != commitV1 ||
		revision.InputName != lockfileName || revision.InputChecksum != wantChecksum {
		t.Fatalf("revision = %#v", revision)
	}
	if err := client.BuildAPI(context.Background()); err != nil {
		t.Fatalf("BuildAPI: %v", err)
	}
	api, err := ParseAPI(client.GetAPIPath())
	if err != nil {
		t.Fatalf("ParseAPI: %v", err)
	}
	if _, found := (*api)["openai"]; !found {
		t.Fatalf("built API = %#v", api)
	}
	invocationData, err = os.ReadFile(invocations)
	if err != nil {
		t.Fatalf("read build invocations: %v", err)
	}
	if !strings.Contains(string(invocationData), "run script/build.ts") {
		t.Fatalf("build invocation missing from %q", invocationData)
	}
}

func TestGitSemanticPromotionRejectsSyntacticallyValidIncompleteBuild(t *testing.T) {
	api, err := parseAPIData([]byte(mockAPIJSON()))
	if err != nil {
		t.Fatalf("parseAPIData: %v", err)
	}
	if err := validateAPIPromotion(api, nil); err == nil {
		t.Fatal("semantic promotion accepted a syntactically valid one-model Git build")
	}
}

func TestGitClientRejectsFloatingRevisionBeforeExternalWork(t *testing.T) {
	client := &GitClient{RepoPath: filepath.Join(t.TempDir(), "checkout"), RepoURL: "https://example.invalid/repository.git"}
	if _, err := client.PrepareRepository(context.Background()); err == nil {
		t.Fatal("PrepareRepository accepted an empty floating revision")
	}
	if _, err := os.Stat(client.RepoPath); !os.IsNotExist(err) {
		t.Fatalf("unpinned verification performed external work: %v", err)
	}
}

func runGitTestCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
