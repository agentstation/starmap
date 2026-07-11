#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR="$(mktemp -d "${TMPDIR:-/tmp}/starmap-verify.XXXXXX")"
trap 'rm -rf "$TMPDIR"' EXIT

cd "$ROOT"

run() {
	printf '\n==> %s\n' "$*"
	"$@"
}

run_optional_lint() {
	if command -v golangci-lint >/dev/null 2>&1; then
		run golangci-lint run
		return
	fi
	if command -v devbox >/dev/null 2>&1; then
		run devbox run golangci-lint run
		return
	fi
	printf '\n==> skipping golangci-lint: command not found\n'
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
	check_coverage ./internal/attribution 85
	check_coverage ./internal/attribution/matcher 75
	check_coverage ./internal/catalog/pipeline 70
	check_coverage ./internal/catalog/query 75
	check_coverage ./internal/providers/clients 80
	check_coverage ./internal/sources/providers 75
	check_coverage ./internal/server/events 70
	check_coverage ./internal/server/middleware 90
	check_coverage ./internal/server/params 95
	check_coverage ./internal/server/response 95
	check_coverage ./internal/server/sse 90
	check_coverage ./internal/server/websocket 85
	check_coverage ./internal/transport 40
	check_coverage ./pkg/authority 90
	check_coverage ./pkg/catalogs 55
	check_coverage ./pkg/errors 80
	check_coverage ./pkg/reconciler 75
	check_coverage ./pkg/sources 35
}

if [ "${STARMAP_VERIFY_COVERAGE_ONLY:-}" = "1" ]; then
	check_critical_coverage
	printf '\ncritical seam coverage passed\n'
	exit 0
fi

run go test ./...
run go test ./... -race -short
run go vet ./...
run ./scripts/verify-catalog-performance.sh
run_optional_lint

check_critical_coverage

run make docs-check
run git diff --check

run go build -o "$TMPDIR/starmap" ./cmd/starmap
run "$TMPDIR/starmap" version
run "$TMPDIR/starmap" validate catalog
printf '\n==> isolated credential-free provider listing\n'
(
	cd "$TMPDIR"
	env \
	-u ALIBABA_MODEL_STUDIO_API_KEY \
	-u ANTHROPIC_API_KEY \
	-u CEREBRAS_API_KEY \
	-u DASHSCOPE_API_KEY \
	-u DEEPINFRA_TOKEN \
	-u DEEPSEEK_API_KEY \
	-u FIREWORKS_API_KEY \
	-u GOOGLE_API_KEY \
	-u GROQ_API_KEY \
	-u MOONSHOT_API_KEY \
	-u OPENAI_API_KEY \
	"$TMPDIR/starmap" providers
)
run "$TMPDIR/starmap" models list --limit 5

printf '\nrepository verification passed\n'
