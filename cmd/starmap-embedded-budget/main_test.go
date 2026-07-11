package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/internal/embeddedbudget"
)

func TestEmbeddedBudgetCommandEmitsPassingMachineReadableReport(t *testing.T) {
	generation, err := bootstrap.Generation()
	if err != nil {
		t.Fatalf("bootstrap.Generation: %v", err)
	}
	var output bytes.Buffer
	if err := run(nil, &output, func(string) string { return "" }, generation.Manifest.GeneratedAt.Add(time.Hour)); err != nil {
		t.Fatalf("run: %v", err)
	}
	var report embeddedbudget.Report
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("Unmarshal report: %v", err)
	}
	if !report.Passed || report.CompressedBytes <= 0 || report.UncompressedBytes <= 0 || report.AgeSeconds <= 0 {
		t.Fatalf("report = %#v", report)
	}
}

func TestEmbeddedBudgetOverrideRequiresReviewReason(t *testing.T) {
	values := map[string]string{envMaxAge: "8760h"}
	getenv := func(name string) string { return values[name] }
	if _, _, err := limitsFromEnvironment(getenv); err == nil {
		t.Fatal("limitsFromEnvironment accepted override without review reason")
	}
	values[envOverrideReason] = "approved in catalog size review"
	limits, reason, err := limitsFromEnvironment(getenv)
	if err != nil {
		t.Fatalf("limitsFromEnvironment: %v", err)
	}
	if limits.MaxAge != 365*24*time.Hour || reason != values[envOverrideReason] {
		t.Fatalf("limits/reason = %#v/%q", limits, reason)
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
