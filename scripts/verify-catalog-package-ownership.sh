#!/usr/bin/env bash
set -uo pipefail

ROOT="${STARMAP_CATALOG_PACKAGE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
MODULE="github.com/agentstation/starmap"
BASELINE_GO_MOD_SHA256="563a7e3779efe55001ab43d0a42c53154a14b470bbeecb59464972c48c1d493c"
RESULTS="$(mktemp -d "${TMPDIR:-/tmp}/starmap-catalog-package-ownership.XXXXXX")"
trap 'rm -rf "$RESULTS"' EXIT

passed=0
failed=0

run_condition() {
	local id="$1"
	local description="$2"
	local check="$3"
	local output="$RESULTS/$id.log"

	if "$check" >"$output" 2>&1; then
		printf '%s PASS: %s\n' "$id" "$description"
		passed=$((passed + 1))
		return
	fi

	printf '%s FAIL: %s\n' "$id" "$description"
	sed 's/^/  /' "$output"
	failed=$((failed + 1))
}

require_package() {
	local path="$1"
	local package_name="$2"

	test -d "$ROOT/$path" || {
		printf 'missing package directory: %s\n' "$path"
		return 1
	}
	grep -RqsE "^[[:space:]]*package[[:space:]]+$package_name([[:space:]]|$)" \
		--include='*.go' "$ROOT/$path" || {
		printf 'missing package declaration %s in %s\n' "$package_name" "$path"
		return 1
	}
}

package_uses_only_standard_library() {
	local package_path="$1"
	local imports
	local imported
	local standard

	imports="$(cd "$ROOT" && go list -f '{{join .Imports "\n"}}' "$package_path")" || return 1
	while IFS= read -r imported; do
		[[ -z "$imported" ]] && continue
		standard="$(cd "$ROOT" && go list -f '{{.Standard}}' "$imported")" || return 1
		if [[ "$standard" != "true" ]]; then
			printf '%s imports non-standard package %s\n' "$package_path" "$imported"
			return 1
		fi
	done <<<"$imports"
}

catalogs_root_has() {
	local pattern="$1"
	local file

	while IFS= read -r -d '' file; do
		if grep -qsE "$pattern" "$file"; then
			return 0
		fi
	done < <(find "$ROOT/pkg/catalogs" -maxdepth 1 -type f -name '*.go' -print0)
	return 1
}

sha256_file() {
	local file="$1"

	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$file" | awk '{print $1}'
		return
	fi
	shasum -a 256 "$file" | awk '{print $1}'
}

old_paths() {
	printf '%s\n' \
		"pkg/catalog""meta" \
		"pkg/catalog""store" \
		"pkg/catalog""artifact" \
		"pkg/catalog""remote"
}

check_v01() {
	require_package "pkg/catalogs/evidence" "evidence" &&
		require_package "pkg/catalogs/projection" "projection" &&
		require_package "pkg/catalogs/storage" "storage" &&
		require_package "pkg/catalogs/artifact" "artifact" &&
		require_package "pkg/catalogs/remote" "remote"
}

check_v02() {
	local path
	while IFS= read -r path; do
		if [[ -e "$ROOT/$path" ]]; then
			printf 'removed package still exists: %s\n' "$path"
			return 1
		fi
	done < <(old_paths)
}

check_v03() {
	catalogs_root_has '^type[[:space:]]+Generation[[:space:]]+struct' &&
		catalogs_root_has '^func[[:space:]]+\([^)]*Generation\)[[:space:]]+Copy\(' &&
		catalogs_root_has '^func[[:space:]]+\([^)]*Generation\)[[:space:]]+Validate\('
}

check_v04() {
	local function_name
	for function_name in EncodeCatalogPayload DecodeCatalogPayload DecodeSourceObservationPayload; do
		catalogs_root_has "^func[[:space:]]+$function_name\\(" || {
			printf 'catalogs does not own %s\n' "$function_name"
			return 1
		}
	done
}

check_v05() {
	require_package "pkg/catalogs/evidence" "evidence" &&
		package_uses_only_standard_library "./pkg/catalogs/evidence" &&
		! grep -RqsE '^type[[:space:]]+Projection' --include='*.go' "$ROOT/pkg/catalogs/evidence"
}

check_v06() {
	require_package "pkg/catalogs/projection" "projection" &&
		package_uses_only_standard_library "./pkg/catalogs/projection" &&
		! grep -RqsE '^type[[:space:]]+[[:alnum:]_]+[[:space:]]*=' --include='*.go' "$ROOT/pkg/catalogs/projection"
}

check_v07() {
	require_package "pkg/catalogs/storage" "storage" &&
		require_package "pkg/catalogs/storage/s3" "s3" &&
		test -f "$ROOT/pkg/catalogs/storage/memory.go" &&
		test -f "$ROOT/pkg/catalogs/storage/filesystem.go" &&
		test -f "$ROOT/pkg/catalogs/storage/object.go" &&
		grep -RqsF 'catalogs.Generation' --include='*.go' "$ROOT/pkg/catalogs/storage" &&
		grep -RqsE '^func[[:space:]]+TestCatalogStoreConformance\(' --include='*_test.go' "$ROOT/pkg/catalogs/storage"
}

check_v08() {
	local imports
	require_package "pkg/catalogs/artifact" "artifact" || return 1
	imports="$(cd "$ROOT" && go list -f '{{join .Imports "\n"}}' ./pkg/catalogs/artifact)" || return 1
	if grep -Fxq "$MODULE/pkg/catalogs/storage" <<<"$imports"; then
		printf 'artifact imports storage\n'
		return 1
	fi
	grep -RqsE '^func[[:space:]]+TestBundleReproducibleFixtureHashes\(' \
		--include='*_test.go' "$ROOT/pkg/catalogs/artifact"
}

check_v09() {
	local imports
	require_package "pkg/catalogs/remote" "remote" || return 1
	imports="$(cd "$ROOT" && go list -f '{{join .Imports "\n"}}' ./pkg/catalogs/remote)" || return 1
	if grep -Fxq "$MODULE/pkg/catalogs/storage" <<<"$imports"; then
		printf 'remote imports storage\n'
		return 1
	fi
	grep -RqsE '^func[[:space:]]+TestRemoteCatalogFetchValidatesManifestPayloadChecksumAndCompatibility\(' \
		--include='*_test.go' "$ROOT/pkg/catalogs/remote" &&
		grep -RqsE '^func[[:space:]]+TestEventStreamParsesCommentsAndStablePublication\(' \
		--include='*_test.go' "$ROOT/pkg/catalogs/remote"
}

check_v10() {
	local module
	for module in read-only store-only pinned-artifact server-embed remote-subscriber server-storage; do
		test -f "$ROOT/testdata/consumers/$module/go.mod" || {
			printf 'missing external consumer module: %s\n' "$module"
			return 1
		}
		(
			cd "$ROOT/testdata/consumers/$module"
			GOWORK=off go test ./...
		) || return 1
	done
}

check_v11() {
	local migration="$ROOT/docs/MIGRATING_TO_V0.5.md"
	local path
	local files=()

	while IFS= read -r -d '' path; do
		files+=("$path")
	done < <(
		find "$ROOT/docs" "$ROOT/pkg" \
			\( -path "$ROOT/docs/reviews" -o -path "$ROOT/docs/proof" -o -path "$ROOT/docs/plans" \) -prune \
			-o -type f -name '*.md' ! -path "$migration" -print0
	)
	while IFS= read -r path; do
		if ((${#files[@]} != 0)) && grep -IFnH -- "$path" "${files[@]}"; then
			printf 'current documentation names removed path: %s\n' "$path"
			return 1
		fi
	done < <(old_paths)

	test -f "$migration" || {
		printf 'missing migration guide: docs/MIGRATING_TO_V0.5.md\n'
		return 1
	}
	grep -F 'pkg/catalog'"meta" "$migration" | grep -Fq 'pkg/catalogs/evidence' &&
		grep -F 'pkg/catalog'"meta" "$migration" | grep -Fq 'pkg/catalogs/projection' &&
		grep -F 'pkg/catalog'"store" "$migration" | grep -Fq 'pkg/catalogs/storage' &&
		grep -F 'pkg/catalog'"artifact" "$migration" | grep -Fq 'pkg/catalogs/artifact' &&
		grep -F 'pkg/catalog'"remote" "$migration" | grep -Fq 'pkg/catalogs/remote'
}

check_v12() {
	local path
	local files=()

	while IFS= read -r -d '' path; do
		files+=("$path")
	done < <(
		find "$ROOT" \
			\( -path "$ROOT/.git" -o -path "$ROOT/docs" \) -prune \
			-o -type f \( -name '*.go' -o -name '*.sh' -o -name '*.yml' -o -name '*.yaml' -o -name 'Makefile' -o -name 'go.mod' \) \
			-print0
	)
	while IFS= read -r path; do
		if ((${#files[@]} != 0)) && grep -IFnH -- "$path" "${files[@]}"; then
			printf 'current source names removed path: %s\n' "$path"
			return 1
		fi
	done < <(old_paths)
}

check_v13() {
	local checksum
	test -f "$ROOT/go.mod" || {
		printf 'missing go.mod\n'
		return 1
	}
	checksum="$(sha256_file "$ROOT/go.mod")" || return 1
	if [[ "$checksum" != "$BASELINE_GO_MOD_SHA256" ]]; then
		printf 'go.mod checksum changed: got %s, want %s\n' "$checksum" "$BASELINE_GO_MOD_SHA256"
		return 1
	fi
	(cd "$ROOT" && go list ./... >/dev/null)
}

run_condition CPO-V01 "the five approved catalog child packages exist" check_v01
run_condition CPO-V02 "the four removed package trees are absent" check_v02
run_condition CPO-V03 "catalogs owns Generation copy and validation" check_v03
run_condition CPO-V04 "catalogs owns all canonical payload codecs" check_v04
run_condition CPO-V05 "evidence is a standard-library-only leaf" check_v05
run_condition CPO-V06 "projection is a standard-library-only leaf without aliases" check_v06
run_condition CPO-V07 "storage accepts catalog generations and retains adapters" check_v07
run_condition CPO-V08 "artifact is storage-independent and retains its byte test" check_v08
run_condition CPO-V09 "remote is storage-independent and retains protocol tests" check_v09
run_condition CPO-V10 "all six external consumer modules compile" check_v10
run_condition CPO-V11 "current docs and the migration guide use approved paths" check_v11
run_condition CPO-V12 "current authority passes the historical allowlist scan" check_v12
run_condition CPO-V13 "the package graph resolves without a module change" check_v13

printf 'Summary: %d passed, %d failed\n' "$passed" "$failed"
((failed == 0))
