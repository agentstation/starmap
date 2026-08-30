package ciworkflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPullRequestWorkflowPinsToolchainActionsToolsAndRequiredJobs(t *testing.T) {
	workflow := readFixture(t, "../../.github/workflows/pr.yaml")
	module := readFixture(t, "../../go.mod")
	minimumVersion := regexp.MustCompile(`(?m)^go ([0-9]+\.[0-9]+\.[0-9]+)$`).FindStringSubmatch(module)
	if len(minimumVersion) != 2 {
		t.Fatal("go.mod does not declare an exact three-component Go version")
	}
	preferredVersion := regexp.MustCompile(`(?m)^toolchain go([0-9]+\.[0-9]+\.[0-9]+)$`).FindStringSubmatch(module)
	if len(preferredVersion) != 2 {
		t.Fatal("go.mod does not declare an exact preferred Go toolchain")
	}
	minimumPatchVersion := "1.25.12"
	if minimumVersion[1] != "1.25.0" {
		t.Fatalf("minimum Go language version = %q, want 1.25.0", minimumVersion[1])
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
		"  action-pins:",
		"name: Action Pin Provenance",
		"run: make verify-action-pins",
		"name: Test minimum supported Go version",
		"name: Test minimum supported external consumer",
		"CGO_ENABLED: 0",
		`go-version: "` + minimumPatchVersion + `"`,
		"GOTOOLCHAIN: local",
		"run: make test-consumer-deps",
		`go-version: "` + preferredVersion[1] + `"`,
		"run: make verify",
		"golangci-lint@v2.12.2",
		"gomarkdoc@v1.1.0",
		"govulncheck@v1.6.0",
		"govulncheck ./...",
		"FuzzParseAPIDataNoPanic",
		"FuzzSourceExtensionNoPanic",
		"FuzzReconciliationNoPanic",
		"Migration|Rollback|Fault|Corrupt|ReopensCurrent",
	}
	for _, check := range checks {
		if !strings.Contains(workflow, check) {
			t.Fatalf("pull request workflow is missing %q", check)
		}
	}
	requireSHAPinnedActions(t, workflow, "pull request",
		"actions/checkout",
		"actions/setup-go",
	)
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
		`VERIFY_CATALOG_DATABASE_PATH="$TMPDIR/catalog"`,
		`VERIFY_HOME="$TMPDIR/home"`,
		`GOLANGCI_LINT_CACHE="$TMPDIR/golangci-lint-cache"`,
		`export GOLANGCI_LINT_CACHE`,
		`GOLANGCI_LINT_VERSION="2.12.2"`,
		`run make test-pure-go`,
		`run make test-file-sizes`,
		`run env CGO_ENABLED=1 go test ./... -race -short -timeout=20m`,
		`CATALOG_PATH="$VERIFY_CATALOG_DATABASE_PATH" CATALOG_EXPORT_PATH="$VERIFY_CATALOG_PATH"`,
		`env -i`,
		`PATH="$PATH"`,
		`CLOUDSDK_CONFIG="$VERIFY_HOME/.config/gcloud"`,
		`HOME="$VERIFY_HOME"`,
		`XDG_CONFIG_HOME="$VERIFY_HOME/.config"`,
	} {
		if !strings.Contains(verifyScript, check) {
			t.Fatalf("repository verification script is missing isolated catalog state %q", check)
		}
	}
	if strings.Contains(verifyScript, "skipping golangci-lint") {
		t.Fatal("repository verification must not silently skip its pinned linter")
	}
	if strings.Contains(verifyScript, "\n\t\t-u ") {
		t.Fatal("credential-free verification must not maintain a provider environment roster")
	}
}

func TestGoFileSizeVerificationEnforcesReviewAndHardLimits(t *testing.T) {
	script, err := filepath.Abs("../../scripts/verify-go-file-sizes.sh")
	if err != nil {
		t.Fatalf("Abs script: %v", err)
	}
	for _, test := range []struct {
		name          string
		lines         int
		generated     bool
		justification bool
		wantSuccess   bool
		wantListed    bool
	}{
		{name: "normal", lines: 1000, wantSuccess: true},
		{name: "review", lines: 1001, wantSuccess: true, wantListed: true},
		{name: "missing rationale", lines: 1501, wantListed: true},
		{name: "durable rationale", lines: 1501, justification: true, wantSuccess: true, wantListed: true},
		{name: "hard limit ignores rationale", lines: 2000, justification: true, wantListed: true},
		{name: "generated excluded", lines: 2000, generated: true, wantSuccess: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := strings.Repeat("// authored\n", test.lines)
			if test.generated {
				source = "// Code generated by fixture. DO NOT EDIT.\n" +
					strings.Repeat("// generated\n", test.lines-1)
			}
			if err := os.WriteFile(
				filepath.Join(root, "large.go"),
				[]byte(source),
				0o600,
			); err != nil {
				t.Fatalf("WriteFile source: %v", err)
			}
			reviews := filepath.Join(root, "reviews.tsv")
			reviewData := "# path\trationale\n"
			if test.justification {
				reviewData += "large.go\tcohesive deep fixture rationale\n"
			}
			if err := os.WriteFile(reviews, []byte(reviewData), 0o600); err != nil {
				t.Fatalf("WriteFile reviews: %v", err)
			}

			command := exec.Command("bash", script)
			command.Env = append(
				os.Environ(),
				"STARMAP_GO_FILE_SIZE_ROOT="+root,
				"STARMAP_GO_FILE_SIZE_REVIEWS="+reviews,
			)
			output, runErr := command.CombinedOutput()
			if (runErr == nil) != test.wantSuccess {
				t.Fatalf("script error = %v, want success %t\n%s", runErr, test.wantSuccess, output)
			}
			listed := strings.Contains(string(output), "\tlarge.go")
			if listed != test.wantListed {
				t.Fatalf("listed = %t, want %t\n%s", listed, test.wantListed, output)
			}
		})
	}
}

func TestExternalReadOnlyConsumerUsesCanonicalCatalogDX(t *testing.T) {
	consumer := readFixture(t, "../../testdata/consumers/read-only/consumer.go")
	for _, check := range []string{
		"sm, err := starmap.New()",
		"catalog := sm.Catalog()",
		`catalog.FindModel("gpt-4o")`,
	} {
		if !strings.Contains(consumer, check) {
			t.Fatalf("external read-only consumer is missing %q", check)
		}
	}
	for _, forbidden := range []string{
		"Snapshot",
		"catalog, err := sm.Catalog()",
		"acquisition",
	} {
		if strings.Contains(consumer, forbidden) {
			t.Fatalf("external read-only consumer contains forbidden surface %q", forbidden)
		}
	}
}

func TestReadOnlyConsumerDependencyBudgetIsPlatformIndependent(t *testing.T) {
	script := readFixture(t, "../../scripts/verify-consumer-deps.sh")
	for _, check := range []string{
		"MAX_NON_STANDARD_PACKAGES=32",
		"{{if not .Standard}}{{.ImportPath}}{{end}}",
		`"$non_standard_package_count" -gt "$MAX_NON_STANDARD_PACKAGES"`,
		`banned="$(find_banned_dependencies "$banned_pattern" "$DEPS")"`,
		`grep -Fvx 'github.com/agentstation/starmap/pkg/sources/payload'`,
	} {
		if !strings.Contains(script, check) {
			t.Fatalf("consumer dependency verification is missing %q", check)
		}
	}
	if strings.Contains(script, "MAX_PACKAGES=160") {
		t.Fatal("read-only dependency budget must not count platform-specific standard-library packages")
	}
}

func TestExternalStoreConsumerUsesCallerOwnedAdapter(t *testing.T) {
	consumer := readFixture(t, "../../testdata/consumers/store-only/consumer.go")
	store := readFixture(t, "../../testdata/consumers/store-only/starport_store.go")
	script := readFixture(t, "../../scripts/verify-consumer-deps.sh")

	if !strings.Contains(consumer, "starmap.WithCatalogStore(store)") {
		t.Fatal("external store consumer does not inject its caller-owned store")
	}
	if strings.Contains(consumer, "storage.NewMemory") {
		t.Fatal("external store consumer still delegates to a Starmap-owned adapter")
	}
	if !strings.Contains(store, "var _ storage.Store = (*starportStore)(nil)") {
		t.Fatal("external Starport-style adapter lacks a compile-time Store assertion")
	}
	for _, check := range []string{
		`store_banned_pattern=`,
		`database/sql`,
		`go-sql-driver/mysql`,
		`lib/pq`,
		`jackc/pgx`,
		`modernc\.org/sqlite`,
		`github\.com/aws/(aws-sdk-go-v2|smithy-go)`,
	} {
		if !strings.Contains(script, check) {
			t.Fatalf("store-only dependency verification is missing %q", check)
		}
	}
}

func TestPinnedArtifactConsumerIsOfflineAndDependencyBounded(t *testing.T) {
	consumer := readFixture(
		t,
		"../../testdata/consumers/pinned-artifact/consumer.go",
	)
	script := readFixture(t, "../../scripts/verify-consumer-deps.sh")
	for _, check := range []string{
		"artifact.VerifyRelease",
		"pinnedVerifier{digest:",
		"client.Activate(ctx, verified)",
	} {
		if !strings.Contains(consumer, check) {
			t.Fatalf("pinned-artifact consumer is missing %q", check)
		}
	}
	for _, forbidden := range []string{
		`"github.com/agentstation/starmap/acquisition"`,
		`"github.com/agentstation/starmap/remote"`,
		`"github.com/agentstation/starmap/server"`,
		"http.",
	} {
		if strings.Contains(consumer, forbidden) {
			t.Fatalf("pinned-artifact consumer contains online surface %q", forbidden)
		}
	}
	for _, check := range []string{
		`PINNED_ARTIFACT_MODULE=`,
		`STARMAP_RELEASE_GOTOOLCHAIN`,
		`PINNED_MAX_NON_STANDARD_PACKAGES=32`,
		`pinned_banned_pattern=`,
		`starmap/pkg/catalogs/artifact`,
		`starmap/pkg/catalogs/storage/s3`,
		`google\.golang\.org/(genai|grpc)`,
	} {
		if !strings.Contains(script, check) {
			t.Fatalf("pinned-artifact dependency verification is missing %q", check)
		}
	}
}

func TestExternalServerStorageMatrixStaysOptional(t *testing.T) {
	storage := readFixture(t, "../../testdata/consumers/server-storage/storage.go")
	script := readFixture(t, "../../scripts/verify-consumer-deps.sh")
	for _, check := range []string{
		"StorageFilesystem StorageMode",
		"StorageObject StorageMode",
		"s3store.New(c.S3Client",
		"storage.NewObject(backend, c.ObjectPrefix)",
	} {
		if !strings.Contains(storage, check) {
			t.Fatalf("external server storage composition is missing %q", check)
		}
	}
	for _, check := range []string{
		`SERVER_STORAGE_MODULE=`,
		`SERVER_STORAGE_MAX_PACKAGES=340`,
		`go list -deps -test`,
		`starmap/pkg/catalogs/storage/s3`,
		`starmap/remote`,
		`starmap/server`,
	} {
		if !strings.Contains(script, check) {
			t.Fatalf("server-storage dependency verification is missing %q", check)
		}
	}
}

func TestPureGoAndRaceVerificationHaveSeparateCgoModes(t *testing.T) {
	makefile := readFixture(t, "../../Makefile")
	pureGoScript := readFixture(t, "../../scripts/verify-pure-go.sh")
	verifyScript := readFixture(t, "../../scripts/verify.sh")

	if !strings.Contains(makefile, "test-pure-go:") {
		t.Fatal("Makefile does not expose the pure-Go composition gate")
	}
	for _, check := range []string{
		`git -C "$ROOT" grep`,
		`CGO_ENABLED=0 "$ROOT/scripts/verify-consumer-deps.sh"`,
		`CGO_ENABLED=0 go test ./pkg/catalogs/storage/s3`,
		`CGO_ENABLED=0 go build -trimpath`,
		`CGO_ENABLED=0$`,
		`import[[:space:]]+"C"`,
	} {
		if !strings.Contains(pureGoScript, check) {
			t.Fatalf("pure-Go verification is missing %q", check)
		}
	}
	if strings.Contains(pureGoScript, "rg -n") {
		t.Fatal("pure-Go verification must use tooling available on the hosted runner")
	}
	if !strings.Contains(verifyScript, "run make test-pure-go") {
		t.Fatal("repository verification does not run the pure-Go composition gate")
	}
	if !strings.Contains(verifyScript, "run env CGO_ENABLED=1 go test ./... -race") {
		t.Fatal("repository race verification must remain explicitly cgo-enabled")
	}
}

func TestGolangCILintVersionIsConsistentAcrossVerificationSurfaces(t *testing.T) {
	const version = "2.12.2"
	fixtures := map[string]string{
		"Devbox":           "../../devbox.json",
		"Makefile":         "../../Makefile",
		"verification":     "../../scripts/verify.sh",
		"pull request":     "../../.github/workflows/pr.yaml",
		"release workflow": "../../.github/workflows/release.yaml",
	}
	for name, path := range fixtures {
		if contents := readFixture(t, path); !strings.Contains(contents, version) {
			t.Errorf("%s does not pin golangci-lint %s", name, version)
		}
	}
}

func TestLocalBuildVersionIgnoresCatalogGenerationTags(t *testing.T) {
	makefile := readFixture(t, "../../Makefile")
	if !strings.Contains(makefile, "git describe --tags --abbrev=0 --match 'v[0-9]*'") {
		t.Fatal("local builds must derive application versions from v* tags only")
	}

	goreleaser := readFixture(t, "../../.goreleaser.yaml")
	for _, catalogTag := range []string{"catalog-payload-*", "catalog-semantic-*"} {
		if !strings.Contains(goreleaser, catalogTag) {
			t.Fatalf("GoReleaser does not ignore catalog generation tag %q", catalogTag)
		}
	}
}

func TestOpenAPICheckPinsSelfContainedGenerator(t *testing.T) {
	makefile := readFixture(t, "../../Makefile")
	for _, check := range []string{
		"SWAG_VERSION=2.0.0-rc4",
		"SWAG_RUN=$(GOCMD) run github.com/swaggo/swag/v2/cmd/swag@v$(SWAG_VERSION)",
		"openapi-check:",
		"$(SWAG_RUN) init",
	} {
		if !strings.Contains(makefile, check) {
			t.Fatalf("OpenAPI verification is missing %q", check)
		}
	}
	if strings.Contains(makefile, "which swag") {
		t.Fatal("OpenAPI verification must not depend on an ambient swag binary")
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
