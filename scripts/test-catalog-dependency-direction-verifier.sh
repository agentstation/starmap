#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERIFIER="$ROOT/scripts/verify-catalog-dependency-direction.sh"
FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/starmap-catalog-dependency-verifier.XXXXXX")"
trap 'rm -rf "$FIXTURE"' EXIT

assert_complete_report() {
	local report="$1"
	local condition_count
	local total

	condition_count="$(grep -Ec '^SM-D[0-9]{2} (PASS|FAIL):' "$report")"
	if [[ "$condition_count" != "8" ]]; then
		printf 'verifier reported %s conditions; want 8\n' "$condition_count" >&2
		exit 1
	fi
	total="$(awk '/^Summary:/ {print $2 + $4}' "$report")"
	if [[ "$total" != "8" ]]; then
		printf 'verifier summary covers %s conditions; want 8\n' "$total" >&2
		exit 1
	fi
}

write_package() {
	local path="$1"
	local package_name="$2"
	local imported="${3:-}"

	mkdir -p "$FIXTURE/$path"
	{
		printf 'package %s\n' "$package_name"
		if [[ -n "$imported" ]]; then
			printf 'import _ "%s"\n' "github.com/agentstation/starmap/$imported"
		fi
	} >"$FIXTURE/$path/package.go"
}

printf 'module github.com/agentstation/starmap\n\ngo 1.25.0\n' >"$FIXTURE/go.mod"
write_package "internal/catalog/authority" "authority"
write_package "internal/constants" "constants"
write_package "internal/embedded" "embedded"
write_package "internal/sources/payload" "payload"
write_package "pkg/catalogs" "catalogs"
write_package "pkg/catalogs/artifact" "artifact"
write_package "pkg/catalogs/remote" "remote"
write_package "pkg/catalogs/storage" "storage"
write_package "pkg/catalogs/storage/s3" "s3"

clean_report="$FIXTURE/clean.txt"
STARMAP_CATALOG_DEPENDENCY_ROOT="$FIXTURE" bash "$VERIFIER" >"$clean_report"
assert_complete_report "$clean_report"
grep -Fq 'Summary: 8 passed, 0 failed' "$clean_report"

conditions=(
	"SM-D01|pkg/catalogs|catalogs|internal/catalog/authority"
	"SM-D02|pkg/catalogs|catalogs|internal/constants"
	"SM-D03|pkg/catalogs|catalogs|internal/embedded"
	"SM-D04|pkg/catalogs|catalogs|internal/sources/payload"
	"SM-D05|pkg/catalogs/artifact|artifact|internal/constants"
	"SM-D06|pkg/catalogs/remote|remote|internal/constants"
	"SM-D07|pkg/catalogs/storage|storage|internal/constants"
	"SM-D08|pkg/catalogs/storage/s3|s3|internal/constants"
)

for mutation in "${conditions[@]}"; do
	IFS='|' read -r id importer package_name forbidden <<<"$mutation"
	write_package "$importer" "$package_name" "$forbidden"
	report="$FIXTURE/$id.txt"
	if STARMAP_CATALOG_DEPENDENCY_ROOT="$FIXTURE" bash "$VERIFIER" >"$report" 2>&1; then
		printf '%s mutation unexpectedly passed verification\n' "$id" >&2
		exit 1
	fi
	assert_complete_report "$report"
	grep -Fq "$id FAIL:" "$report" || {
		printf '%s mutation did not fail its condition\n' "$id" >&2
		exit 1
	}
	grep -Fq 'Summary: 7 passed, 1 failed' "$report" || {
		printf '%s mutation affected more than one condition\n' "$id" >&2
		exit 1
	}
	grep -Fq "$forbidden" "$report" || {
		printf '%s failure did not name %s\n' "$id" "$forbidden" >&2
		exit 1
	}
	write_package "$importer" "$package_name"
done

if grep -Eq 'make[[:space:]]+verify|scripts/verify\.sh' "$VERIFIER"; then
	printf 'dependency verifier invokes a full repository gate\n' >&2
	exit 1
fi

printf 'catalog dependency direction verifier tests passed\n'
