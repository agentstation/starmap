#!/usr/bin/env bash
set -euo pipefail

ROOT="${STARMAP_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
SCAN_ONLY=false

if [[ "${1:-}" == "--scan-only" ]]; then
	SCAN_ONLY=true
	shift
fi
if (($# != 0)); then
	printf 'usage: %s [--scan-only]\n' "$0" >&2
	exit 2
fi
if [[ ! -d "$ROOT" ]]; then
	printf 'repository root does not exist: %s\n' "$ROOT" >&2
	exit 2
fi

if [[ "$SCAN_ONLY" == false ]]; then
	(
		cd "$ROOT"
		go test ./internal/architecture -run '^TestApprovedInternalPackageLayout$' -count=1
	)
fi

forbidden_paths=(
	"internal/bootstrap""manifest"
	"internal/embedded""budget"
	"internal/source""payload"
	"internal/test""catalog"
	"internal/test""logging"
	"internal/providers/test""helper"
)
forbidden_packages=(
	"bootstrap""manifest"
	"embedded""budget"
	"source""payload"
	"test""catalog"
	"test""logging"
	"test""helper"
)

files=()
while IFS= read -r -d '' file; do
	files+=("$file")
done < <(
	find "$ROOT" \
		\( -path "$ROOT/.git" \
		-o -path "$ROOT/docs/reviews" \
		-o -path "$ROOT/docs/proof" \
		-o -path "$ROOT/docs/plans/proof" \
		-o -path "$ROOT/htmlcov" \) -prune \
		-o -type f \
		! -path "$ROOT/docs/ARCHITECTURE_CONTROL_PLANE.md" \
		! -path "$ROOT/docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md" \
		! -path "$ROOT/coverage.html" \
		\( -name '*.go' -o -name '*.md' -o -name '*.sh' -o -name '*.yml' -o -name '*.yaml' -o -name Makefile \) \
		-print0
)

failures=0
removed_provider_packages=(
	"internal/providers/cere""bras"
	"internal/providers/deep""seek"
	"internal/providers/gr""oq"
	"internal/providers/moonshot""-ai"
)
for removed in "${removed_provider_packages[@]}"; do
	if [[ -e "$ROOT/$removed" ]]; then
		printf 'removed provider-specific test package still exists: %s\n' "$removed" >&2
		failures=$((failures + 1))
	fi
done
if ((${#files[@]} != 0)); then
	for forbidden in "${forbidden_paths[@]}"; do
		if matches="$(grep -IFnH -- "$forbidden" "${files[@]}")"; then
			printf '%s\n' "$matches" >&2
			failures=$((failures + 1))
		else
			status=$?
			if ((status > 1)); then
				printf 'package-layout scan failed while checking %q\n' "$forbidden" >&2
				exit 2
			fi
		fi
	done
	for forbidden in "${forbidden_packages[@]}"; do
		pattern="^[[:space:]]*package[[:space:]]+${forbidden}([[:space:]]|$)"
		if matches="$(grep -IEnH "$pattern" "${files[@]}")"; then
			printf '%s\n' "$matches" >&2
			failures=$((failures + 1))
		else
			status=$?
			if ((status > 1)); then
				printf 'package-layout scan failed while checking package %q\n' "$forbidden" >&2
				exit 2
			fi
		fi
	done
fi

if ((failures != 0)); then
	printf 'package-layout verification failed: %d stale reference group(s) found\n' "$failures" >&2
	exit 1
fi

printf 'package-layout verification passed\n'
