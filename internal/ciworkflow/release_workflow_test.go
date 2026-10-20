package ciworkflow

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestReleaseWorkflowPinsToolchainPublisherAndVerification(t *testing.T) {
	workflow := readFixture(t, "../../.github/workflows/release.yaml")
	checks := []string{
		"name: Release",
		"tags:\n      - \"v*\"",
		"workflow_dispatch:",
		"Existing release tag to publish or recover",
		"Failed Release run that owns the exact dist artifact. Leave empty to build and publish the tag again.",
		"group: release-${{ inputs.tag || github.ref_name }}",
		"RELEASE_TAG: ${{ inputs.tag || github.ref_name }}",
		"if: github.event_name == 'push' || inputs.source_run_id == ''",
		"if: github.event_name == 'workflow_dispatch' && inputs.source_run_id != ''",
		"ref: ${{ inputs.tag || github.ref }}",
		"Require release tag from main",
		"Require the release tag at the checkout",
		"a release for $RELEASE_TAG already exists",
		"GORELEASER_CURRENT_TAG: ${{ env.RELEASE_TAG }}",
		"permissions:\n  contents: read",
		"    permissions:\n      attestations: write\n      contents: write\n      discussions: write\n      id-token: write\n      packages: write",
		`go-version: "1.27.2"`,
		"git merge-base --is-ancestor",
		"golangci-lint@v2.14.0",
		"name: Checkout pinned technical-writing skill",
		"repository: agentstation/skills",
		"ref: dc9948f59089426c7dd077e41469104ec788cf7f",
		"path: .ci/agentstation-skills",
		"persist-credentials: false",
		"TECHNICAL_WRITING: ${{ github.workspace }}/.ci/agentstation-skills/technical-writing/scripts/technical-writing",
		"make verify-checks",
		"make release-check",
		"syft-version: v1.46.0",
		"version: v2.17.0",
		"Verify portable release binaries",
		"./scripts/verify-release-binaries.sh dist",
		"subject-checksums: dist/checksums.txt",
		"Verify draft release assets before publication",
		"Publish verified immutable release",
		`gh release edit "$RELEASE_TAG" --draft=false`,
		`--jq .immutable`,
		"gpg --batch --verify checksums.txt.sig checksums.txt",
		"sha256sum --check checksums.txt",
		`--repo "$GITHUB_REPOSITORY"`,
		`--signer-workflow "$GITHUB_REPOSITORY/.github/workflows/release.yaml"`,
		"Validate the recovery source and release",
		`test "$(jq -r .headSha <<<"$SOURCE_RUN")" = "$TAG_SHA"`,
		`repos/$GITHUB_REPOSITORY/releases?per_page=100`,
		`test "$LATEST_APPLICATION_TAG" = "$RELEASE_TAG"`,
		`echo "RELEASE_ID=$(jq -r '.[0].id' <<<"$RELEASES")" >> "$GITHUB_ENV"`,
		`echo "PUBLISH_RELEASE=$PUBLISH_RELEASE" >> "$GITHUB_ENV"`,
		"if: env.PUBLISH_RELEASE == 'true'",
		`gh run download "$SOURCE_RUN_ID" --name release-dist --dir dist`,
		`test "$(jq -r .commit dist/metadata.json)" = "$(git rev-list -n 1 "$RELEASE_TAG")"`,
		"Publish the generated Homebrew cask with a deploy key",
		`git -C "$TAP_DIRECTORY" diff --cached --quiet -- Casks/starmap.rb`,
		"Attest recovered archives and SBOMs",
		"Verify exact release assets and provenance",
		`repos/$GITHUB_REPOSITORY/releases/assets/$asset_id`,
		"expected-release-assets.txt",
		"actual-release-assets.txt",
		"Publish the recovered immutable release",
		`repos/$GITHUB_REPOSITORY/releases/$RELEASE_ID`,
		"Verify recovered public assets and publisher identity",
		"verify-homebrew-recovery:",
		"brew install agentstation/tap/starmap",
		`go version -m "$BINARY"`,
		`grep -Ev '^(/usr/lib/|/System/Library/)'`,
		"!contains(inputs.tag || github.ref_name, '-')",
		"inputs.source_run_id != '' && !contains(inputs.tag, '-')",
	}
	for _, check := range checks {
		if !strings.Contains(workflow, check) {
			t.Errorf("release workflow is missing %q", check)
		}
	}
	requireSHAPinnedActions(t, workflow, "release",
		"goreleaser/goreleaser-action",
		"docker/login-action",
		"anchore/sbom-action/download-syft",
		"actions/attest-build-provenance",
	)
	for _, forbidden := range []string{
		"go-version-file:",
		"check-homebrew-eligibility",
		"starmap-catalog-release",
		"STARMAP_CATALOG_OCI_MIRROR",
		"permissions:\n  attestations: write",
		`RELEASE=$(gh api "repos/$GITHUB_REPOSITORY/releases/tags/$RELEASE_TAG")`,
		"HEAD:master",
		"$GITHUB_REF_NAME",
		"timeout-minutes: 75",
		"timeout-minutes: 180",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("release workflow contains obsolete coupling %q", forbidden)
		}
	}
	publicVerification := strings.Index(workflow, "Verify recovered public assets and publisher identity")
	homebrewPublication := strings.Index(workflow, "Publish the generated Homebrew cask with a deploy key")
	if publicVerification < 0 || homebrewPublication < publicVerification {
		t.Error("release recovery updates Homebrew before it verifies the public release")
	}
}

type releaseVerificationStep struct {
	Uses string
	If   string
	With map[string]string
	Run  string
	Env  map[string]string
}

type releaseVerificationWorkflow struct {
	Jobs map[string]struct {
		If             string
		Needs          any
		TimeoutMinutes any `yaml:"timeout-minutes"`
		Strategy       struct {
			FailFast bool `yaml:"fail-fast"`
			Matrix   struct {
				Include []map[string]string
			}
		}
		Env   map[string]string
		Steps []releaseVerificationStep
	}
}

// A release runs the pull request verification jobs in parallel. One serial
// job exceeded the 180-minute job limit on v0.17.1 and v0.17.2.
func TestReleaseVerificationMirrorsPullRequestJobsBeforePublication(t *testing.T) {
	var pr, release releaseVerificationWorkflow
	if err := yaml.Unmarshal([]byte(readFixture(t, "../../.github/workflows/pr.yaml")), &pr); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal([]byte(readFixture(t, "../../.github/workflows/release.yaml")), &release); err != nil {
		t.Fatal(err)
	}
	verification := []any{"verification-checks", "verification-tests", "verification-capacity", "release-check"}
	if !reflect.DeepEqual(release.Jobs["release"].Needs, verification) {
		t.Fatalf("release needs = %v, want %v", release.Jobs["release"].Needs, verification)
	}
	for _, name := range verification {
		job := release.Jobs[name.(string)]
		if job.If != "github.event_name == 'push' || inputs.source_run_id == ''" {
			t.Errorf("%s runs in the recovery mode: %q", name, job.If)
		}
		if len(job.Steps) == 0 || !strings.HasPrefix(job.Steps[0].Uses, "actions/checkout@") || job.Steps[0].With["ref"] != "${{ inputs.tag || github.ref }}" {
			t.Errorf("%s does not check out the release tag first", name)
		}
	}
	for _, name := range []string{"verification-checks", "verification-tests", "verification-capacity"} {
		want, got := pr.Jobs[name], release.Jobs[name]
		if fmt.Sprint(got.TimeoutMinutes) != fmt.Sprint(want.TimeoutMinutes) || !reflect.DeepEqual(got.Needs, want.Needs) ||
			!reflect.DeepEqual(got.Env, want.Env) || got.Strategy.FailFast != want.Strategy.FailFast ||
			!reflect.DeepEqual(got.Strategy.Matrix.Include, want.Strategy.Matrix.Include) {
			t.Errorf("release %s differs from the pull request job", name)
		}
		// Checkout steps differ only by the release tag reference.
		for _, step := range want.Steps {
			if strings.HasPrefix(step.Uses, "actions/checkout@") {
				continue
			}
			if !slices.ContainsFunc(got.Steps, func(candidate releaseVerificationStep) bool { return reflect.DeepEqual(candidate, step) }) {
				t.Errorf("release %s is missing pull request step %+v", name, step)
			}
		}
	}
	readiness := release.Jobs["release-check"]
	if readiness.Needs != "verification-checks" ||
		!slices.ContainsFunc(readiness.Steps, func(step releaseVerificationStep) bool { return step.Run == "make release-check" }) {
		t.Error("release readiness does not follow the cheap checks")
	}
}

func TestReleaseConfigurationPinsInputsAndBuildsSupportedTargets(t *testing.T) {
	config := readFixture(t, "../../.goreleaser.yaml")
	devbox := readFixture(t, "../../devbox.json")
	for _, check := range []string{
		"ignore_tags:\n    - \"catalog-*\"",
		"goos:\n      - linux\n      - darwin\n      - windows",
		"goarch:\n      - amd64\n      - arm64",
		"env:\n      - CGO_ENABLED=0",
		"cgr.dev/chainguard/static@sha256:f51c2493951313c3ad4069080b2814ffb6ed6fe3909dabeb84a9482f42d5600b",
		`"{{ if not .Prerelease }}latest{{ end }}"`,
		`make_latest: '{{ if .Prerelease }}false{{ else }}true{{ end }}'`,
		"draft: true",
		"mode: keep-existing",
		"homebrew_casks:",
		"name: homebrew-tap",
		"url: ssh://git@github.com/agentstation/homebrew-tap.git",
		`private_key: '{{ index .Env "HOMEBREW_TAP_DEPLOY_KEY" }}'`,
		"skip_upload: auto",
		`enabled: '{{ isEnvSet "MACOS_SIGN_P12" }}'`,
		"MACOS_NOTARY_ISSUER_ID",
		"artifacts: checksum",
	} {
		if !strings.Contains(config, check) {
			t.Errorf("GoReleaser configuration is missing %q", check)
		}
	}
	if strings.Contains(config, "static:latest") {
		t.Error("container build uses a mutable base image tag")
	}
	if strings.Contains(config, "HOMEBREW_TAP_TOKEN") {
		t.Error("Homebrew publication still uses the expired cross-repository token")
	}
	if got := strings.Count(config, "CGO_ENABLED=0"); got != 2 {
		t.Errorf("GoReleaser cgo-disabled build declarations = %d, want 2", got)
	}
	var environment struct {
		Packages map[string]json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal([]byte(devbox), &environment); err != nil {
		t.Fatal(err)
	}
	if string(environment.Packages["goreleaser"]) != `"2.17.0"` {
		t.Error("developer environment does not pin the hosted GoReleaser version")
	}
}

func TestReleaseBinaryVerificationPinsPortableTargetMatrix(t *testing.T) {
	script := readFixture(t, "../../scripts/verify-release-binaries.sh")
	if strings.Contains(script, "darwin/amd64") {
		t.Fatal("release verification includes unsupported Intel macOS")
	}
	for _, check := range []string{
		"darwin/arm64",
		"linux/amd64",
		"linux/arm64",
		"windows/amd64",
		"windows/arm64",
		"CGO_ENABLED=0",
		"readelf -lW",
		"readelf -dW",
		"msvcrt|ucrtbase|vcruntime|libgcc|libstdc",
	} {
		if !strings.Contains(script, check) {
			t.Errorf("release binary verification is missing %q", check)
		}
	}
}
