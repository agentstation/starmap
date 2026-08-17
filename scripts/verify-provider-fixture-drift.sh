#!/usr/bin/env bash
set -euo pipefail

# Enforces the reviewed fixture maximum-age policy and compares every governed
# fixture against the live provider wire shape. Only a live capture clears a
# stale or drifted fixture, so this gate needs catalog-acquisition credentials
# and never runs in the offline pull-request path.
#
# Each provider fixture belongs to the client that proves its wire contract, so
# this gate covers both the OpenAI-compatible clients and every custom protocol
# client.
#
# Clear a reported fixture with: make testdata PROVIDER=<provider-id>

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT="$(mktemp "${TMPDIR:-/tmp}/starmap-fixture-drift.XXXXXX")"
trap 'rm -f "$OUTPUT"' EXIT

cd "$ROOT"

status=0
STARMAP_PROVIDER_FIXTURE_CURRENCY=1 \
	GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.5}" \
	go test ./internal/providers/openai ./internal/providers/anthropic -count=1 -v \
	-run '^(TestOpenAICompatibleProviderFixtureCurrency|TestAnthropicProviderFixtureCurrency)$' \
	>"$OUTPUT" 2>&1 || status=$?

credential_names=(
	ALIBABA_MODEL_STUDIO_API_KEY
	ANTHROPIC_API_KEY
	CEREBRAS_API_KEY
	DASHSCOPE_API_KEY
	DEEPINFRA_TOKEN
	DEEPSEEK_API_KEY
	FIREWORKS_API_KEY
	GOOGLE_API_KEY
	GROQ_API_KEY
	MOONSHOT_API_KEY
	OPENAI_API_KEY
)
for name in "${credential_names[@]}"; do
	value="$(printenv "$name" 2>/dev/null || true)"
	if [ -n "$value" ] && grep -Fq -- "$value" "$OUTPUT"; then
		printf 'fixture drift verification leaked %s\n' "$name" >&2
		exit 1
	fi
done

printf 'Provider fixture currency (credential values absent):\n'
grep -E '^(=== NAME|[[:space:]]*--- (PASS|FAIL|SKIP)|[[:space:]]+provider_catalog_contract_test\.go:)' "$OUTPUT" |
	sed -e 's/^=== NAME[[:space:]]*/provider /' -e 's/^[[:space:]]*//' || true

if [ "$status" -ne 0 ]; then
	printf '\nOne or more fixtures are stale or no longer mirror the provider.\n' >&2
	printf 'Refresh each reported provider: make testdata PROVIDER=<provider-id>\n' >&2
	exit "$status"
fi
