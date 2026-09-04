package ciworkflow

import (
	"testing"

	"github.com/goccy/go-yaml"
)

// catalogTransferBoundMinutes is the CAT-D12 catalog transfer bound. Every
// publisher limit must nest above it.
const catalogTransferBoundMinutes = 60

// catalogWorkflowDocument is the structural view of the publisher workflow that
// the nested timing bounds need.
type catalogWorkflowDocument struct {
	Jobs map[string]struct {
		TimeoutMinutes int `yaml:"timeout-minutes"`
		Steps          []struct {
			Name           string `yaml:"name"`
			TimeoutMinutes int    `yaml:"timeout-minutes"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

// TestCatalogGenerationWorkflowNestsTimeoutLimits proves the CAT-D12 bounds. The
// refresh step limit exceeds the catalog transfer bound, and the job limit
// exceeds the refresh step limit.
func TestCatalogGenerationWorkflowNestsTimeoutLimits(t *testing.T) {
	t.Parallel()

	var document catalogWorkflowDocument
	source := readFixture(t, "../../.github/workflows/catalog-generation.yaml")
	if err := yaml.Unmarshal([]byte(source), &document); err != nil {
		t.Fatalf("parse catalog generation workflow: %v", err)
	}

	job, found := document.Jobs["generate"]
	if !found {
		t.Fatal("catalog generation workflow has no generate job")
	}

	const (
		wantJobMinutes     = 90
		wantRefreshMinutes = 75
		refreshStepName    = "Refresh candidate catalog"
	)
	if job.TimeoutMinutes != wantJobMinutes {
		t.Fatalf("jobs.generate.timeout-minutes = %d, want %d", job.TimeoutMinutes, wantJobMinutes)
	}

	refreshMinutes := 0
	refreshFound := false
	for _, step := range job.Steps {
		if step.Name != refreshStepName {
			continue
		}
		if refreshFound {
			t.Fatalf("catalog generation workflow declares %q more than once", refreshStepName)
		}
		refreshFound = true
		refreshMinutes = step.TimeoutMinutes
	}
	if !refreshFound {
		t.Fatalf("catalog generation workflow has no %q step", refreshStepName)
	}
	if refreshMinutes != wantRefreshMinutes {
		t.Fatalf("%q timeout-minutes = %d, want %d", refreshStepName, refreshMinutes, wantRefreshMinutes)
	}

	if refreshMinutes <= catalogTransferBoundMinutes {
		t.Errorf("refresh step limit %d does not exceed the transfer bound %d",
			refreshMinutes, catalogTransferBoundMinutes)
	}
	if job.TimeoutMinutes <= refreshMinutes {
		t.Errorf("job limit %d does not exceed the refresh step limit %d",
			job.TimeoutMinutes, refreshMinutes)
	}
}
