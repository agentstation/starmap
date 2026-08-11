package transport

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestTransportAuthenticationRegistriesUsePrimitives(t *testing.T) {
	providerConstant := regexp.MustCompile(`catalogs\.ProviderID[A-Z]`)
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller path is unavailable")
	}
	directories := []string{
		filepath.Dir(currentFile),
		filepath.Join(filepath.Dir(currentFile), "..", "providers", "clients"),
	}
	for _, directory := range directories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatalf("ReadDir(%s): %v", directory, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
				strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile(%s): %v", path, err)
			}
			if providerConstant.Match(contents) {
				t.Fatalf("production transport selection contains a provider roster: %s", path)
			}
		}
	}
}
