// Package gitfixture builds local models.dev repositories with real Git and Bun.
package gitfixture

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
)

//go:embed testdata/*
var inputs embed.FS

// Fixture owns pinned metadata revisions and their immutable build input.
type Fixture struct {
	Commits          []string
	LockfileChecksum string
	buildLog         string
}

// New creates a local repository and redirects the production Git URL to it.
// Each payload creates one commit. Tests that use New must run serially.
// CI requires the tools. Other environments report an explicit skip if absent.
func New(t *testing.T, payloads ...[]byte) *Fixture {
	t.Helper()
	for _, tool := range []string{"git", "bun"} {
		if _, err := exec.LookPath(tool); err != nil {
			if os.Getenv("CATALOG_GIT_FIXTURE_REQUIRED") == "1" {
				t.Fatalf("required Git acquisition tool %s: %v", tool, err)
			}
			t.Skipf("Git acquisition requires %s: %v", tool, err)
		}
	}
	if len(payloads) == 0 {
		t.Fatal("Git fixture requires at least one payload")
	}
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(key), "GIT_") || strings.HasPrefix(strings.ToUpper(key), "BUN_") {
			t.Setenv(key, value)
			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
		}
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	t.Setenv("BUN_INSTALL_CACHE_DIR", filepath.Join(home, "bun-cache"))
	f := &Fixture{buildLog: filepath.Join(home, "builds")}
	t.Setenv("STARMAP_GIT_BUILD_LOG", f.buildLog)
	root := filepath.Join(home, "remote with spaces")
	if err := os.MkdirAll(root, constants.DirPermissions); err != nil {
		t.Fatal(err)
	}
	var bun struct{ Version, Platform, Arch string }
	if err := json.Unmarshal([]byte(run(t, root, "bun", "-e", `console.log(JSON.stringify({Version:Bun.version,Platform:process.platform,Arch:process.arch}))`)), &bun); err != nil {
		t.Fatal(err)
	}
	platform, arch := runtime.GOOS, runtime.GOARCH
	if platform == "windows" {
		platform = "win32"
	}
	if arch == "amd64" {
		arch = "x64"
	}
	if bun.Version != "1.3.12" || bun.Platform != platform || bun.Arch != arch {
		t.Fatalf("Bun version/platform/architecture = %+v, want 1.3.12/%s/%s", bun, platform, arch)
	}
	t.Logf("Git fixture tools: %s; Bun %s/%s/%s", strings.TrimSpace(run(t, root, "git", "--version")), bun.Version, bun.Platform, bun.Arch)
	for source, target := range map[string]string{"package.json": "package.json", "web-package.json": "packages/web/package.json", "bun.lock": "bun.lock", "build.ts": "packages/web/script/build.ts"} {
		data, err := inputs.ReadFile("testdata/" + source)
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, target), data)
		if source == "bun.lock" {
			f.LockfileChecksum = fmt.Sprintf("sha256:%x", sha256.Sum256(data))
		}
	}
	run(t, root, "git", "-c", "init.defaultBranch=main", "init", "--template=")
	run(t, root, "git", "config", "user.name", "Catalog fixture")
	run(t, root, "git", "config", "user.email", "fixture@example.invalid")
	run(t, root, "git", "config", "core.autocrlf", "false")
	for index, payload := range payloads {
		if !json.Valid(payload) {
			t.Fatalf("payload %d is invalid JSON", index)
		}
		write(t, filepath.Join(root, "packages/web/fixture.json"), payload)
		run(t, root, "git", "add", ".")
		run(t, root, "git", "-c", "commit.gpgsign=false", "commit", "-m", fmt.Sprintf("fixture %d", index))
		f.Commits = append(f.Commits, strings.TrimSpace(run(t, root, "git", "rev-parse", "HEAD")))
	}
	// Git accepts native absolute paths, including Windows drive letters and spaces.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url."+filepath.ToSlash(root)+".insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/sst/models.dev.git")
	return f
}

// BuildCount returns the number of completed real Bun builds.
func (f *Fixture) BuildCount(t *testing.T) int {
	t.Helper()
	data, err := os.ReadFile(f.buildLog)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "built\n")
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), constants.DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, constants.FilePermissions); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir, tool string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	var cmd *exec.Cmd
	switch tool {
	case "git":
		cmd = exec.CommandContext(ctx, "git", args...) //nolint:gosec // G204: Git fixture arguments come only from repository-owned test code.
	case "bun":
		cmd = exec.CommandContext(ctx, "bun", args...) //nolint:gosec // G204: Bun fixture arguments come only from repository-owned test code.
	default:
		t.Fatalf("unsupported fixture tool %q", tool)
		return ""
	}
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", tool, args, err, output)
	}
	return string(output)
}
