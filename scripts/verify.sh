#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR="$(mktemp -d "${TMPDIR:-/tmp}/starmap-verify.XXXXXX")"
trap 'rm -rf "$TMPDIR"' EXIT
VERIFY_CATALOG_PATH="$ROOT/internal/embedded/catalog"
VERIFY_CATALOG_DATABASE_PATH="$TMPDIR/catalog"
VERIFY_HOME="$TMPDIR/home"
GOLANGCI_LINT_CACHE="$TMPDIR/golangci-lint-cache"
GOLANGCI_LINT_VERSION="2.12.2"
export GOLANGCI_LINT_CACHE

cd "$ROOT"

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
	if command -v devbox >/dev/null 2>&1; then
		require_lint_version devbox run golangci-lint
		run devbox run golangci-lint run
		return
	fi
	if command -v golangci-lint >/dev/null 2>&1; then
		require_lint_version golangci-lint
		run golangci-lint run
		return
	fi
	printf 'golangci-lint %s is required; install it with:\n' "$GOLANGCI_LINT_VERSION" >&2
	printf '  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v%s\n' "$GOLANGCI_LINT_VERSION" >&2
	exit 1
}

check_coverage() {
	local pkg="$1"
	local min="$2"
	local profile
	local output coverage
	profile="$TMPDIR/$(printf '%s' "$pkg" | tr '/.' '__').out"

	printf '\n==> coverage %s >= %s%%\n' "$pkg" "$min"
	output="$(go test -covermode=atomic -coverprofile="$profile" "$pkg" 2>&1)"
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
	check_coverage ./internal/catalog/authority 90
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

run go test ./...
run make test-pure-go
run make test-file-sizes
run ./scripts/verify-package-layout.sh
run ./scripts/test-package-layout-verifier.sh
run ./scripts/verify-catalog-package-ownership.sh
run ./scripts/test-catalog-package-ownership-verifier.sh
run ./scripts/verify-catalog-dependency-direction.sh
run ./scripts/test-catalog-dependency-direction-verifier.sh
run env CGO_ENABLED=1 go test ./... -race -short -timeout=20m
run go vet ./...
run ./scripts/verify-catalog-performance.sh
run_lint

check_critical_coverage

run make docs-check
run git diff --check

run go build -o "$TMPDIR/starmap" ./cmd/starmap
run "$TMPDIR/starmap" version
run env CATALOG_PATH="$VERIFY_CATALOG_DATABASE_PATH" CATALOG_EXPORT_PATH="$VERIFY_CATALOG_PATH" \
	"$TMPDIR/starmap" validate catalog
printf '\n==> isolated credential-free provider listing\n'
mkdir -p "$VERIFY_HOME"
(
	cd "$TMPDIR"
	env -i \
		PATH="$PATH" \
		CATALOG_PATH="$VERIFY_CATALOG_DATABASE_PATH" \
		CATALOG_EXPORT_PATH="$VERIFY_CATALOG_PATH" \
	CLOUDSDK_CONFIG="$VERIFY_HOME/.config/gcloud" \
	HOME="$VERIFY_HOME" \
	XDG_CONFIG_HOME="$VERIFY_HOME/.config" \
	"$TMPDIR/starmap" providers
)
run env CATALOG_PATH="$VERIFY_CATALOG_DATABASE_PATH" CATALOG_EXPORT_PATH="$VERIFY_CATALOG_PATH" \
	"$TMPDIR/starmap" models list --limit 5

printf '\nrepository verification passed\n'
