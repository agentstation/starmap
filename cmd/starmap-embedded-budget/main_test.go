package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/bootstrap/budget"
)

func TestEmbeddedBudgetCommandEmitsPassingMachineReadableReport(t *testing.T) {
	generation, err := bootstrap.Generation()
	if err != nil {
		t.Fatalf("bootstrap.Generation: %v", err)
	}
	var output bytes.Buffer
	if err := run(nil, &output, generation.Manifest.GeneratedAt.Add(time.Hour)); err != nil {
		t.Fatalf("run: %v", err)
	}
	var report budget.Report
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("Unmarshal report: %v", err)
	}
	if !report.Passed || report.SchemaVersion != budget.CurrentPolicyVersion ||
		report.Policy.Version != budget.CurrentPolicyVersion || report.CompressedBytes <= 0 ||
		report.UncompressedBytes <= 0 || report.AgeSeconds <= 0 {
		t.Fatalf("report = %#v", report)
	}
}

func TestEmbeddedBudgetCommandRejectsArguments(t *testing.T) {
	if err := run([]string{"override"}, &bytes.Buffer{}, time.Now()); err == nil {
		t.Fatal("run accepted an unsupported policy override")
	}
}

func TestEmbeddedBudgetReleaseWorkflowRunsCheckedInGate(t *testing.T) {
	workflow, err := os.ReadFile("../../.github/workflows/release.yaml")
	if err != nil {
		t.Fatalf("ReadFile release workflow: %v", err)
	}
	for _, required := range []string{"Check embedded catalog budgets", "make embedded-catalog-budget-check"} {
		if !strings.Contains(string(workflow), required) {
			t.Errorf("release workflow is missing %q", required)
		}
	}
}
