#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR="$(mktemp -d "${TMPDIR:-/tmp}/starmap-pure-go.XXXXXX")"
trap 'rm -rf "$TMPDIR"' EXIT

if git -C "$ROOT" grep -n -E '^[[:space:]]*import[[:space:]]+"C"' -- '*.go'; then
	printf 'repository Go source imports C\n' >&2
	exit 1
fi

CGO_ENABLED=0 "$ROOT/scripts/verify-consumer-deps.sh"
(
	cd "$ROOT"
	CGO_ENABLED=0 go test ./pkg/catalogs/storage/s3
)

CGO_ENABLED=0 go build -trimpath -o "$TMPDIR/starmap" "$ROOT/cmd/starmap"
"$TMPDIR/starmap" version

build_info="$(go version -m "$TMPDIR/starmap")"
if ! grep -Eq $'^[[:space:]]*build[[:space:]]+CGO_ENABLED=0$' <<<"$build_info"; then
	printf 'local Starmap binary is not recorded as a cgo-disabled build:\n%s\n' \
		"$build_info" >&2
	exit 1
fi

case "$(go env GOOS)" in
linux)
	if ! command -v readelf >/dev/null 2>&1; then
		printf 'readelf is required to verify a static Linux binary\n' >&2
		exit 1
	fi
	if readelf -lW "$TMPDIR/starmap" | grep -q 'INTERP'; then
		printf 'cgo-disabled Linux binary unexpectedly has a program interpreter\n' >&2
		exit 1
	fi
	if readelf -dW "$TMPDIR/starmap" 2>&1 | grep -q '(NEEDED)'; then
		printf 'cgo-disabled Linux binary unexpectedly has a dynamic library dependency\n' >&2
		exit 1
	fi
	;;
darwin)
	unexpected="$(
		otool -L "$TMPDIR/starmap" |
			tail -n +2 |
			awk '{print $1}' |
			grep -Ev '^(/usr/lib/|/System/Library/)' || true
	)"
	if [ -n "$unexpected" ]; then
		printf 'cgo-disabled Darwin binary has a non-system dynamic dependency:\n%s\n' \
			"$unexpected" >&2
		exit 1
	fi
	;;
esac

printf 'pure-Go verification passed: library, pinned artifact, stores including S3, server, remote, and CLI execute with CGO_ENABLED=0\n'
