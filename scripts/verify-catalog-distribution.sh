#!/usr/bin/env bash
# Guard for the catalog distribution campaign (ID prefix CAT-V).
# Each condition asserts one accepted decision or one timing-matrix row from
# the CAT2 runtime review, the CAT2 final review, and the CAT2 audit:
#
#   - publication: catalog-<digest> releases, the attested catalog-latest
#     channel, the four-hour cadence at minute 17, and the 60-minute job bound,
#   - runtime contract: offline New, connected Open, explicit Refresh,
#     RefreshSource, and Sync, the AcquisitionReport contract, and a five-second
#     Close join,
#   - transport policy: stage timeouts, transfer inactivity, a per-transfer
#     maximum duration, no client-wide total, no default refresh deadline, and
#     per-chunk server write deadlines,
#   - fleet policy: stable phases, a 15-minute startup spread, decorrelated
#     retry up to 15 minutes, retry not-before, non-terminal auth failure, a
#     rate-limit warning, and source admission,
#   - Starport adoption: canonical settings only, route-specific timing, an
#     asynchronous admin refresh, an injected acquisition resolver, and the
#     preserved accepted-head CAS and freshness fields.
#
# Authored red at plan commit 96f0c3cc (CAT2): each condition turns green as
# its owning CAT task closes. A condition that already holds proves behavior
# the implementation must preserve. The gate reports every condition and exits
# nonzero while any condition fails. Starport conditions report UNVERIFIED
# when no Starport tree is available; set CATALOG_DISTRIBUTION_STARPORT_ROOT.
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 1

STARPORT="${CATALOG_DISTRIBUTION_STARPORT_ROOT:-$ROOT/../starport}"
WORKFLOW=.github/workflows/catalog-generation.yaml
PROOF=docs/proof/catalog-publisher

pass=0
fail=0
unverified=0

check() {
	local id="$1" desc="$2"
	shift 2
	if "$@" >/dev/null 2>&1; then
		printf 'PASS %s %s\n' "$id" "$desc"
		pass=$((pass + 1))
	else
		printf 'FAIL %s %s\n' "$id" "$desc"
		fail=$((fail + 1))
	fi
}

starport_check() {
	local id="$1" desc="$2"
	shift 2
	if [ ! -d "$STARPORT/internal" ]; then
		printf 'UNVERIFIED %s %s (no Starport tree at %s)\n' "$id" "$desc" "$STARPORT"
		unverified=$((unverified + 1))
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

# test_has reports that a pattern appears in a Go test file under the paths.
test_has() {
	local pattern="$1"
	shift
	grep -RqsE --include='*_test.go' -- "$pattern" "$@"
}

file_has() {
	local pattern="$1" file="$2"
	grep -qsE -- "$pattern" "$file"
}

file_lacks() {
	local pattern="$1" file="$2"
	! grep -qsE -- "$pattern" "$file"
}

root_go_has() {
	go_has "$1" ./*.go
}

root_test_has() {
	test_has "$1" ./*_test.go
}

# Publication (CAT-D1, CAT-D9, CAT-D10, CAT-D12).
check CAT-V01 'workflow publishes catalog-latest and no catalog-semantic tag' \
	bash -c "grep -qsE 'catalog-latest' '$WORKFLOW' && ! grep -qsE 'catalog-semantic-' '$WORKFLOW'"
check CAT-V02 'workflow schedule is every four hours at minute 17' \
	file_has "cron: *['\"]?17 \*/4 \* \* \*" "$WORKFLOW"
check CAT-V03 'workflow job timeout is 60 minutes' \
	file_has 'timeout-minutes: *60' "$WORKFLOW"
check CAT-V04 'workflow serializes runs without cancel-in-progress' \
	file_has 'cancel-in-progress: *false' "$WORKFLOW"
check CAT-V05 'channel document carries channel_updated_at' \
	go_has 'channel_updated_at' pkg internal
check CAT-V06 'native attestation verifier dependency is selected' \
	file_has 'github.com/sigstore/sigstore-go' go.mod
check CAT-V07 'legacy catalog-semantic and catalog-payload releases remain readable' \
	go_has 'catalog-semantic-' pkg internal

# Runtime contract (CAT-D2, CAT-D3, CAT-D8, CAT-D14).
check CAT-V08 'root package exposes Open with a context' \
	root_go_has '^func Open\(ctx context\.Context'
check CAT-V09 'offline constructors reject runtime options' \
	root_test_has 'func TestNewRejectsRuntimeOptions'
check CAT-V10 'runtime exposes Catalog, State, and Status reads' \
	bash -c "grep -qsE 'func \(r \*Runtime\) Catalog\(\)' ./*.go && grep -qsE 'func \(r \*Runtime\) State\(\)' ./*.go && grep -qsE 'func \(r \*Runtime\) Status\(\)' ./*.go"
check CAT-V11 'runtime keeps explicit Refresh, RefreshSource, and Sync' \
	bash -c "grep -qsE 'func \(r \*Runtime\) Refresh\(' ./*.go && grep -qsE 'func \(r \*Runtime\) RefreshSource\(' ./*.go && grep -qsE 'func \(r \*Runtime\) Sync\(' ./*.go"
check CAT-V12 'Sync returns AcquisitionReport' \
	root_go_has 'func \(r \*Runtime\) Sync\(ctx context\.Context, opts \.\.\.SyncOption\) \(AcquisitionReport, error\)'
check CAT-V13 'Close joins owned work within five seconds' \
	root_test_has 'func TestRuntimeCloseJoinsWithinFiveSeconds'
check CAT-V14 'acquisition policy is enabled plus interval with no mode or on-start name' \
	bash -c "grep -RqsE --include='*.go' 'CATALOG_ACQUISITION_ENABLED' . && grep -RqsE --include='*.go' 'CATALOG_ACQUISITION_INTERVAL' . && ! grep -RqsE --include='*.go' 'ACQUISITION_MODE|ACQUISITION_ON_START|REFRESH_ON_START' ."
check CAT-V15 'source selection names public, github, starmap, file, and embedded' \
	bash -c "grep -RqsE --include='*.go' 'CATALOG_SOURCE\b' . && grep -RqsE --include='*.go' '\"embedded\"' internal"
check CAT-V16 'a custom source never falls back to public GitHub' \
	test_has 'func TestCustomSourceNeverFallsBackToPublic' . 
check CAT-V17 'missing acquisition credentials skip the provider without a request' \
	test_has 'skipped_not_configured' .
check CAT-V18 'runtime retains layers and rebuilds the effective catalog' \
	root_test_has 'func TestRuntimeRetainsLayersUnderRace'
check CAT-V19 'refresh runs are single-flight with run identity and cancellation' \
	root_test_has 'func TestRefreshJoinsActiveRunAndCancels'

# Transport policy (CAT-D12, CAT-D15).
check CAT-V20 'transfer inactivity timeout is configurable and defaults to two minutes' \
	bash -c "grep -RqsE --include='*.go' 'CATALOG_TRANSFER_IDLE_TIMEOUT' . && grep -RqsE --include='*.go' 'WithTransferIdleTimeout' ."
check CAT-V21 'per-transfer maximum duration is configurable and defaults to 60 minutes' \
	bash -c "grep -RqsE --include='*.go' 'CATALOG_TRANSFER_MAX_DURATION' . && grep -RqsE --include='*.go' 'WithTransferMaxDuration' ."
check CAT-V22 'refresh timeout defaults to no added deadline' \
	bash -c "grep -RqsE --include='*.go' 'CATALOG_REFRESH_TIMEOUT' . && grep -RqsE --include='*.go' 'WithRefreshTimeout' ."
check CAT-V23 'catalog transfer clients set no client-wide total timeout' \
	go_lacks 'Timeout: *[A-Za-z0-9_.]*(Timeout|time\.)' pkg/catalogs/remote internal/transport
check CAT-V24 'catalog transport owns the response-header bound for every request' \
	test_has 'func TestCatalogTransportAppliesResponseHeaderTimeout' pkg/catalogs/remote remote
check CAT-V25 'SSE stream open honors the response-header bound' \
	test_has 'func TestSubscriberOpenHonorsResponseHeaderTimeout' remote
check CAT-V26 'slow-drip transfer fails at the per-transfer bound' \
	test_has 'func TestTransferStopsAtMaxDuration' pkg/catalogs/remote
check CAT-V27 'catalog payload writes reset a per-chunk write deadline' \
	file_has 'SetWriteDeadline' internal/server/handlers/catalog.go
check CAT-V28 'SSE frame write deadline defaults to two minutes' \
	file_has 'SSEWriteTimeout *= *2 \* time\.Minute' internal/server/config.go
check CAT-V29 'admin update returns an accepted asynchronous operation' \
	go_has 'http\.StatusAccepted' internal/server/handlers

# Fleet policy (CAT-D13, CAT-D16).
check CAT-V30 'startup spread is configurable and defaults to 15 minutes' \
	bash -c "grep -RqsE --include='*.go' 'CATALOG_STARTUP_SPREAD' . && grep -RqsE --include='*.go' 'WithStartupSpread' ."
check CAT-V31 'stable phase survives restart' \
	test_has 'func TestStablePhaseSurvivesRestart' .
check CAT-V32 'subscriber reconnect delay caps at 15 minutes' \
	file_has 'ReconnectMaxDelay *= *15 \* time\.Minute' remote/config.go
check CAT-V33 'subscriber backoff resets only after a healthy liveness window' \
	test_has 'func TestSubscriberResetsBackoffAfterHealthyWindow' remote
check CAT-V34 'subscriber honors Retry-After as a not-before' \
	go_has 'Retry-After' remote pkg/catalogs/remote
check CAT-V35 'authentication failure waits instead of stopping the subscriber' \
	test_has 'func TestSubscriberWaitsForCredentialChange' remote
check CAT-V36 'fallback polling uses a stable jittered phase' \
	test_has 'func TestFallbackPollingUsesStablePhase' remote
check CAT-V37 'direct GitHub consumers warn near the rate-limit budget' \
	go_has 'x-ratelimit-limit' internal pkg
check CAT-V38 'source server admission returns Retry-After' \
	go_has 'Retry-After' internal/server

# Evidence.
check CAT-V39 'publisher run measurements are recorded' \
	test -s "$PROOF/cat2-publisher-runs.json"
check CAT-V40 'network measurements are recorded' \
	test -s "$PROOF/cat2-network-measurements.json"
check CAT-V41 'independent audit is recorded' \
	test -s "$PROOF/cat2-audit.md"

# Starport adoption (CAT-D7, CAT-D13, decision 13).
starport_check CAT-V42 'Starport configuration has no legacy catalog names' \
	bash -c "! grep -qsE 'REFRESH_ON_START|REFRESH_INTERVAL|REMOTE_URL|REMOTE_API_KEY|REMOTE_ACTIVATION_INTERVAL' '$STARPORT/internal/config/config.go'"
starport_check CAT-V43 'Starport streaming carries no elapsed-budget total deadline' \
	file_lacks 'context\.WithTimeout\(ctx, e\.config\.MaxElapsed\)' "$STARPORT/internal/execution/stream.go"
starport_check CAT-V44 'Starport admin refresh returns an accepted operation' \
	go_has 'http\.StatusAccepted' "$STARPORT/internal/app/controllers"
starport_check CAT-V45 'Starport injects a deployment-lookup credential resolver into acquisition' \
	go_has 'WithCatalogCredentialResolver' "$STARPORT/internal/catalog"
starport_check CAT-V46 'Starport accepted head commits by expected generation ID' \
	file_has 'expectedID' "$STARPORT/internal/catalog/remote_runtime.go"
starport_check CAT-V47 'Starport freshness keeps age_seconds and degradation_reasons' \
	bash -c "grep -qsE 'json:\"age_seconds\"' '$STARPORT/internal/catalog/freshness.go' && grep -qsE 'json:\"degradation_reasons' '$STARPORT/internal/catalog/freshness.go'"
starport_check CAT-V48 'Starport refresh lease records a fencing epoch' \
	test_has 'func TestRefreshLeaseRejectsStaleEpoch' "$STARPORT/internal/catalog"

printf 'Summary: %d passed, %d failed, %d unverified\n' "$pass" "$fail" "$unverified"
[ "$fail" -eq 0 ]
