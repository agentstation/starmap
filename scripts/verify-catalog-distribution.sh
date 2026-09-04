#!/usr/bin/env bash
# Gate for the catalog distribution campaign (ID prefix CAT-V).
#
# Each condition asserts one accepted decision or one timing row from the CAT2
# reviews and the CAT2 audit. A behavior condition runs the named Go test and
# passes only when the test command exits zero and reports PASS. A declarative condition reads a
# workflow, module, or configuration fact that no test can prove. The header
# of each condition names its kind.
#
# CAT2 authored the gate red. Each condition turns green when its owning
# task closes. A condition that already holds proves behavior that the campaign
# must preserve. The gate prints every condition and exits nonzero while any
# condition fails or stays unverified.
#
# Starport conditions run in the tree that CATALOG_DISTRIBUTION_STARPORT_ROOT
# names. The default is ../starport. A missing tree reports UNVERIFIED, and an
# unverified condition fails the gate.
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 1

STARPORT="${CATALOG_DISTRIBUTION_STARPORT_ROOT:-$ROOT/../starport}"
WORKFLOW=.github/workflows/catalog-generation.yaml
PROOF=docs/proof/catalog-publisher
SELECTION="$PROOF/cat2.1-verifier-selection.md"

pass=0
fail=0
unverified=0

record() {
	local outcome="$1" id="$2" desc="$3"
	printf '%s %s %s.\n' "$outcome" "$id" "$desc"
	case "$outcome" in
	PASS) pass=$((pass + 1)) ;;
	FAIL) fail=$((fail + 1)) ;;
	UNVERIFIED) unverified=$((unverified + 1)) ;;
	esac
}

# check runs a command and records PASS or FAIL.
check() {
	local id="$1" desc="$2"
	shift 2
	if "$@" >/dev/null 2>&1; then
		record PASS "$id" "$desc"
	else
		record FAIL "$id" "$desc"
	fi
}

# go_test_passes runs one named test in one package tree of the tree at $1.
# It passes only when the test reports PASS.
go_test_passes() {
	local tree="$1" pkg="$2" name="$3" output
	output="$(cd "$tree" && go test -count=1 -run "^${name}\$" -v "$pkg" 2>&1)" || return 1
	printf '%s\n' "$output" | grep -q -- "^--- PASS: ${name}"
}

# starmap_test records the outcome of one named Starmap test.
starmap_test() {
	local id="$1" desc="$2" pkg="$3" name="$4"
	check "$id" "$desc" go_test_passes "$ROOT" "$pkg" "$name"
}

# starport_test records the outcome of one named Starport test.
starport_test() {
	local id="$1" desc="$2" pkg="$3" name="$4"
	if [ ! -d "$STARPORT/internal" ]; then
		record UNVERIFIED "$id" "$desc (no Starport tree at $STARPORT)"
		return
	fi
	check "$id" "$desc" go_test_passes "$STARPORT" "$pkg" "$name"
}

# starport_check records a declarative Starport condition.
starport_check() {
	local id="$1" desc="$2"
	shift 2
	if [ ! -d "$STARPORT/internal" ]; then
		record UNVERIFIED "$id" "$desc (no Starport tree at $STARPORT)"
		return
	fi
	check "$id" "$desc" "$@"
}

# go_has reports that a pattern appears in a non-test Go file under the paths.
go_has() {
	local pattern="$1"
	shift
	grep -RqsE --include='*.go' --exclude='*_test.go' -- "$pattern" "$@"
}

file_has() {
	local pattern="$1" file="$2"
	grep -qsE -- "$pattern" "$file"
}

# selected_engine_required reads the CAT2.1 record and requires that go.mod
# names the module the record selected.
selected_engine_required() {
	local engine
	# shellcheck disable=SC2016
	engine="$(sed -nE 's/^Selected engine: `([^`]+)`.*/\1/p' "$SELECTION" | head -1)"
	[ -n "$engine" ] && grep -qsF -- "$engine " go.mod
}

# Publication: declarative workflow facts (CAT-D1, CAT-D9, CAT-D10, CAT-D12).
check CAT-V01 'the workflow publishes the catalog/v1 channel branch and no catalog-semantic tag' \
	bash -c "grep -qsE 'CHANNEL_BRANCH: catalog/v1' '$WORKFLOW' && grep -qsF 'git push origin' '$WORKFLOW' && ! grep -qsE 'catalog-semantic-' '$WORKFLOW'"
check CAT-V02 'the workflow schedule runs every four hours at minute 17' \
	file_has "cron: *['\"]?17 \*/4 \* \* \*" "$WORKFLOW"
# The limits nest: 60-minute transfer, 75-minute publisher step, 90-minute job.
# The test parses the workflow with the module's YAML dependency. It reads
# jobs.generate.timeout-minutes and the Refresh candidate catalog step limit
# at their positions. Reversed or unrelated values cannot pass.
starmap_test CAT-V03 'the workflow parses with the generate job at 90 minutes and the Refresh candidate catalog step at 75 minutes' \
	./internal/ciworkflow TestCatalogGenerationWorkflowNestsTimeoutLimits
check CAT-V04 'the workflow serializes runs and cancels no run in progress' \
	file_has 'cancel-in-progress: *false' "$WORKFLOW"
starmap_test CAT-V05 'the channel document advances channel_updated_at without a new catalog generation' \
	./pkg/catalogs/artifact/... TestChannelAdvancesUpdatedAtWithoutNewGeneration
check CAT-V06 'go.mod names the attestation engine that the CAT2.1 record selected' \
	selected_engine_required
starmap_test CAT-V07 'the GitHub source reads legacy catalog-semantic and catalog-payload releases' \
	./internal/sources/... TestGitHubSourceReadsLegacyReleaseTags

# Runtime contract: behavior tests (CAT-D2, CAT-D3, CAT-D8, CAT-D14).
starmap_test CAT-V08 'Open returns a connected runtime with embedded state before any network reply' \
	./runtime TestOpenReturnsEmbeddedStateBeforeSourceReply
starmap_test CAT-V09 'the offline constructors reject runtime options' \
	. TestNewRejectsRuntimeOptions
starmap_test CAT-V10 'Catalog, State, and Status reads reach no external system' \
	./runtime TestRuntimeReadsReachNoExternalSystem
starmap_test CAT-V11 'Refresh, RefreshSource, and Sync change distinct layers' \
	./runtime TestRuntimeRefreshMethodsChangeDistinctLayers
starmap_test CAT-V12 'Sync returns an AcquisitionReport' \
	./runtime TestRuntimeSyncReturnsAcquisitionReport
starmap_test CAT-V13 'Close joins runtime-owned work within five seconds' \
	./runtime TestRuntimeCloseJoinsWithinFiveSeconds
policy_names_hold() {
	go_test_passes "$ROOT" ./runtime TestAcquisitionPolicyFromEnabledAndInterval &&
		! grep -RqsE --include='*.go' 'ACQUISITION_MODE|ACQUISITION_ON_START|REFRESH_ON_START' "$ROOT"
}
check CAT-V14 'acquisition policy is enabled plus interval with no mode or on-start name' \
	policy_names_hold
starmap_test CAT-V15 'source selection accepts public, github, starmap, file, and embedded' \
	./runtime TestSourceSelectionNames
starmap_test CAT-V16 'a custom source never falls back to public GitHub' \
	./runtime TestCustomSourceNeverFallsBackToPublic
starmap_test CAT-V17 'a missing acquisition credential skips the provider without a request' \
	./internal/sources/providers TestMissingCredentialSkipsProviderWithoutRequest
starmap_test CAT-V18 'the runtime retains layers and rebuilds the effective catalog under race' \
	./runtime TestRuntimeRetainsLayersUnderRace
starmap_test CAT-V19 'refresh runs are single-flight with run identity and cancellation' \
	./runtime TestRefreshJoinsActiveRunAndCancels

# Transport policy: behavior tests plus declarative defaults (CAT-D12, CAT-D15).
starmap_test CAT-V20 'a stalled body stops at the two-minute inactivity timeout' \
	./pkg/catalogs/remote TestTransferIdleTimeoutStopsStalledBody
transfer_max_duration_contract() {
	go_test_passes "$ROOT" ./pkg/catalogs/remote TestTransferMaxDurationDefaultsToSixtyMinutes &&
		go_test_passes "$ROOT" ./pkg/catalogs/remote TestTransferMaxDurationRejectsZero
}
check CAT-V21 'the per-transfer maximum duration defaults to 60 minutes and rejects zero' \
	transfer_max_duration_contract
starmap_test CAT-V22 'Refresh adds no deadline by default' \
	./runtime TestRefreshAddsNoDeadlineByDefault
no_client_wide_timeout() {
	go_test_passes "$ROOT" ./pkg/catalogs/remote TestNewClientSetsNoClientWideTimeout &&
		go_test_passes "$ROOT" ./internal/transport TestNewSetsNoClientWideTimeout
}
check CAT-V23 'catalog transfer clients set no client-wide total timeout' \
	no_client_wide_timeout
starmap_test CAT-V24 'the catalog transport applies the response-header bound to every request' \
	./pkg/catalogs/remote TestCatalogTransportAppliesResponseHeaderTimeout
starmap_test CAT-V25 'the SSE stream open honors the response-header bound' \
	./remote TestSubscriberOpenHonorsResponseHeaderTimeout
starmap_test CAT-V26 'a slow-drip transfer fails at the per-transfer bound' \
	./pkg/catalogs/remote TestSlowDripTransferFailsAtMaxDuration
starmap_test CAT-V27 'the catalog payload handler resets a write deadline before each chunk' \
	./internal/server/handlers TestCatalogPayloadResetsWriteDeadlinePerChunk
starmap_test CAT-V28 'the SSE frame write deadline defaults to two minutes' \
	./internal/server/... TestSSEFrameWriteDeadlineDefaultsToTwoMinutes
starmap_test CAT-V29 'the admin update returns an accepted asynchronous operation' \
	./internal/server/handlers TestAdminUpdateReturnsAcceptedOperation

# Fleet policy: behavior tests (CAT-D13, CAT-D16).
starmap_test CAT-V30 'the startup spread defaults to 15 minutes' \
	./runtime TestStartupSpreadDefaultsToFifteenMinutes
starmap_test CAT-V31 'a stable phase survives restart' \
	./runtime TestStablePhaseSurvivesRestart
starmap_test CAT-V32 'the subscriber reconnect delay uses decorrelated jitter up to 15 minutes' \
	./remote TestSubscriberReconnectDelayCapsAtFifteenMinutes
starmap_test CAT-V33 'the subscriber resets backoff only after a healthy liveness window' \
	./remote TestSubscriberResetsBackoffAfterHealthyWindow
starmap_test CAT-V34 'the subscriber honors Retry-After as a not-before' \
	./remote TestSubscriberHonorsRetryAfterNotBefore
starmap_test CAT-V35 'an authentication failure waits for credential change instead of stopping' \
	./remote TestSubscriberWaitsForCredentialChange
starmap_test CAT-V36 'fallback polling uses a stable jittered phase' \
	./remote TestFallbackPollingUsesStablePhase
starmap_test CAT-V37 'the GitHub source reports the rate-limit budget from limit, used, remaining, and reset headers and warns at 80 percent' \
	./internal/sources/... TestGitHubSourceReportsRateLimitBudget
starmap_test CAT-V38 'source server admission returns Retry-After on refusal' \
	./internal/server/... TestSourceAdmissionReturnsRetryAfter

# Evidence: declarative proof files.
check CAT-V39 'the proof root records publisher run measurements' \
	test -s "$PROOF/cat2-publisher-runs.json"
check CAT-V40 'the proof root records network measurements' \
	test -s "$PROOF/cat2-network-measurements.json"
check CAT-V41 'the proof root records the independent audit' \
	test -s "$PROOF/cat2-audit.md"

# Starport adoption: behavior tests plus declarative facts (CAT-D7, CAT-D13, CAT-D18).
removed_variables_rejected() {
	go_test_passes "$STARPORT" ./internal/config TestRemovedCatalogVariableFailsStartup &&
		! grep -qsE 'REFRESH_ON_START|REFRESH_INTERVAL|REMOTE_URL|REMOTE_API_KEY|REMOTE_ACTIVATION_INTERVAL' "$STARPORT/internal/config/config.go"
}
starport_check CAT-V42 'Starport rejects each removed catalog variable with a named startup error' \
	removed_variables_rejected
starport_test CAT-V43 'Starport streaming carries no elapsed deadline after the first byte' \
	./internal/execution TestStreamingCarriesNoElapsedDeadlineAfterFirstByte
starport_test CAT-V44 'the Starport admin refresh returns an accepted operation' \
	./internal/server/controllers TestAdminRefreshReturnsAcceptedOperation
starport_test CAT-V45 'the Starport acquisition resolver reads only the deployment lookup and never a BYOK credential' \
	./internal/catalog TestAcquisitionResolverReadsOnlyDeploymentLookup
starport_test CAT-V46 'Starport accepts only a matching forward state through the expected-ID compare-and-swap' \
	./internal/catalog TestRemoteRuntimeAcceptsOnlyMatchingForwardState
starport_check CAT-V47 'Starport freshness keeps the age_seconds and degradation_reasons fields' \
	bash -c "grep -qsE 'json:\"age_seconds\"' '$STARPORT/internal/catalog/freshness.go' && grep -qsE 'json:\"degradation_reasons' '$STARPORT/internal/catalog/freshness.go'"
starport_test CAT-V48 'the Starport candidate-to-accepted transaction rejects a stale lease epoch' \
	./internal/catalog TestAcceptRejectsStaleLeaseEpoch

# Runtime lease: the Starmap runtime fences its durable generation commit (CAT-D18, owned by CAT5).
starmap_test CAT-V49 'the runtime lease fences the durable generation commit with its epoch' \
	./runtime TestRuntimeLeaseRejectsStaleEpochAtCommit

# Console: the shell-owned catalog chip and panel (CAT-D19, CAT8.1 owns them).
# A console condition runs one vitest file in the Starport console tree. It
# reports UNVERIFIED without that tree or without pnpm.
starport_console_test() {
	local id="$1" desc="$2" file="$3"
	if [ ! -d "$STARPORT/console/src" ] || ! command -v pnpm >/dev/null 2>&1; then
		record UNVERIFIED "$id" "$desc (no Starport console tree or pnpm)"
		return
	fi
	check "$id" "$desc" bash -c "cd '$STARPORT/console' && [ -f '$file' ] && pnpm vitest run --reporter=dot '$file'"
}
starport_console_test CAT-V50 'the catalog chip renders one element per status concept and a models:read render holds no admin pill or activity icon' \
	src/components/shell/CatalogChip.test.tsx
# The safe route projects an allowlisted summary. The first test fills every
# operational field with a sentinel value and proves that none reaches the
# response. The second proves the sanitized 503. The third proves the admin scope.
safe_catalog_boundary() {
	go_test_passes "$STARPORT" ./internal/server TestSafeCatalogRouteProjectsAllowlistedSummaryOnly &&
		go_test_passes "$STARPORT" ./internal/server TestSafeCatalogRouteAnswersMissingCatalogWithSanitized503 &&
		go_test_passes "$STARPORT" ./internal/server TestAdminCatalogStatusRequiresAdminScope
}
starport_check CAT-V51 'the safe catalog route serializes only the allowlisted summary with no sentinel operational value, answers a missing catalog with a sanitized 503, and the admin status route requires the admin scope' \
	safe_catalog_boundary
starport_console_test CAT-V52 'the catalog panel always draws the embedded baseline, labels direct and upstream-reported hops, and names the next update' \
	src/components/shell/CatalogPanel.test.tsx
starport_console_test CAT-V53 'the shell renders the chip on Overview, Models, and Chat, uses the 44 px small-screen control, and bounds the panel to the viewport' \
	src/components/shell/Shell.catalog.test.tsx
starport_console_test CAT-V54 'Enter and Space open the catalog panel, Escape closes it, and focus returns to the chip' \
	src/components/shell/CatalogPanel.keyboard.test.tsx
starport_console_test CAT-V55 'after a 403 the shell shows the no-authorization sentence and sends no second catalog request' \
	src/components/shell/CatalogChip.unauthorized.test.tsx

# Starport documentation: declarative conditions, because prose has no runtime (CAT-D20, CAT9.1 owns them).
topology_guide_complete() {
	local guide="$STARPORT/docs/DEPLOYMENT-TOPOLOGIES.md"
	[ -f "$guide" ] &&
		file_has '^## .*[Ss]ingle Starport' "$guide" &&
		file_has '^## .*[Ff]leet' "$guide" &&
		file_has '^## .*[Cc]entral Starmap' "$guide" &&
		file_has '^## .*[Rr]estricted' "$guide" &&
		file_has '^## .*[Aa]ir-gapped' "$guide" &&
		file_has '^## .*[Rr]eplicated' "$guide" &&
		[ "$(grep -c '^```mermaid' "$guide")" -ge 6 ] &&
		(cd "$STARPORT" && bash scripts/verify-doc-links.sh)
}
starport_check CAT-V56 'the Starport topology guide names the five topologies and the replicated variant with a diagram each and its links resolve' \
	topology_guide_complete
starport_check CAT-V57 'the Starport README names the central Starmap server topology and links the guide' \
	bash -c "grep -qsi 'central Starmap' '$STARPORT/README.md' && grep -qs 'docs/DEPLOYMENT-TOPOLOGIES.md' '$STARPORT/README.md'"
catalog_reference_complete() {
	local guide="$STARPORT/docs/OPERATOR-GUIDE.md" example="$STARPORT/.env.example" suffix
	[ -f "$guide" ] && [ -f "$example" ] || return 1
	while read -r suffix; do
		[ -n "$suffix" ] || continue
		grep -qsF -- "STARPORT_CATALOG_$suffix" "$guide" || return 1
		grep -qsF -- "STARPORT_CATALOG_$suffix" "$example" || return 1
	done < <(
		# shellcheck disable=SC2016
		sed -nE 's/^\| `CATALOG_([A-Z_]+)` \|.*/\1/p' "$PROOF/cat2-final-review.md"
	)
	! grep -qsE 'STARPORT_CATALOG_(REMOTE_URL|REMOTE_API_KEY|REMOTE_ACTIVATION_INTERVAL|REFRESH_ON_START|REFRESH_INTERVAL)' "$example"
}
starport_check CAT-V58 'the Starport operator guide and environment example document every canonical catalog name and the example holds no removed name' \
	catalog_reference_complete

# Starmap documentation: the central server runbook and the Kubernetes pair (CAT-D20, CAT9.2 owns it).
runbook_complete() {
	local runbook=docs/ENTERPRISE_CATALOG_SERVER.md
	[ -f "$runbook" ] &&
		[ "$(grep -c '^## ' "$runbook")" -ge 8 ] &&
		file_has 'starmap serve --auth' "$runbook" &&
		file_has 'STARPORT_CATALOG_SOURCE_URL' "$runbook" &&
		file_has 'DOCKER.md' "$runbook" &&
		grep -qs "$runbook" README.md
}
check CAT-V59 'the Starmap runbook has its eight sections and the README links it' \
	runbook_complete

# Runtime status and provider retention: accepted behavior of the runtime (CAT-D14, CAT-D17, owned by CAT5).
starmap_test CAT-V60 'the runtime status reports usability, freshness, fallback, direct source health, and upstream-reported health as independent values' \
	./runtime TestRuntimeStatusKeepsUsabilityFreshnessFallbackAndHealthIndependent
starmap_test CAT-V61 'a partial provider failure publishes the succeeded layers and the failed provider retains its own last-known-good observation' \
	./acquisition/... TestSyncPartialFailurePublishesAndRetainsProviderLastKnownGood

# Starport model and provider surfaces: route-validation state and per-model facts (CAT-D17, CAT8 owns them).
starport_test CAT-V62 'the admin catalog status reports candidate, accepted, rejected, and pending route-validation state as distinct values' \
	./internal/server TestAdminCatalogStatusReportsRouteValidationState
starport_console_test CAT-V63 'the model detail shows lifecycle, credential-specific availability, provenance, and routing state as separate elements' \
	src/components/models/ModelDetail.lifecycle.test.tsx

# Kubernetes example: a repository-owned structural parse of the manifests in docs/DOCKER.md (CAT-D20, CAT9.2 owns it).
# The test uses the module's YAML dependency. It checks the Deployment names, the Service selector
# and target port, the Starmap container port, and the Starport source URL. No ambient interpreter takes part.
starmap_test CAT-V64 'the Docker document Kubernetes pair parses as two named Deployments and one Service whose selector and port match Starmap, and the Starport source URL names that Service' \
	./internal/deploymentdocs TestDockerDocumentKubernetesPairWiresStarportToStarmap

# Fifth-review conditions: propagated freshness, bounded coalescing, clone-safe identity, and the console lifecycle.
starmap_test CAT-V65 'freshness measures the propagated channel_updated_at through two Starmap hops with local acquisition on each hop' \
	./internal/server TestCascadedFreshnessPropagatesChannelUpdatedAtThroughHops
starmap_test CAT-V66 'completed provider observations publish through one bounded coalescing window while another provider stays blocked' \
	./acquisition/... TestSyncPublishesCompletedProvidersWhileAnotherBlocked
starmap_test CAT-V67 'cloned state and a shared store give replicas distinct scheduler phases and a restart keeps its phase' \
	./runtime TestSchedulerIdentityDivergesAcrossClonedState
starport_console_test CAT-V68 'the shell owns one summary query with a visible-only cadence, waits for Retry-After after a 503, stops after a 401 until the session changes, and polls admin status only while the panel is open' \
	src/components/shell/CatalogSummary.lifecycle.test.tsx

printf 'Summary: %d passed, %d failed, %d unverified.\n' "$pass" "$fail" "$unverified"
[ "$fail" -eq 0 ] && [ "$unverified" -eq 0 ]
