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

# CPO-V13 must report a dependency change and ignore a version change. A gate
# that also reported version changes would fail every routine dependency
# update, which is what the earlier whole-file checksum did.
MODULE_FIXTURE="$FIXTURE/module"
mkdir -p "$MODULE_FIXTURE"
printf '%s\n' 'example.com/kept' >"$MODULE_FIXTURE/approved.txt"

write_module_fixture() {
	local version="$1"
	local extra="${2:-}"

	{
		printf 'module example.com/fixture\n\ngo 1.25.0\n\nrequire (\n'
		printf '\texample.com/kept %s\n' "$version"
		printf '\texample.com/carried v0.1.0 // indirect\n'
		if [[ -n "$extra" ]]; then
			printf '\t%s v1.0.0\n' "$extra"
		fi
		printf ')\n'
	} >"$MODULE_FIXTURE/go.mod"
}

run_module_fixture() {
	local report="$1"

	STARMAP_CATALOG_PACKAGE_ROOT="$MODULE_FIXTURE" \
		STARMAP_CATALOG_DIRECT_MODULES="$MODULE_FIXTURE/approved.txt" \
		bash "$VERIFIER" >"$report" 2>&1 || true
}

for version in v1.2.3 v1.9.9; do
	write_module_fixture "$version"
	version_report="$MODULE_FIXTURE/version-$version.txt"
	run_module_fixture "$version_report"
	assert_complete_report "$version_report"
	grep -Fq 'CPO-V13 PASS:' "$version_report" || {
		printf 'verifier rejected a version-only module change at %s\n' "$version" >&2
		exit 1
	}
done

write_module_fixture v1.2.3 example.com/unapproved
dependency_report="$MODULE_FIXTURE/dependency.txt"
run_module_fixture "$dependency_report"
assert_complete_report "$dependency_report"
grep -Fq 'CPO-V13 FAIL:' "$dependency_report" || {
	printf 'verifier accepted an unapproved direct dependency\n' >&2
	exit 1
}
grep -Fq 'example.com/unapproved' "$dependency_report" || {
	printf 'verifier did not name the unapproved dependency\n' >&2
	exit 1
}

{
	printf 'module example.com/fixture\n\ngo 1.25.0\n\nrequire (\n'
	printf '\texample.com/kept v1.2.3\n'
	printf '\texample.com/carried v0.1.0 // indirect\n'
	printf '\texample.com/hidden v1.0.0 // indirect\n'
	printf ')\n'
} >"$MODULE_FIXTURE/go.mod"
indirect_report="$MODULE_FIXTURE/indirect.txt"
run_module_fixture "$indirect_report"
grep -Fq 'CPO-V13 PASS:' "$indirect_report" || {
	printf 'verifier reported an indirect requirement as a dependency change\n' >&2
	exit 1
}

printf 'catalog package ownership verifier regression tests passed\n'
