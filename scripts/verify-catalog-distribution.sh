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
check CAT-V01 'the workflow publishes catalog-latest and no catalog-semantic tag' \
	bash -c "grep -qsE 'catalog-latest' '$WORKFLOW' && ! grep -qsE 'catalog-semantic-' '$WORKFLOW'"
check CAT-V02 'the workflow schedule runs every four hours at minute 17' \
	file_has "cron: *['\"]?17 \*/4 \* \* \*" "$WORKFLOW"
check CAT-V03 'the workflow job stops after 60 minutes' \
	file_has 'timeout-minutes: *60' "$WORKFLOW"
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
	. TestOpenReturnsEmbeddedStateBeforeSourceReply
starmap_test CAT-V09 'the offline constructors reject runtime options' \
	. TestNewRejectsRuntimeOptions
starmap_test CAT-V10 'Catalog, State, and Status reads reach no external system' \
	. TestRuntimeReadsReachNoExternalSystem
starmap_test CAT-V11 'Refresh, RefreshSource, and Sync change distinct layers' \
	. TestRuntimeRefreshMethodsChangeDistinctLayers
starmap_test CAT-V12 'Sync returns an AcquisitionReport' \
	. TestRuntimeSyncReturnsAcquisitionReport
starmap_test CAT-V13 'Close joins runtime-owned work within five seconds' \
	. TestRuntimeCloseJoinsWithinFiveSeconds
policy_names_hold() {
	go_test_passes "$ROOT" . TestAcquisitionPolicyFromEnabledAndInterval &&
		! grep -RqsE --include='*.go' 'ACQUISITION_MODE|ACQUISITION_ON_START|REFRESH_ON_START' "$ROOT"
}
check CAT-V14 'acquisition policy is enabled plus interval with no mode or on-start name' \
	policy_names_hold
starmap_test CAT-V15 'source selection accepts public, github, starmap, file, and embedded' \
	. TestSourceSelectionNames
starmap_test CAT-V16 'a custom source never falls back to public GitHub' \
	. TestCustomSourceNeverFallsBackToPublic
starmap_test CAT-V17 'a missing acquisition credential skips the provider without a request' \
	./internal/sources/providers TestMissingCredentialSkipsProviderWithoutRequest
starmap_test CAT-V18 'the runtime retains layers and rebuilds the effective catalog under race' \
	. TestRuntimeRetainsLayersUnderRace
starmap_test CAT-V19 'refresh runs are single-flight with run identity and cancellation' \
	. TestRefreshJoinsActiveRunAndCancels

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
	. TestRefreshAddsNoDeadlineByDefault
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
	. TestStartupSpreadDefaultsToFifteenMinutes
starmap_test CAT-V31 'a stable phase survives restart' \
	. TestStablePhaseSurvivesRestart
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
	./internal/app/controllers TestAdminRefreshReturnsAcceptedOperation
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
	. TestRuntimeLeaseRejectsStaleEpochAtCommit

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
starport_console_test CAT-V50 'the shell catalog chip shows the state, the generation, and the age, and opens the panel' \
	src/components/shell/CatalogChip.test.tsx
starport_test CAT-V51 'the safe catalog metadata route reports the source, the chain, the freshness verdict, and the next attempt' \
	./internal/catalog TestCatalogMetadataReportsSourceChainAndNextAttempt
starport_console_test CAT-V52 'the catalog panel draws one node per hop and names the next update' \
	src/components/shell/CatalogPanel.test.tsx

# Documentation: declarative conditions, because prose has no runtime (CAT-D20, CAT9.1 owns them).
topology_guide_complete() {
	local guide="$STARPORT/docs/DEPLOYMENT-TOPOLOGIES.md"
	[ -f "$guide" ] &&
		file_has '^## .*[Ss]ingle Starport' "$guide" &&
		file_has '^## .*[Ff]leet' "$guide" &&
		file_has '^## .*[Cc]entral Starmap' "$guide" &&
		file_has '^## .*[Cc]entral-only' "$guide" &&
		[ "$(grep -c '^```mermaid' "$guide")" -ge 4 ] &&
		(cd "$STARPORT" && bash scripts/verify-doc-links.sh)
}
starport_check CAT-V53 'the Starport topology guide names the four topologies with a diagram each and its links resolve' \
	topology_guide_complete
starport_check CAT-V54 'the Starport README names the central Starmap server topology and links the guide' \
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
starport_check CAT-V55 'the Starport operator guide and environment example document every canonical catalog name and the example holds no removed name' \
	catalog_reference_complete
runbook_complete() {
	local runbook=docs/ENTERPRISE_CATALOG_SERVER.md
	[ -f "$runbook" ] &&
		[ "$(grep -c '^## ' "$runbook")" -ge 7 ] &&
		file_has 'starmap serve --auth' "$runbook" &&
		file_has 'STARPORT_CATALOG_SOURCE_URL' "$runbook" &&
		grep -qs "$runbook" README.md
}
check CAT-V56 'the Starmap enterprise catalog server runbook has its seven sections and the README links it' \
	runbook_complete

printf 'Summary: %d passed, %d failed, %d unverified.\n' "$pass" "$fail" "$unverified"
[ "$fail" -eq 0 ] && [ "$unverified" -eq 0 ]
