package catalogartifact

import (
	"os"
	"strings"
	"testing"
)

func TestScheduledGenerationWorkflowPublishesOnlyValidatedChangedPayload(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/catalog-generation.yaml")
	if err != nil {
		t.Fatalf("Read catalog generation workflow: %v", err)
	}
	workflow := string(data)
	for _, required := range []string{
		"schedule:", `cron: "17 3 * * *"`, "workflow_dispatch:", "cancel-in-progress: false",
		"./scripts/generate-embedded-catalog.sh", "jq -r .changed catalog-generation.json",
		"STARMAP_GENERATION_STATE_PATH:", "STARMAP_GENERATION_STORE_PATH:",
		"TAG=catalog-semantic-${SEMANTIC_DIGEST}", `if [[ "$CHANGED" != "true" ]]`,
		`select(.isPrerelease == true and .isDraft == false`, `write_output previous_tag "$PREVIOUS_TAG"`,
		`gh release view "$TAG"`, `--verify-dir "$EXISTING"`,
		`jq -er .semantic_checksum catalog-existing-verification.json`,
		"Validate changed candidate", "make catalog-generation-check", "make embedded-catalog-budget-check",
		"go run ./cmd/starmap-catalog-release", `--generation-store "${RUNNER_TEMP}/starmap-catalog-generation/update-home/.starmap/state/catalog"`,
		"actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8 # v4.2.2",
		"gh attestation verify", "--signer-workflow \"$GITHUB_REPOSITORY/.github/workflows/catalog-generation.yaml\"",
		"Publish changed validated catalog generation", `if: ${{ steps.change.outputs.publish == 'true' }}`,
		`gh release create "${{ steps.change.outputs.tag }}"`, "--prerelease",
		"Verify downloaded public catalog prerelease", `gh release download "$TAG" --pattern 'starmap-catalog*'`,
		`go run ./cmd/starmap-catalog-release --verify-dir "$DOWNLOAD_DIRECTORY"`,
		"Verify prior catalog prerelease remains readable", `steps.change.outputs.previous_tag != ''`,
		`go run ./cmd/starmap-catalog-release --verify-dir "$ROLLBACK_DIRECTORY"`,
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("scheduled generation workflow is missing %q", required)
		}
	}
	refresh := strings.Index(workflow, "Refresh candidate catalog")
	classify := strings.Index(workflow, "Classify catalog semantic change")
	validate := strings.Index(workflow, "Validate changed candidate")
	stage := strings.Index(workflow, "Stage validated immutable generation")
	publish := strings.Index(workflow, "Publish changed validated catalog generation")
	verify := strings.Index(workflow, "Verify downloaded public catalog prerelease")
	rollback := strings.Index(workflow, "Verify prior catalog prerelease remains readable")
	if refresh < 0 || !(refresh < classify && classify < validate && validate < stage && stage < publish && publish < verify && verify < rollback) {
		t.Fatalf("workflow order refresh/classify/validate/stage/publish/verify/rollback = %d/%d/%d/%d/%d/%d/%d", refresh, classify, validate, stage, publish, verify, rollback)
	}
	if strings.Contains(workflow, "actions/upload-artifact") {
		t.Fatal("scheduled generation uses expiring Actions artifacts as runtime publication")
	}
	if strings.Contains(workflow, "--clobber") {
		t.Fatal("scheduled generation can overwrite an existing release asset")
	}
	if strings.Contains(workflow, "jq -er .changed catalog-generation.json") {
		t.Fatal("scheduled generation treats the valid false boolean as a shell failure")
	}
}
