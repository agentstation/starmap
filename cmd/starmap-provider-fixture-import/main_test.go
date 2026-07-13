package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeModelPath(t *testing.T) {
	path, err := safeModelPath("hyperbolic", "meta-llama/Llama-3")
	if err != nil {
		t.Fatalf("safeModelPath: %v", err)
	}
	want := filepath.Join("providers", "hyperbolic", "models", "meta-llama", "Llama-3.yaml")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	for _, id := range []string{"", "../secret", "/absolute", `org\model`} {
		if _, err := safeModelPath("provider", id); err == nil {
			t.Fatalf("safeModelPath accepted %q", id)
		}
	}
}

func TestRunImportsFixtureWithoutCredentials(t *testing.T) {
	output := t.TempDir()
	fixture := filepath.Join("..", "..", "internal", "providers", "hyperbolic", "testdata", "models_list.json")
	if err := run(t.Context(), []string{"--provider", "hyperbolic", "--fixture", fixture, "--output", output}); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := filepath.Join(output, "providers", "hyperbolic", "models", "meta-llama", "Llama-3.3-70B-Instruct.yaml")
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("imported model is empty")
	}
}
