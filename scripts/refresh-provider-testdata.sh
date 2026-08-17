#!/usr/bin/env bash
set -euo pipefail

ROOT="${STARMAP_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

usage() {
	printf 'usage: %s <governed-provider-id>\n' "$0" >&2
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
	usage
	exit 0
fi
if (($# != 1)); then
	usage
	exit 2
fi

provider="$1"
if [[ ! "$provider" =~ ^[a-z0-9-]+$ ]]; then
	printf 'provider ID must use lowercase letters, digits, or hyphens: %s\n' "$provider" >&2
	exit 2
fi

# Each governed fixture belongs to the client that proves its wire contract, so
# a custom protocol client owns its own capture path.
case "$provider" in
anthropic)
	fixture_root="internal/providers/anthropic/testdata/providers"
	package="./internal/providers/anthropic"
	test_name='^TestRefreshAnthropicProviderFixture$'
	;;
*)
	fixture_root="internal/providers/openai/testdata/providers"
	package="./internal/providers/openai"
	test_name='^TestRefreshOpenAICompatibleProviderFixture$'
	;;
esac

fixture="$ROOT/$fixture_root/$provider/models_list.json"
metadata="$ROOT/$fixture_root/$provider/models_list.metadata.json"
if [[ ! -f "$fixture" || ! -f "$metadata" ]]; then
	printf 'no governed fixture exists for provider %s\n' "$provider" >&2
	printf 'available providers:\n' >&2
	find "$ROOT/internal/providers" -mindepth 4 -maxdepth 4 -type d -path '*/testdata/providers/*' \
		-exec basename {} \; 2>/dev/null | sort -u >&2
	exit 3
fi

before="$(cksum "$fixture" "$metadata")"
(
	cd "$ROOT"
	if [[ -n "${STARMAP_GO_TEST_BIN:-}" ]]; then
		STARMAP_PROVIDER_FIXTURE="$provider" "$STARMAP_GO_TEST_BIN" test "$package" \
			-run "$test_name" -count=1 -update
	elif command -v devbox >/dev/null 2>&1; then
		STARMAP_PROVIDER_FIXTURE="$provider" devbox run go test "$package" \
			-run "$test_name" -count=1 -update
	else
		STARMAP_PROVIDER_FIXTURE="$provider" go test "$package" \
			-run "$test_name" -count=1 -update
	fi
)
after="$(cksum "$fixture" "$metadata")"

if [[ "$before" == "$after" ]]; then
	printf 'provider refresh completed without updating payload and metadata for %s\n' "$provider" >&2
	exit 4
fi

printf 'updated %s/%s/{models_list.json,models_list.metadata.json}\n' "$fixture_root" "$provider"
