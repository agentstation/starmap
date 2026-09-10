package acquisition

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/test/gitfixture"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/runtime"
)

func TestRealPinnedGitSourceAcquisition(t *testing.T) {
	var payloads [][]byte
	for version := range 2 {
		payload := make(map[string]any)
		for _, id := range []string{"openai", "fixture-b", "fixture-c", "fixture-d", "fixture-e"} {
			models := make(map[string]any)
			for index := range 20 {
				modelID := fmt.Sprintf("model-%02d", index)
				if index == 0 {
					modelID = "gpt-image-2"
				}
				models[modelID] = map[string]any{"id": modelID, "name": "Fixture model", "description": fmt.Sprintf("Git revision %d. ", version) + strings.Repeat("Fixture metadata. ", 100), "temperature": version == 1, "limit": map[string]any{"context": 32768, "output": 4096}}
			}
			payload[id] = map[string]any{"id": id, "name": "Fixture provider", "models": models}
		}
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		payloads = append(payloads, data)
	}
	fixture := gitfixture.New(t, payloads...)
	acquirer, err := NewSourceAcquirer(pkgsync.WithSources(sources.ModelsDevGitID), pkgsync.WithModelsDevGitCommit(fixture.Commits[0]), pkgsync.WithSourcesDir(filepath.Join(t.TempDir(), "sources")), pkgsync.WithSkipDepPrompts(true))
	if err != nil {
		t.Fatal(err)
	}
	if fixture.BuildCount(t) != 0 {
		t.Fatal("constructor started acquisition")
	}
	var previous string
	for index, commit := range fixture.Commits {
		observations, err := acquirer.AcquireSources(t.Context(), runtime.SourceAcquisitionRequest{Current: acquisitionTestCatalog(t), Providers: []catalogs.ProviderID{"openai"}, ModelsDevGitCommit: &commit})
		if err != nil {
			t.Fatal(err)
		}
		if len(observations) != 1 {
			t.Fatalf("observations=%d", len(observations))
		}
		observation := observations[0]
		if observation.SourceID != sources.ModelsDevGitID || observation.Revision.Kind != sources.RevisionKindGitCommit || observation.Revision.Value != commit || observation.Revision.InputName != "bun.lock" || observation.Revision.InputChecksum != fixture.LockfileChecksum {
			t.Fatalf("unexpected revision: %+v", observation.Revision)
		}
		if observation.Status != sources.ObservationStatusSucceeded || observation.Completeness != sources.ObservationCompletenessComplete {
			t.Fatalf("source health: %s/%s", observation.Status, observation.Completeness)
		}
		if _, err := observation.Receipt(); err != nil {
			t.Fatal(err)
		}
		provider, err := observation.Catalog.Provider("openai")
		if err != nil {
			t.Fatal(err)
		}
		model := provider.Models["gpt-image-2"]
		if model == nil || !strings.HasPrefix(model.Description, fmt.Sprintf("Git revision %d. ", index)) {
			t.Fatalf("changed Git model missing: %+v", model)
		}
		if observation.ID == "" || observation.ID == previous {
			t.Fatal("changed source retained the previous receipt")
		}
		previous = observation.ID
	}
	if fixture.BuildCount(t) != 2 {
		t.Fatal("acquisition did not build both pinned revisions")
	}
}

func TestRequiredGitFixtureToolsCannotSkip(t *testing.T) {
	const marker = "CATALOG_GIT_FIXTURE_MISSING_TOOLS_CHILD"
	if os.Getenv(marker) == "1" {
		gitfixture.New(t, []byte("{}"))
		t.Fatal("required fixture accepted missing tools")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), executable, "-test.run=^TestRequiredGitFixtureToolsCannotSkip$", "-test.timeout=30s")
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "PATH") && name != "CATALOG_GIT_FIXTURE_REQUIRED" && name != marker {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "PATH="+t.TempDir(), "CATALOG_GIT_FIXTURE_REQUIRED=1", marker+"=1")
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "required Git acquisition tool git") || strings.Contains(string(output), "--- SKIP") {
		t.Fatalf("missing required tools did not fail: %v\n%s", err, output)
	}
}
