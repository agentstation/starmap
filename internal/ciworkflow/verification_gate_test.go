package ciworkflow

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

type verificationWorkflow struct {
	Jobs map[string]struct {
		Name     string
		If       string
		Needs    any
		Strategy struct {
			FailFast bool `yaml:"fail-fast"`
			Matrix   struct {
				Group   []string
				Go      []string
				Suite   []string
				Include []map[string]string
			}
		}
		Env   map[string]string
		Steps []struct {
			Uses string
			With map[string]string
			Run  string
			Env  map[string]string
		}
	}
}

func readVerificationWorkflow(t *testing.T) verificationWorkflow {
	t.Helper()
	var workflow verificationWorkflow
	if err := yaml.Unmarshal([]byte(readFixture(t, "../../.github/workflows/pr.yaml")), &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow
}

func TestVerificationGateRequiresEverySuiteEvenAfterFailure(t *testing.T) {
	workflow := readVerificationWorkflow(t)
	gate := workflow.Jobs["verification"]
	if gate.Name != "Verification Gate" || gate.If != "always()" {
		t.Fatal("the required check must run after every prerequisite outcome")
	}
	want := []any{"verification-checks", "verification-tests", "verification-capacity"}
	if !reflect.DeepEqual(gate.Needs, want) || len(gate.Steps) != 1 {
		t.Fatalf("incomplete aggregate gate: %+v", gate)
	}
	step := gate.Steps[0]
	for variable, job := range map[string]string{"CHECKS": "verification-checks", "TESTS": "verification-tests", "CAPACITY": "verification-capacity"} {
		if step.Env[variable] != "${{ needs."+job+".result }}" {
			t.Fatalf("%s does not read the prerequisite result", variable)
		}
	}
	for _, result := range []string{"success", "failure", "cancelled", "skipped", ""} {
		for _, position := range []string{"CHECKS", "TESTS", "CAPACITY"} {
			t.Run(position+"/"+result, func(t *testing.T) {
				values := map[string]string{"CHECKS": "success", "TESTS": "success", "CAPACITY": "success"}
				values[position] = result
				command := exec.CommandContext(t.Context(), "sh", "-c", step.Run)
				command.Env = []string{"CHECKS=" + values["CHECKS"], "TESTS=" + values["TESTS"], "CAPACITY=" + values["CAPACITY"]}
				err := command.Run()
				if (err == nil) != (result == "success") {
					t.Fatalf("gate result for %s=%q: %v", position, result, err)
				}
			})
		}
	}
}

func TestVerificationMatrixPreservesPackageGroupsWithoutCompilerDuplicates(t *testing.T) {
	workflow := readVerificationWorkflow(t)
	job := workflow.Jobs["verification-tests"]
	if job.Needs != "verification-checks" || job.Strategy.FailFast {
		t.Fatal("test groups must follow cheap checks and retain independent failures")
	}
	matrix := job.Strategy.Matrix
	if !slices.Equal(matrix.Group, []string{"runtime", "client", "application", "contracts"}) {
		t.Fatalf("incomplete package or suite matrix: %+v", matrix)
	}
	if len(matrix.Go) != 0 || len(matrix.Suite) != 0 || len(matrix.Include) != 0 || job.Env["TEST_SUITE"] != "race" {
		t.Fatal("the matrix must run one complete race suite without compiler duplicates")
	}
	capacity := workflow.Jobs["verification-capacity"]
	if capacity.Needs != "verification-checks" {
		t.Fatal("full-catalog capacity must follow cheap checks")
	}
	found := false
	for _, step := range capacity.Steps {
		found = found || step.Run == `python3 scripts/verification_tests.py capacity --output "$RUNNER_TEMP/go-capacity-events.jsonl"`
	}
	if !found {
		t.Fatal("release-toolchain capacity execution is absent")
	}
}

func TestWorkflowGoPinsMatchProductToolchain(t *testing.T) {
	paths, err := filepath.Glob("../../.github/workflows/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	setups := 0
	for _, path := range paths {
		var workflow verificationWorkflow
		if err := yaml.Unmarshal([]byte(readFixture(t, path)), &workflow); err != nil {
			t.Fatal(err)
		}
		for _, job := range workflow.Jobs {
			for _, step := range job.Steps {
				if strings.HasPrefix(step.Uses, "actions/setup-go@") {
					setups++
					if step.With["go-version"] != "1.27.1" || step.With["go-version-file"] != "" {
						t.Fatalf("%s selects a different Go toolchain: %v", path, step.With)
					}
				}
			}
		}
	}
	if setups == 0 {
		t.Fatal("no Go setup steps found")
	}
}
