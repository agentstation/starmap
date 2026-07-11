package ciworkflow

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPullRequestWorkflowPinsToolchainActionsToolsAndRequiredJobs(t *testing.T) {
	workflow := readFixture(t, "../../.github/workflows/pr.yaml")
	module := readFixture(t, "../../go.mod")
	goVersion := regexp.MustCompile(`(?m)^go ([0-9]+\.[0-9]+\.[0-9]+)$`).FindStringSubmatch(module)
	if len(goVersion) != 2 {
		t.Fatal("go.mod does not declare an exact three-component Go version")
	}
	checks := []string{
		"name: Pull Request",
		"pull_request:",
		"branches:\n      - main",
		"workflow_dispatch:",
		"group: pr-",
		"  verification:",
		"name: Verification Gate",
		"name: Run verification gate",
		"  security-reliability:",
		"name: Security & Reliability",
		`go-version: "` + goVersion[1] + `"`,
		"run: make verify",
		"golangci-lint@v2.5.0",
		"gomarkdoc@v1.1.0",
		"govulncheck@v1.6.0",
		"govulncheck ./...",
		"FuzzParseAPIDataNoPanic",
		"FuzzSourceExtensionNoPanic",
		"FuzzReconciliationNoPanic",
		"Migration|Rollback|Fault|Corrupt|ReopensCurrent",
		"actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0",
		"actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16",
	}
	for _, check := range checks {
		if !strings.Contains(workflow, check) {
			t.Fatalf("pull request workflow is missing %q", check)
		}
	}
	if strings.Contains(workflow, "go-version-file:") {
		t.Fatal("pull request workflow must pin the exact three-component Go version explicitly")
	}
}

func TestPullRequestWorkflowIsTheOnlyActivePRWorkflow(t *testing.T) {
	workflows, err := filepath.Glob("../../.github/workflows/*.yaml")
	if err != nil {
		t.Fatalf("Glob workflows: %v", err)
	}
	var pullRequestWorkflows []string
	for _, workflowPath := range workflows {
		workflow := readFixture(t, workflowPath)
		if strings.Contains(workflow, "\n  pull_request:") {
			pullRequestWorkflows = append(pullRequestWorkflows, filepath.Base(workflowPath))
		}
	}
	if len(pullRequestWorkflows) != 1 || pullRequestWorkflows[0] != "pr.yaml" {
		t.Fatalf("active pull request workflows = %v, want [pr.yaml]", pullRequestWorkflows)
	}
}

func TestMakeVerifyUsesCanonicalVerificationScript(t *testing.T) {
	makefile := readFixture(t, "../../Makefile")
	verifyScript := readFixture(t, "../../scripts/verify.sh")
	verifyRecipe := regexp.MustCompile(`(?m)^verify:.*\n\t@\./scripts/verify\.sh$`)
	if !verifyRecipe.MatchString(makefile) {
		t.Fatal("make verify must invoke scripts/verify.sh directly")
	}

	info, err := os.Stat("../../scripts/verify.sh")
	if err != nil {
		t.Fatalf("Stat verification script: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("scripts/verify.sh must be executable")
	}

	for _, check := range []string{
		`VERIFY_CATALOG_PATH="$ROOT/internal/embedded/catalog"`,
		`VERIFY_CATALOG_STORE_PATH="$TMPDIR/catalog-store"`,
		`CATALOG_STORE_PATH="$VERIFY_CATALOG_STORE_PATH" LOCAL_PATH="$VERIFY_CATALOG_PATH"`,
	} {
		if !strings.Contains(verifyScript, check) {
			t.Fatalf("repository verification script is missing isolated catalog state %q", check)
		}
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
