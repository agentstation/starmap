package ciworkflow

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestEnterprisePRWorkflowPinsGoModToolchainAndRequiredGates(t *testing.T) {
	workflow := readFixture(t, "../../.github/workflows/enterprise-pr.yaml")
	module := readFixture(t, "../../go.mod")
	goVersion := regexp.MustCompile(`(?m)^go ([0-9]+\.[0-9]+\.[0-9]+)$`).FindStringSubmatch(module)
	if len(goVersion) != 2 {
		t.Fatal("go.mod does not declare an exact three-component Go version")
	}
	checks := []string{
		"name: Enterprise PR",
		"pull_request:",
		"name: Enterprise Gate",
		"name: Security & Reliability",
		`go-version: "` + goVersion[1] + `"`,
		"run: make verify",
		"govulncheck ./...",
		"FuzzParseAPIDataNoPanic",
		"FuzzSourceExtensionNoPanic",
		"FuzzReconciliationNoPanic",
		"Migration|Rollback|Fault|Corrupt|ReopensCurrent",
		"actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5",
		"actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff",
	}
	for _, check := range checks {
		if !strings.Contains(workflow, check) {
			t.Fatalf("enterprise workflow is missing %q", check)
		}
	}
	if strings.Contains(workflow, "go-version-file:") {
		t.Fatal("enterprise workflow must pin the exact three-component Go version explicitly")
	}
}

func readFixture(t testing.TB, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	return string(data)
}
