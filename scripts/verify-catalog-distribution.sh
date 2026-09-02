#!/usr/bin/env bash
# Gate for the catalog distribution campaign (ID prefix CAT-V).
#
# Each condition asserts one accepted decision or one timing row from the CAT2
# reviews and the CAT2 audit. A behavior condition runs the named Go test and
# passes only when that test reports PASS. A declarative condition reads a
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
	local tree="$1" pkg="$2" name="$3"
	(cd "$tree" && go test -count=1 -run "^${name}\$" -v "$pkg" 2>&1 |
		grep -q -- "^--- PASS: ${name}")
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

# go_lacks reports that a pattern appears in no non-test Go file under the paths.
go_lacks() {
	local pattern="$1"
	shift
	! grep -RqsE --include='*.go' --exclude='*_test.go' -- "$pattern" "$@"
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
starmap_test CAT-V21 'the per-transfer maximum duration defaults to 60 minutes' \
	./pkg/catalogs/remote TestTransferMaxDurationDefaultsToSixtyMinutes
starmap_test CAT-V22 'Refresh adds no deadline by default' \
	. TestRefreshAddsNoDeadlineByDefault
check CAT-V23 'catalog transfer clients set no client-wide total timeout' \
	go_lacks 'Timeout: *[A-Za-z0-9_.]*(Timeout|time\.)' pkg/catalogs/remote internal/transport
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
starport_test CAT-V48 'the Starport refresh lease renews within its TTL and the commit rejects a stale epoch' \
	./internal/catalog TestRefreshLeaseRejectsStaleEpoch

printf 'Summary: %d passed, %d failed, %d unverified.\n' "$pass" "$fail" "$unverified"
[ "$fail" -eq 0 ] && [ "$unverified" -eq 0 ]
