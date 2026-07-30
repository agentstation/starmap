package ciworkflow

import (
	"strings"
	"testing"
)

func TestReleaseWorkflowPinsToolchainPublisherAndVerification(t *testing.T) {
	workflow := readFixture(t, "../../.github/workflows/release.yaml")
	checks := []string{
		"name: Release",
		"tags:\n      - \"v*\"",
		"group: release-",
		`go-version: "1.26.5"`,
		"git merge-base --is-ancestor",
		"golangci-lint@v2.12.2",
		"make verify",
		"make release-check",
		"goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3",
		"docker/login-action@abd2ef45e78c5afb21d64d4ca52ee8550d9572c7 # v4.5.1",
		"anchore/sbom-action/download-syft@e22c389904149dbc22b58101806040fa8d37a610 # v0.24.0",
		"syft-version: v1.46.0",
		"actions/attest-build-provenance@0f67c3f4856b2e3261c31976d6725780e5e4c373 # v4.1.1",
		"version: v2.17.0",
		"Verify portable release binaries",
		"./scripts/verify-release-binaries.sh dist",
		"subject-checksums: dist/checksums.txt",
		"Verify draft release assets before publication",
		"Publish verified immutable release",
		`gh release edit "$GITHUB_REF_NAME" --draft=false`,
		`--jq .immutable`,
		"gpg --batch --verify checksums.txt.sig checksums.txt",
		"sha256sum --check checksums.txt",
		`--repo "$GITHUB_REPOSITORY"`,
		`--signer-workflow "$GITHUB_REPOSITORY/.github/workflows/release.yaml"`,
		"brew install agentstation/tap/starmap",
		`go version -m "$BINARY"`,
		`grep -Ev '^(/usr/lib/|/System/Library/)'`,
		"!contains(github.ref_name, '-')",
	}
	for _, check := range checks {
		if !strings.Contains(workflow, check) {
			t.Errorf("release workflow is missing %q", check)
		}
	}
	for _, forbidden := range []string{
		"go-version-file:",
		"check-homebrew-eligibility",
		"starmap-catalog-release",
		"STARMAP_CATALOG_OCI_MIRROR",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("release workflow contains obsolete coupling %q", forbidden)
		}
	}
}

func TestReleaseConfigurationPinsInputsAndBuildsSupportedTargets(t *testing.T) {
	config := readFixture(t, "../../.goreleaser.yaml")
	devbox := readFixture(t, "../../devbox.json")
	for _, check := range []string{
		"ignore_tags:\n    - \"catalog-payload-*\"\n    - \"catalog-semantic-*\"",
		"goos:\n      - linux\n      - darwin\n      - windows",
		"goarch:\n      - amd64\n      - arm64",
		"env:\n      - CGO_ENABLED=0",
		"cgr.dev/chainguard/static@sha256:60582b2ae6074f641094af0f370d4ab241aab271858a66223dcde7eee9f51638",
		`"{{ if not .Prerelease }}latest{{ end }}"`,
		`make_latest: '{{ if .Prerelease }}false{{ else }}true{{ end }}'`,
		"draft: true",
		"mode: keep-existing",
		"homebrew_casks:",
		"name: homebrew-tap",
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
	if got := strings.Count(config, "CGO_ENABLED=0"); got != 2 {
		t.Errorf("GoReleaser cgo-disabled build declarations = %d, want 2", got)
	}
	if !strings.Contains(devbox, `"goreleaser@2.17.0"`) {
		t.Error("developer environment does not pin the hosted GoReleaser version")
	}
}

func TestReleaseBinaryVerificationPinsPortableTargetMatrix(t *testing.T) {
	script := readFixture(t, "../../scripts/verify-release-binaries.sh")
	for _, check := range []string{
		"darwin/amd64",
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
