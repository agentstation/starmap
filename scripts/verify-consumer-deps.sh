#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODULE="$ROOT/testdata/consumers/read-only"
MAX_PACKAGES=160
DEPS="$(mktemp "${TMPDIR:-/tmp}/starmap-consumer-deps.XXXXXX")"
trap 'rm -f "$DEPS"' EXIT

(
	cd "$MODULE"
	GOWORK=off go test ./...
	GOWORK=off go list -deps -f '{{.ImportPath}}' . | LC_ALL=C sort -u >"$DEPS"
)

package_count="$(wc -l <"$DEPS" | tr -d '[:space:]')"
if [ "$package_count" -gt "$MAX_PACKAGES" ]; then
	printf 'read-only consumer dependency closure is %s packages; budget is %s\n' \
		"$package_count" "$MAX_PACKAGES" >&2
	exit 1
fi

banned_pattern='^(github\.com/agentstation/starmap/(acquisition|internal/(catalog/pipeline|providers|server|sources)(/|$)|pkg/(catalogremote|catalogscheduler|sources|sync)(/|$))|cloud\.google\.com/go/|google\.golang\.org/(genai|grpc)(/|$)|go\.opentelemetry\.io/otel(/|$)|github\.com/gorilla/websocket(/|$)|github\.com/spf13/cobra(/|$)|modernc\.org/sqlite(/|$)|github\.com/(mattn|ncruces)/go-sqlite3(/|$))'
banned="$(grep -E "$banned_pattern" "$DEPS" || true)"
if [ -n "$banned" ]; then
	printf 'read-only consumer imports forbidden implementation dependencies:\n%s\n' \
		"$banned" >&2
	exit 1
fi

printf 'read-only consumer dependency closure: %s/%s packages; forbidden families absent\n' \
	"$package_count" "$MAX_PACKAGES"
