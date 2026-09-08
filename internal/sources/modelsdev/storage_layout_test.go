package modelsdev

import (
	"github.com/agentstation/starmap/pkg/productpaths"
	"os"
	"path/filepath"
	"testing"
)

func TestModelsDevCacheAndSourceCheckoutUseSeparatePassiveRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "local"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	httpClient := NewHTTPClient("")
	gitClient := NewGitClient("")
	cache, err := productpaths.UserDefaults(productpaths.Starmap)(productpaths.Cache)
	if err != nil {
		t.Fatal(err)
	}
	wantCache := filepath.Join(cache, "models.dev")
	wantSource := filepath.Join(cache, "sources", "models.dev-git")
	if httpClient.CacheDir != wantCache {
		t.Fatalf("HTTP cache = %q, want %q", httpClient.CacheDir, wantCache)
	}
	if gitClient.RepoPath != wantSource {
		t.Fatalf("Git source = %q, want %q", gitClient.RepoPath, wantSource)
	}
	for _, path := range []string{wantCache, wantSource} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("constructor created %q: %v", path, err)
		}
	}
}

func TestInvalidNativeSourceDirectoriesRefuseEffects(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("XDG_CACHE_HOME", "relative-cache")
	root := t.TempDir()
	t.Chdir(root)
	for _, name := range []string{"models.dev", "models.dev-git"} {
		if err := os.Mkdir(name, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(name, "preserve"), []byte("preserve"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	httpClient := NewHTTPClient("")
	gitClient := NewGitClient("")
	if _, err := httpClient.AcquireAPI(t.Context()); err == nil {
		t.Fatal("invalid native HTTP path allowed acquisition")
	}
	if _, err := gitClient.PrepareRepository(t.Context()); err == nil {
		t.Fatal("invalid native Git path allowed acquisition")
	}
	if err := httpClient.Cleanup(); err == nil {
		t.Fatal("invalid native HTTP path allowed cleanup")
	}
	if err := gitClient.Cleanup(); err == nil {
		t.Fatal("invalid native Git path allowed cleanup")
	}
	for _, name := range []string{"models.dev", "models.dev-git"} {
		actual, err := os.ReadFile(filepath.Join(name, "preserve"))
		if err != nil || string(actual) != "preserve" {
			t.Fatalf("path failure changed %s", name)
		}
	}
	explicit := filepath.Join(root, "explicit")
	if client := NewHTTPClient(explicit); client.pathErr != nil || client.CacheDir != filepath.Join(explicit, "models.dev") {
		t.Fatal("explicit HTTP path depended on native defaults")
	}
	if client := NewGitClient(explicit); client.pathErr != nil || client.RepoPath != filepath.Join(explicit, "models.dev-git") {
		t.Fatal("explicit Git path depended on native defaults")
	}
}
