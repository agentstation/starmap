#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT="$(mktemp "${TMPDIR:-/tmp}/starmap-live-providers.XXXXXX")"
trap 'rm -f "$OUTPUT"' EXIT

cd "$ROOT"

status=0
GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.5}" \
	go run ./cmd/starmap providers --test --timeout 30s --no-color >"$OUTPUT" 2>&1 || status=$?

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
		printf 'live provider verification leaked %s\n' "$name" >&2
		exit 1
	fi
done

printf 'Live provider matrix (credential values absent):\n'
awk '
/^Testing [^ ]+\.\.\. / {
	provider = $2
	sub(/\.\.\.$/, "", provider)
	if ($0 ~ /Success$/) {
		printf "Testing %s... Success\n", provider
	} else if ($0 ~ /Skipped$/) {
		printf "Testing %s... Skipped\n", provider
	} else {
		printf "Testing %s... Failed (protected diagnostics suppressed)\n", provider
	}
}
' "$OUTPUT"

if [ "$status" -ne 0 ]; then
	printf 'live provider verification failed; rerun locally for protected diagnostics\n' >&2
	exit "$status"
fi
