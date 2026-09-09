package ciworkflow

import (
	"github.com/goccy/go-yaml"
	"strings"
	"testing"
)

func TestCatalogGenerationWorkflowSharesAcquisitionAndStagingStore(t *testing.T) {
	var document struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Name string            `yaml:"name"`
				Env  map[string]string `yaml:"env"`
				Run  string            `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(readFixture(t, "../../.github/workflows/catalog-generation.yaml")), &document); err != nil {
		t.Fatal(err)
	}
	job := document.Jobs["generate"]
	store := job.Env["STARMAP_GENERATION_STORE_PATH"]
	if !strings.HasPrefix(store, "${{ runner.temp }}/") {
		t.Fatal("publisher store is not explicit job-owned temporary state")
	}
	refreshFound, stageFound := false, false
	for _, step := range job.Steps {
		if step.Name != "Refresh candidate catalog" && step.Name != "Stage validated immutable generation" {
			continue
		}
		if _, overridden := step.Env["STARMAP_GENERATION_STORE_PATH"]; overridden {
			t.Fatal("publisher phase replaces the shared store")
		}
		if step.Name == "Refresh candidate catalog" {
			refreshFound = strings.Contains(step.Run, "./scripts/generate-embedded-catalog.sh")
		}
		if step.Name == "Stage validated immutable generation" {
			stageFound = strings.Contains(step.Run, `--generation-store "${STARMAP_GENERATION_STORE_PATH}"`)
		}
	}
	if !refreshFound || !stageFound {
		t.Fatal("acquisition and staging do not use the same store contract")
	}
}
