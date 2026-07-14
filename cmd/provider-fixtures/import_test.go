package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	responsefixtures "github.com/agentstation/starmap/internal/providers/fixtures/responses"
	"github.com/agentstation/starmap/pkg/constants"
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
	fixture := governedOpenAIFixture(t)
	if err := run(t.Context(), []string{"import", "--provider", "openai", "--source", "models", "--fixture", fixture, "--output", output}); err != nil {
		t.Fatalf("run: %v", err)
	}
	want := filepath.Join(output, "providers", "openai", "models", "gpt-3.5-turbo.yaml")
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("imported model is empty")
	}
}

func TestRunReplaysGovernedFixtureWithoutCredentials(t *testing.T) {
	fixture := governedOpenAIFixture(t)
	if err := run(t.Context(), []string{"replay", "--provider", "openai", "--source", "models", "--fixture", fixture}); err != nil {
		t.Fatalf("run replay: %v", err)
	}
}

func TestRunRejectsUngovernedAndTamperedFixturesBeforeImport(t *testing.T) {
	fixture := governedOpenAIFixture(t)
	output := t.TempDir()
	if err := os.WriteFile(fixture, []byte(`{"object":"list","data":[{"id":"tampered"}]}`), constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), []string{"import", "--provider", "openai", "--source", "models", "--fixture", fixture, "--output", output}); err == nil {
		t.Fatal("tampered governed fixture was imported")
	}
	if err := run(t.Context(), []string{"replay", "--provider", "openai", "--source", "models", "--fixture", fixture}); err == nil {
		t.Fatal("tampered governed fixture was replayed")
	}
	if entries, err := os.ReadDir(output); err != nil || len(entries) != 0 {
		t.Fatalf("import wrote output before verification: entries=%v err=%v", entries, err)
	}

	ungoverned := filepath.Join(t.TempDir(), "responses", "openai", "models", "models_list.json")
	if err := os.MkdirAll(filepath.Dir(ungoverned), constants.DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ungoverned, []byte(`{"object":"list","data":[]}`), constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), []string{"import", "--provider", "openai", "--source", "models", "--fixture", ungoverned, "--output", output}); err == nil {
		t.Fatal("metadata-less fixture was imported")
	}
}

func governedOpenAIFixture(t *testing.T) string {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("..", "..", "internal", "providers", "fixtures", "responses", "openai", "models", "models_list.json"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(t.TempDir(), "responses", "openai", "models", "models_list.json")
	_, err = responsefixtures.Refresh(context.Background(), responsefixtures.RefreshOptions{
		Provider: "openai", Source: "models", FixturePath: fixture, Now: time.Now().UTC(),
		Fetch: func(context.Context) (responsefixtures.FetchResult, error) {
			return responsefixtures.FetchResult{Payload: payload}, nil
		},
		Validate: func(context.Context, []byte) error { return nil },
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return fixture
}
