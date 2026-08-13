#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERIFIER="$ROOT/scripts/verify-catalog-package-ownership.sh"
FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/starmap-catalog-package-verifier.XXXXXX")"
trap 'rm -rf "$FIXTURE"' EXIT

assert_complete_report() {
	local report="$1"
	local condition_count
	local total

	condition_count="$(grep -Ec '^CPO-V[0-9]{2} (PASS|FAIL):' "$report")"
	if [[ "$condition_count" != "13" ]]; then
		printf 'verifier reported %s conditions; want 13\n' "$condition_count" >&2
		exit 1
	fi
	total="$(awk '/^Summary:/ {print $2 + $4}' "$report")"
	if [[ "$total" != "13" ]]; then
		printf 'verifier summary covers %s conditions; want 13\n' "$total" >&2
		exit 1
	fi
}

mkdir -p "$FIXTURE/docs/reviews"
printf '%s\n' 'Historical import: github.com/agentstation/starmap/pkg/catalog'"meta" \
	>"$FIXTURE/docs/reviews/history.md"
printf '%s\n' 'Archived import: github.com/agentstation/starmap/pkg/catalog'"store" \
	>"$FIXTURE/docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md"
printf '%s\n' \
	'pkg/catalog'"meta"' -> pkg/catalogs/evidence and pkg/catalogs/projection' \
	'pkg/catalog'"store"' -> pkg/catalogs/storage' \
	'pkg/catalog'"artifact"' -> pkg/catalogs/artifact' \
	'pkg/catalog'"remote"' -> pkg/catalogs/remote' \
	>"$FIXTURE/docs/MIGRATING_TO_V0.5.md"

historical_report="$FIXTURE/historical.txt"
if STARMAP_CATALOG_PACKAGE_ROOT="$FIXTURE" bash "$VERIFIER" >"$historical_report" 2>&1; then
	printf 'incomplete fixture unexpectedly passed verification\n' >&2
	exit 1
fi
assert_complete_report "$historical_report"
grep -Fq 'CPO-V11 PASS:' "$historical_report" || {
	printf 'verifier rejected allowlisted historical documentation\n' >&2
	exit 1
}
grep -Fq 'CPO-V12 PASS:' "$historical_report" || {
	printf 'verifier rejected an allowlisted historical path\n' >&2
	exit 1
}

mkdir -p "$FIXTURE/current"
printf '%s\n' 'import _ "github.com/agentstation/starmap/pkg/catalog'"meta"'"' \
	>"$FIXTURE/current/stale.go"
current_report="$FIXTURE/current.txt"
if STARMAP_CATALOG_PACKAGE_ROOT="$FIXTURE" bash "$VERIFIER" >"$current_report" 2>&1; then
	printf 'stale current fixture unexpectedly passed verification\n' >&2
	exit 1
fi
assert_complete_report "$current_report"
grep -Fq 'CPO-V12 FAIL:' "$current_report" || {
	printf 'verifier accepted a stale current import path\n' >&2
	exit 1
}

if grep -Eq 'make[[:space:]]+verify|scripts/verify\.sh' "$VERIFIER"; then
	printf 'structural verifier invokes a full repository gate\n' >&2
	exit 1
fi

printf 'catalog package ownership verifier regression tests passed\n'
