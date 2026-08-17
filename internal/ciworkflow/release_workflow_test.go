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
		"workflow_dispatch:",
		"Existing release tag to recover",
		"Failed Release run that owns the exact dist artifact",
		"group: release-${{ inputs.tag || github.ref_name }}",
		"permissions:\n  contents: read",
		"    permissions:\n      attestations: write\n      contents: write\n      discussions: write\n      id-token: write\n      packages: write",
		`go-version: "1.26.6"`,
		"git merge-base --is-ancestor",
		"golangci-lint@v2.12.2",
		"make verify",
		"make release-check",
		"syft-version: v1.46.0",
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
		"!contains(github.ref_name, '-')",
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
