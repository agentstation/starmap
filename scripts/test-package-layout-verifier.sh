#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERIFIER="$ROOT/scripts/verify-package-layout.sh"
FIXTURE="$(mktemp -d "${TMPDIR:-/tmp}/starmap-package-layout.XXXXXX")"
trap 'rm -rf "$FIXTURE"' EXIT

bash "$VERIFIER" >/dev/null

mkdir -p "$FIXTURE/current" "$FIXTURE/docs/reviews"
old_path="internal/bootstrap""manifest"
printf 'import _ %q\n' "example.com/project/$old_path" >"$FIXTURE/current/stale.go"
if STARMAP_ROOT="$FIXTURE" bash "$VERIFIER" --scan-only >/dev/null 2>&1; then
	printf 'package-layout verifier accepted a stale current import path\n' >&2
	exit 1
fi

mv "$FIXTURE/current/stale.go" "$FIXTURE/docs/reviews/historical.go"
if ! STARMAP_ROOT="$FIXTURE" bash "$VERIFIER" --scan-only >/dev/null; then
	printf 'package-layout verifier rejected archived review evidence\n' >&2
	exit 1
fi

old_package="embedded""budget"
printf 'package %s\n' "$old_package" >"$FIXTURE/current/stale.go"
if STARMAP_ROOT="$FIXTURE" bash "$VERIFIER" --scan-only >/dev/null 2>&1; then
	printf 'package-layout verifier accepted a stale package declaration\n' >&2
	exit 1
fi

printf 'package-layout verifier regression tests passed\n'
