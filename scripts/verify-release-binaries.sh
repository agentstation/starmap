#!/usr/bin/env bash
set -euo pipefail

DIST="${1:-dist}"
EXPECTED_TARGETS="$(
	cat <<'EOF'
darwin/amd64
darwin/arm64
linux/amd64
linux/arm64
windows/amd64
windows/arm64
EOF
)"
ACTUAL_TARGETS="$(mktemp "${TMPDIR:-/tmp}/starmap-release-targets.XXXXXX")"
trap 'rm -f "$ACTUAL_TARGETS"' EXIT

verified=0
while IFS= read -r binary; do
	build_info="$(go version -m "$binary" 2>/dev/null || true)"
	if [[ "$build_info" != *$'\tpath\tgithub.com/agentstation/starmap/cmd/starmap'* ]]; then
		continue
	fi

	if ! grep -Eq $'^[[:space:]]*build[[:space:]]+CGO_ENABLED=0$' <<<"$build_info"; then
		printf 'release binary is not recorded as a cgo-disabled build: %s\n' \
			"$binary" >&2
		exit 1
	fi

	goos="$(awk '$1 == "build" && $2 ~ /^GOOS=/ { sub(/^GOOS=/, "", $2); print $2 }' <<<"$build_info")"
	goarch="$(awk '$1 == "build" && $2 ~ /^GOARCH=/ { sub(/^GOARCH=/, "", $2); print $2 }' <<<"$build_info")"
	printf '%s/%s\n' "$goos" "$goarch" >>"$ACTUAL_TARGETS"

	case "$goos" in
	darwin)
		if command -v otool >/dev/null 2>&1; then
			unexpected="$(
				otool -L "$binary" |
					tail -n +2 |
					awk '{print $1}' |
					grep -Ev '^(/usr/lib/|/System/Library/)' || true
			)"
			if [ -n "$unexpected" ]; then
				printf 'Darwin release binary has a non-system dynamic dependency: %s\n%s\n' \
					"$binary" "$unexpected" >&2
				exit 1
			fi
		fi
		;;
	linux)
		if command -v readelf >/dev/null 2>&1; then
			if readelf -lW "$binary" | grep -q 'INTERP'; then
				printf 'Linux release binary has a program interpreter: %s\n' "$binary" >&2
				exit 1
			fi
			if readelf -dW "$binary" 2>&1 | grep -q '(NEEDED)'; then
				printf 'Linux release binary has a dynamic library dependency: %s\n' "$binary" >&2
				exit 1
			fi
		elif ! file "$binary" | grep -q 'statically linked'; then
			printf 'Linux release binary is not reported as statically linked: %s\n' \
				"$binary" >&2
			exit 1
		fi
		;;
	windows)
		if objdump -p "$binary" |
			grep -Ei 'DLL Name:.*(msvcrt|ucrtbase|vcruntime|libgcc|libstdc\+\+)'; then
			printf 'Windows release binary imports a C/C++ runtime: %s\n' "$binary" >&2
			exit 1
		fi
		;;
	esac

	verified=$((verified + 1))
done < <(find "$DIST" -type f \( -name starmap -o -name starmap.exe \) | LC_ALL=C sort)

if [ "$verified" -ne 6 ]; then
	printf 'verified %s release binaries; want exactly 6\n' "$verified" >&2
	exit 1
fi

if ! diff -u <(printf '%s\n' "$EXPECTED_TARGETS") <(LC_ALL=C sort -u "$ACTUAL_TARGETS"); then
	printf 'release target matrix does not match the supported targets\n' >&2
	exit 1
fi

printf 'verified 6 cgo-disabled release binaries; Linux is static and Windows imports no C/C++ runtime\n'
