package artifact

import (
	"os"
	"strings"
	"testing"
)

// workflowSection returns the workflow text between two step names.
func workflowSection(t *testing.T, workflow, start, end string) string {
	t.Helper()
	from := strings.Index(workflow, start)
	to := strings.Index(workflow, end)
	if from < 0 || to <= from {
		t.Fatalf("workflow has no section between %q and %q", start, end)
	}
	return workflow[from:to]
}

func TestScheduledGenerationWorkflowPublishesOnlyValidatedChangedPayload(t *testing.T) {
	data, err := os.ReadFile("../../../.github/workflows/catalog-generation.yaml")
	if err != nil {
		t.Fatalf("Read catalog generation workflow: %v", err)
	}
	workflow := string(data)
	for _, required := range []string{
		"schedule:", `cron: "17 */4 * * *"`, "workflow_dispatch:", "cancel-in-progress: false",
		"timeout-minutes: 90", "timeout-minutes: 75",
		"./scripts/generate-embedded-catalog.sh", "jq -r .changed catalog-generation.json",
		"STARMAP_GENERATION_STATE_PATH:", "STARMAP_GENERATION_STORE_PATH:",
		"TAG=catalog-${CATALOG_DIGEST}", `PREVIOUS_TAG=""`,
		`--rollback-candidates "${RUNNER_TEMP}/catalog-release-listing.json"`,
		`--exclude-tag "$TAG"`,
		`--inspect-dir "$CANDIDATE_DIRECTORY"`, `.supports_current_schema == true`,
		`--signer-workflow "$GITHUB_REPOSITORY/.github/workflows/catalog-generation.yaml"`,
		`write_output previous_tag "$PREVIOUS_TAG"`,
		`gh release view "$TAG"`, `"$EXISTING/starmap-catalog.tar.gz"`, `--verify-dir "$EXISTING"`,
		`jq -er .semantic_checksum catalog-existing-verification.json`,
		"Validate candidate catalog", "make catalog-generation-check", "make embedded-catalog-budget-check",
		"go run ./cmd/starmap-catalog-release", `--generation-store "${RUNNER_TEMP}/starmap-catalog-generation/update-home/.starmap/state/catalog"`,
		"actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8 # v4.2.2",
		"gh attestation verify", "--deny-self-hosted-runners",
		"Publish changed validated catalog generation", `if: ${{ steps.change.outputs.publish == 'true' }}`,
		`gh release create "${{ steps.change.outputs.tag }}"`, "--prerelease",
		"Verify downloaded public catalog prerelease", `gh release download "$TAG" --pattern 'starmap-catalog*'`,
		`go run ./cmd/starmap-catalog-release --verify-dir "$DOWNLOAD_DIRECTORY"`,
		"Verify prior catalog prerelease remains readable", `steps.change.outputs.previous_tag != ''`,
		`go run ./cmd/starmap-catalog-release --verify-dir "$ROLLBACK_DIRECTORY"`,
		"Read current catalog channel document", "Stage attested catalog channel document",
		"Attest catalog channel document", "Verify catalog channel provenance",
		"Publish catalog channel document", "Verify published catalog channel document",
		"catalog-latest", "catalog-latest.json", `--title "Catalog latest"`,
		"--channel-release-dir", "--channel-tag", "--channel-published-at",
		"--channel-current", "--channel-out", "--channel-attestation-verified",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("scheduled generation workflow is missing %q", required)
		}
	}
	refresh := strings.Index(workflow, "Refresh candidate catalog")
	classify := strings.Index(workflow, "Classify candidate catalog generation")
	validate := strings.Index(workflow, "Validate candidate catalog")
	stage := strings.Index(workflow, "Stage validated immutable generation")
	publish := strings.Index(workflow, "Publish changed validated catalog generation")
	verify := strings.Index(workflow, "Verify downloaded public catalog prerelease")
	rollback := strings.Index(workflow, "Verify prior catalog prerelease remains readable")
	if refresh < 0 || !(refresh < classify && classify < validate && validate < stage && stage < publish && publish < verify && verify < rollback) {
		t.Fatalf("workflow order refresh/classify/validate/stage/publish/verify/rollback = %d/%d/%d/%d/%d/%d/%d", refresh, classify, validate, stage, publish, verify, rollback)
	}

	// The channel promotes only after the immutable release verifies.
	channelStage := strings.Index(workflow, "Stage attested catalog channel document")
	channelAttest := strings.Index(workflow, "Attest catalog channel document")
	channelVerify := strings.Index(workflow, "Verify catalog channel provenance")
	channelPublish := strings.Index(workflow, "Publish catalog channel document")
	channelReadback := strings.Index(workflow, "Verify published catalog channel document")
	if !(verify < channelStage && channelStage < channelAttest && channelAttest < channelVerify &&
		channelVerify < channelPublish && channelPublish < channelReadback) {
		t.Fatalf(
			"channel order verify/stage/attest/verify/publish/readback = %d/%d/%d/%d/%d/%d",
			verify, channelStage, channelAttest, channelVerify, channelPublish, channelReadback,
		)
	}

	existing := strings.Index(workflow, `if gh release view "$TAG"`)
	selectPrevious := strings.Index(workflow, `PREVIOUS_TAG=""`)
	inspectPrevious := strings.Index(workflow, `--inspect-dir "$CANDIDATE_DIRECTORY"`)
	acceptPrevious := strings.Index(workflow, `PREVIOUS_TAG="$CANDIDATE_TAG"`)
	if existing < 0 || !(existing < selectPrevious && selectPrevious < inspectPrevious && inspectPrevious < acceptPrevious) {
		t.Fatalf(
			"workflow order existing/select/inspect/accept = %d/%d/%d/%d",
			existing,
			selectPrevious,
			inspectPrevious,
			acceptPrevious,
		)
	}
	if strings.Contains(workflow, "actions/upload-artifact") {
		t.Fatal("scheduled generation uses expiring Actions artifacts as runtime publication")
	}

	// The immutable release never overwrites an asset. Only the mutable channel
	// replaces its document in place.
	immutable := workflowSection(t, workflow,
		"Publish changed validated catalog generation", "Read current catalog channel document")
	if strings.Contains(immutable, "--clobber") {
		t.Fatal("scheduled generation can overwrite an existing immutable release asset")
	}
	if strings.Count(workflow, "--clobber") != 1 {
		t.Fatal("only the catalog channel document replaces a published asset")
	}
	channel := workflowSection(t, workflow,
		"Publish catalog channel document", "Verify published catalog channel document")
	if !strings.Contains(channel, `gh release upload catalog-latest "$DOCUMENT" --clobber`) {
		t.Fatal("catalog channel does not replace its published document")
	}

	// The retired namespaces belong to the Go release command, not the workflow.
	if strings.Contains(workflow, "catalog-semantic-") {
		t.Fatal("scheduled generation still publishes the retired semantic namespace")
	}
	if strings.Contains(workflow, "jq -er .changed catalog-generation.json") {
		t.Fatal("scheduled generation treats the valid false boolean as a shell failure")
	}
	if strings.Contains(workflow, `last | .tagName`) {
		t.Fatal("scheduled generation selects a rollback candidate without compatibility inspection")
	}
}
