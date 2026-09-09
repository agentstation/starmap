package ciworkflow

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestGitAcquisitionQualificationRequiresNativeTools(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Env   map[string]string
			Steps []struct {
				Uses string
				With map[string]any
				Run  string
			}
		}
	}
	if err := yaml.Unmarshal([]byte(readFixture(t, "../../.github/workflows/pr.yaml")), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, owner := range []string{"verification", "native-runtime"} {
		job, ok := workflow.Jobs[owner]
		if !ok {
			t.Fatalf("missing job %s", owner)
		}
		if job.Env["CATALOG_GIT_FIXTURE_REQUIRED"] != "1" {
			t.Errorf("%s allows missing Git qualification tools", owner)
		}
		installed, reached := false, false
		for _, step := range job.Steps {
			if strings.HasPrefix(step.Uses, "oven-sh/setup-bun@") {
				if step.With["bun-version"] != "1.3.12" {
					t.Errorf("%s does not pin Bun 1.3.12", owner)
				}
				installed = true
			}
			if strings.Contains(step.Run, "go test ./...") || strings.Contains(step.Run, "./acquisition") {
				reached = true
				if !installed {
					t.Errorf("%s runs acquisition before installing Bun", owner)
				}
			}
		}
		if !installed || !reached {
			t.Errorf("%s omits Git acquisition qualification", owner)
		}
	}
}
