package ciworkflow

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
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
	if _, exists := job.Env["STARMAP_GENERATION_STORE_PATH"]; exists {
		t.Fatal("publisher store must be selected after the runner starts")
	}
	selected, refreshFound, stageFound := false, false, false
	for _, step := range job.Steps {
		if step.Name == "Select catalog store" {
			if selected || !strings.Contains(step.Run, "STARMAP_GENERATION_STORE_PATH=") ||
				!strings.Contains(step.Run, `"$RUNNER_TEMP"`) ||
				!strings.Contains(step.Run, `>> "$GITHUB_ENV"`) {
				t.Fatal("publisher store must be selected once from runner temporary state")
			}
			selected = true
		}
		if step.Name != "Refresh candidate catalog" && step.Name != "Stage validated immutable generation" {
			continue
		}
		if !selected {
			t.Fatal("publisher phase runs before shared store selection")
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
