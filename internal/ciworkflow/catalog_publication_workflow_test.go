package ciworkflow

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestCatalogGenerationWorkflowRetainsOnePublicationBeforePromotion(t *testing.T) {
	var document struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Name string            `yaml:"name"`
				If   string            `yaml:"if"`
				Env  map[string]string `yaml:"env"`
				Run  string            `yaml:"run"`
				Uses string            `yaml:"uses"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(readFixture(t, "../../.github/workflows/catalog-generation.yaml")), &document); err != nil {
		t.Fatal(err)
	}
	job := document.Jobs["generate"]
	if _, exists := job.Env["CATALOG_PUBLICATION_DIRECTORY"]; exists {
		t.Fatal("publication directory must be selected after the runner starts")
	}
	positions := make(map[string]int)
	operations := []string{"inspect", "prepare", "validate", "begin", "publish", "promote", "channels", "finish"}
	for index, step := range job.Steps {
		if _, duplicate := positions[step.Name]; duplicate {
			t.Fatal("publisher step names must identify one operation")
		}
		positions[step.Name] = index
		if strings.HasPrefix(step.Uses, "actions/checkout@") && step.With["persist-credentials"] != "false" {
			t.Fatal("publisher checkout retains acquisition credentials")
		}
		for name := range step.Env {
			if strings.HasSuffix(name, "_API_KEY") && step.Name != "Refresh candidate catalog" {
				t.Fatal("provider credentials reached a publication or promotion step")
			}
		}
		if _, overridden := step.Env["CATALOG_PUBLICATION_DIRECTORY"]; overridden {
			t.Fatal("publisher phase overrides its shared preparation directory")
		}
		if step.Name == "Select publication directory" {
			if !strings.Contains(step.Run, "CATALOG_PUBLICATION_DIRECTORY=") || !strings.Contains(step.Run, `"$RUNNER_TEMP"`) || !strings.Contains(step.Run, `>> "$GITHUB_ENV"`) {
				t.Fatal("publisher directory does not use runner temporary storage")
			}
		}
		if strings.Contains(step.Run, "scripts/catalog_publication.py") {
			if !strings.Contains(step.Run, `--directory "$CATALOG_PUBLICATION_DIRECTORY"`) {
				t.Fatal("publication phase does not select the shared state directory")
			}
			if len(operations) == 0 || !strings.Contains(step.Run, "scripts/catalog_publication.py "+operations[0]+" ") {
				t.Fatal("publisher phases do not preserve preparation and promotion order")
			}
			operations = operations[1:]
			if (strings.Contains(step.Run, "catalog_publication.py prepare ") || strings.Contains(step.Run, "catalog_publication.py validate ")) && step.If != "steps.publication.outputs.acquire == 'true'" {
				t.Fatal("recovery can reacquire or replace its retained preparation")
			}
		}
		if step.Name == "Retain acquisition corrections and validation logs" {
			if step.If != "always() && steps.publication.outputs.active == 'true'" ||
				!strings.HasPrefix(step.Uses, "actions/upload-artifact@") ||
				step.With["path"] != "${{ env.CATALOG_PUBLICATION_DIRECTORY }}/*.log" {
				t.Fatal("publisher does not retain validation and recovery diagnostics after failure")
			}
		}
		if step.Name == "Retain preparation before public writes" && !strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
			t.Fatal("publisher does not retain its original preparation through an immutable workflow artifact")
		}
		if step.Name == "Retain preparation before public writes" && step.With["retention-days"] != "90" {
			t.Fatal("publisher recovery input does not retain the declared 90-day lifetime")
		}
		if strings.Contains(step.Name, "channels") && step.If != "steps.promotion.outputs.ready == 'true'" {
			t.Fatal("channel publication does not require a verified promotion")
		}
	}
	if len(operations) != 0 {
		t.Fatal("workflow omits a publication phase")
	}
	order := []string{"Select publication directory", "Restore accepted or pending publication", "Refresh candidate catalog",
		"Validate candidate before publication", "Attest exact publication inputs", "Retain preparation before public writes", "Record pending publication",
		"Publish and verify immutable public inputs", "Create promotion installation token", "Promote exact input through checked pull request",
		"Stage channels after verified merge", "Attest both discovery channels", "Publish and verify both discovery channels",
		"Retain acquisition corrections and validation logs"}
	last := -1
	for _, name := range order {
		position, found := positions[name]
		if !found || position <= last {
			t.Fatalf("publisher phase %s is absent or out of order", name)
		}
		last = position
	}
}
