#!/usr/bin/env bash
# Verify that every SHA-pinned GitHub Action resolves to the release tag its
# comment claims.
#
# This is additive supply-chain evidence. The reviewed-pin allowlist in
# internal/ciworkflow proves that a human acknowledged each pin, but it runs
# offline and therefore cannot see whether a pinned commit belongs to the
# version the comment advertises. This check closes that gap, so a pin that
# points away from its stated release fails here.
#
# It never replaces the allowlist: Dependabot always bumps to a real tag, so
# this check alone would approve every bump.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKFLOWS="$ROOT/.github/workflows"

for tool in curl jq; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		printf 'verify-action-pins requires %s\n' "$tool" >&2
		exit 1
	fi
done

# Bash 3.2 ships on macOS and aborts on an empty array expansion under set -u,
# so the authenticated and anonymous calls stay separate.
api() {
	local path="$1"
	if [ -n "${GITHUB_TOKEN:-}" ]; then
		curl -fsS --retry 3 --retry-delay 2 --max-time 30 \
			-H "Accept: application/vnd.github+json" \
			-H "X-GitHub-Api-Version: 2022-11-28" \
			-H "Authorization: Bearer ${GITHUB_TOKEN}" \
			"https://api.github.com${path}"
		return
	fi
	curl -fsS --retry 3 --retry-delay 2 --max-time 30 \
		-H "Accept: application/vnd.github+json" \
		-H "X-GitHub-Api-Version: 2022-11-28" \
		"https://api.github.com${path}"
}

PINS="$(mktemp "${TMPDIR:-/tmp}/starmap-action-pins.XXXXXX")"
trap 'rm -f "$PINS"' EXIT

# Every active `uses:` reference must carry both a 40-character commit and the
# version comment that names what the commit is supposed to be.
unverifiable="$(
	grep -hE '^[[:space:]]*uses:' "$WORKFLOWS"/*.yaml |
		grep -vE 'uses:[[:space:]]+[^@[:space:]]+@[0-9a-f]{40}[[:space:]]*#[[:space:]]*\S+' || true
)"
if [ -n "$unverifiable" ]; then
	printf 'workflow actions without a commit pin and version comment:\n%s\n' \
		"$unverifiable" >&2
	exit 1
fi

grep -hoE 'uses:[[:space:]]+[^@[:space:]]+@[0-9a-f]{40}[[:space:]]*#[[:space:]]*\S+' "$WORKFLOWS"/*.yaml |
	sed -E 's/^uses:[[:space:]]+//; s/@/ /; s/[[:space:]]*#[[:space:]]*/ /' |
	LC_ALL=C sort -u >"$PINS"

if [ ! -s "$PINS" ]; then
	printf 'no SHA-pinned actions found under %s\n' "$WORKFLOWS" >&2
	exit 1
fi

failures=0
checked=0
while read -r action pinned version; do
	[ -z "$action" ] && continue
	# Subdirectory actions such as anchore/sbom-action/download-syft are tagged
	# on their owning repository.
	repo="$(printf '%s\n' "$action" | cut -d/ -f1,2)"

	if ! ref="$(api "/repos/${repo}/git/ref/tags/${version}" 2>/dev/null)"; then
		printf '%s: cannot resolve tag %s in %s\n' "$action" "$version" "$repo" >&2
		failures=$((failures + 1))
		continue
	fi

	object_type="$(jq -r '.object.type // empty' <<<"$ref")"
	resolved="$(jq -r '.object.sha // empty' <<<"$ref")"
	# Annotated tags point at a tag object that must be dereferenced to a commit.
	if [ "$object_type" = "tag" ]; then
		if ! annotated="$(api "/repos/${repo}/git/tags/${resolved}" 2>/dev/null)"; then
			printf '%s: cannot dereference annotated tag %s\n' "$action" "$version" >&2
			failures=$((failures + 1))
			continue
		fi
		resolved="$(jq -r '.object.sha // empty' <<<"$annotated")"
	fi

	if [ -z "$resolved" ]; then
		printf '%s: tag %s resolved to no commit\n' "$action" "$version" >&2
		failures=$((failures + 1))
		continue
	fi

	if [ "$resolved" != "$pinned" ]; then
		printf '%s pinned to %s but %s is %s\n' \
			"$action" "$pinned" "$version" "$resolved" >&2
		failures=$((failures + 1))
		continue
	fi

	checked=$((checked + 1))
	printf '%s %s matches %s\n' "$action" "$version" "$pinned"
done <"$PINS"

if [ "$failures" -gt 0 ]; then
	printf '%s action pin(s) do not match their advertised release\n' "$failures" >&2
	exit 1
fi

printf 'action pins: %s reference(s) resolve to their advertised release tag\n' \
	"$checked"
