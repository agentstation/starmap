package modelsdev

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModelsDevCacheAndSourceCheckoutUseSeparatePassiveRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	httpClient := NewHTTPClient("")
	gitClient := NewGitClient("")
	wantCache := filepath.Join(home, ".starmap", "cache", "models.dev")
	wantSource := filepath.Join(home, ".starmap", "sources", "models.dev-git")
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
