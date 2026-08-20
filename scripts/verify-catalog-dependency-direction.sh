#!/usr/bin/env bash
set -uo pipefail

ROOT="${STARMAP_CATALOG_DEPENDENCY_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
MODULE="github.com/agentstation/starmap"
RESULTS="$(mktemp -d "${TMPDIR:-/tmp}/starmap-catalog-dependency-direction.XXXXXX")"
trap 'rm -rf "$RESULTS"' EXIT

passed=0
failed=0

run_condition() {
	local id="$1"
	local description="$2"
	local importer="$3"
	local forbidden="$4"
	local output="$RESULTS/$id.log"

	if check_edge "$importer" "$forbidden" >"$output" 2>&1; then
		printf '%s PASS: %s\n' "$id" "$description"
		passed=$((passed + 1))
		return
	fi

	printf '%s FAIL: %s\n' "$id" "$description"
	sed 's/^/  /' "$output"
	failed=$((failed + 1))
}

check_edge() {
	local importer="$1"
	local forbidden="$2"
	local imports

	imports="$(cd "$ROOT" && go list -f '{{range .Imports}}{{println .}}{{end}}{{range .TestImports}}{{println .}}{{end}}{{range .XTestImports}}{{println .}}{{end}}' "./$importer")" || return 1
	if grep -Fxq "$MODULE/$forbidden" <<<"$imports"; then
		printf '%s imports forbidden private package %s\n' "$importer" "$forbidden"
		return 1
	fi
}

run_condition SM-D01 "catalogs does not import private authority policy" \
	"pkg/catalogs" "internal/catalog/authority"
run_condition SM-D02 "catalogs does not import repository-wide constants" \
	"pkg/catalogs" "internal/constants"
run_condition SM-D03 "catalogs does not import the private embedded filesystem" \
	"pkg/catalogs" "internal/embedded"
run_condition SM-D04 "catalogs does not import private source payload policy" \
	"pkg/catalogs" "internal/sources/payload"
run_condition SM-D05 "catalog artifacts do not import repository-wide constants" \
	"pkg/catalogs/artifact" "internal/constants"
run_condition SM-D06 "catalog remote transport does not import repository-wide constants" \
	"pkg/catalogs/remote" "internal/constants"
run_condition SM-D07 "catalog storage does not import repository-wide constants" \
	"pkg/catalogs/storage" "internal/constants"
run_condition SM-D08 "catalog S3 storage does not import repository-wide constants" \
	"pkg/catalogs/storage/s3" "internal/constants"

printf 'Summary: %d passed, %d failed\n' "$passed" "$failed"
if ((failed != 0)); then
	exit 1
fi
