#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR="$(mktemp -d "${TMPDIR:-/tmp}/starmap-verify.XXXXXX")"
TMPDIR="$(cd "$TMPDIR" && pwd -P)"
trap 'rm -rf "$TMPDIR"' EXIT
VERIFY_HOME="$TMPDIR/home"
GOLANGCI_LINT_CACHE="$TMPDIR/golangci-lint-cache"
GOLANGCI_LINT_VERSION="2.13.2"
export GOLANGCI_LINT_CACHE

cd "$ROOT"

VERIFY_MODE="${1:-all}"
case "$VERIFY_MODE" in
	all|checks) ;;
	*) printf 'usage: %s [all|checks]\n' "$0" >&2; exit 2 ;;
esac

run() {
	printf '\n==> %s\n' "$*"
	"$@"
}

require_lint_version() {
	local output
	output="$("$@" version 2>&1)"
	if [[ "$output" != *"version $GOLANGCI_LINT_VERSION "* ]]; then
		printf 'golangci-lint %s is required; found:\n%s\n' "$GOLANGCI_LINT_VERSION" "$output" >&2
		exit 1
	fi
}

run_lint() {
	local tool="github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v$GOLANGCI_LINT_VERSION"
	require_lint_version go run "$tool"
	run go run "$tool" run
}

check_coverage() {
	local pkg="$1"
	local min="$2"
	local profile
	local output coverage
	profile="$TMPDIR/$(printf '%s' "$pkg" | tr '/.' '__').out"

	printf '\n==> coverage %s >= %s%%\n' "$pkg" "$min"
	output="$(go test -count=1 -covermode=atomic -coverprofile="$profile" "$pkg" 2>&1)"
	printf '%s\n' "$output"
	coverage="$(printf '%s\n' "$output" | awk '/coverage:/ { for (i = 1; i <= NF; i++) if ($i ~ /%$/) { gsub("%", "", $i); print $i; exit } }')"

	if [ -z "$coverage" ]; then
		printf 'coverage check failed: no coverage percentage found for %s\n' "$pkg" >&2
		exit 1
	fi

	awk -v got="$coverage" -v want="$min" 'BEGIN { if ((got + 0) < (want + 0)) exit 1 }' || {
		printf 'coverage check failed: %s has %s%% coverage, want at least %s%%\n' "$pkg" "$coverage" "$min" >&2
		exit 1
	}
}

check_critical_coverage() {
	check_coverage ./internal/catalog/pipeline 70
	check_coverage ./internal/catalog/query 75
	check_coverage ./internal/providers/clients 80
	check_coverage ./internal/sources/providers 75
	check_coverage ./internal/server/middleware 90
	check_coverage ./internal/server/openrouter 85
	check_coverage ./internal/server/params 95
	check_coverage ./internal/server/response 95
	check_coverage ./internal/server/sse 90
	check_coverage ./internal/transport 40
	check_coverage ./pkg/catalogs/authority 90
	check_coverage ./pkg/catalogs 55
	check_coverage ./pkg/errors 80
	check_coverage ./internal/catalog/reconciler 75
	check_coverage ./pkg/sources 35
}

if [ "${STARMAP_VERIFY_COVERAGE_ONLY:-}" = "1" ]; then
	check_critical_coverage
	printf '\ncritical seam coverage passed\n'
	exit 0
fi

# Fast structural and generated-output failures precede expensive execution.
run make docs-check
run git diff --check
run make test-file-sizes
run ./scripts/verify-package-layout.sh
run ./scripts/test-package-layout-verifier.sh
run ./scripts/verify-catalog-package-ownership.sh
run ./scripts/test-catalog-package-ownership-verifier.sh
run ./scripts/verify-catalog-dependency-direction.sh
run ./scripts/test-catalog-dependency-direction-verifier.sh
run bash ./scripts/verify-canonical-alias-history.sh
run python3 ./scripts/test_catalog_rejection.py
run python3 ./scripts/test_catalog_product_verify.py
run python3 ./scripts/test_prepare_public_catalog_fixture.py
run python3 ./scripts/test_verification_tests.py
run python3 ./scripts/verification_tests.py race --group checks --output "${STARMAP_VERIFY_EVENTS_DIR:-$TMPDIR}/go-check-events.jsonl"
run go vet ./...
run_lint
run go tool goago -stale-ignores ./...
run make technical-writing-check
run make test-pure-go
run ./scripts/verify-catalog-performance.sh
run ./scripts/verify-container-smoke.sh
check_critical_coverage

run go build -o "$TMPDIR/starmap" ./cmd/starmap
# Each CLI check uses only the embedded catalog and operation-owned paths.
run_cli() (
	cd "$TMPDIR"
	env -i \
		PATH="$PATH" \
		TMPDIR="$TMPDIR" \
		HOME="$VERIFY_HOME" \
		XDG_CONFIG_HOME="$VERIFY_HOME/.config" \
		CLOUDSDK_CONFIG="$VERIFY_HOME/.config/gcloud" \
		STARMAP_HOME="$TMPDIR/product" \
		STARMAP_CATALOG_SOURCE=embedded \
		STARMAP_CATALOG_ACQUISITION_ENABLED=false \
		STARMAP_CATALOG_WORKSPACE_PATH= \
		"$TMPDIR/starmap" "$@"
)

mkdir -p "$VERIFY_HOME"
run run_cli version
run run_cli validate catalog
run run_cli providers
run run_cli models list --limit 5

if [ "$VERIFY_MODE" = "all" ]; then
	# Race execution covers ordinary behavior too. The explicit capacity test
	# retains the full public corpus without race instrumentation.
	for verification_shard in 1 2 3; do
		run python3 ./scripts/verification_tests.py race --group runtime --shard "$verification_shard"
	done
	for verification_group in client application contracts; do
		run python3 ./scripts/verification_tests.py race --group "$verification_group"
	done
	run python3 ./scripts/verification_tests.py capacity
fi

printf '\nrepository verification passed (%s)\n' "$VERIFY_MODE"
