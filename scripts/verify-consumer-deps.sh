#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
READ_ONLY_MODULE="$ROOT/testdata/consumers/read-only"
STORE_ONLY_MODULE="$ROOT/testdata/consumers/store-only"
SERVER_EMBED_MODULE="$ROOT/testdata/consumers/server-embed"
MAX_PACKAGES=160
SERVER_MAX_PACKAGES=260
DEPS="$(mktemp "${TMPDIR:-/tmp}/starmap-consumer-deps.XXXXXX")"
SERVER_DEPS="$(mktemp "${TMPDIR:-/tmp}/starmap-server-consumer-deps.XXXXXX")"
trap 'rm -f "$DEPS" "$SERVER_DEPS"' EXIT

(
	cd "$READ_ONLY_MODULE"
	GOWORK=off go test ./...
	GOWORK=off go list -deps -f '{{.ImportPath}}' . | LC_ALL=C sort -u >"$DEPS"
)

(
	cd "$STORE_ONLY_MODULE"
	GOWORK=off go test ./...
)

(
	cd "$SERVER_EMBED_MODULE"
	GOWORK=off go test ./...
	GOWORK=off go list -deps -f '{{.ImportPath}}' . | LC_ALL=C sort -u >"$SERVER_DEPS"
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

server_package_count="$(wc -l <"$SERVER_DEPS" | tr -d '[:space:]')"
if [ "$server_package_count" -gt "$SERVER_MAX_PACKAGES" ]; then
	printf 'server-embed consumer dependency closure is %s packages; budget is %s\n' \
		"$server_package_count" "$SERVER_MAX_PACKAGES" >&2
	exit 1
fi
server_banned_pattern='^(github\.com/agentstation/starmap/(acquisition|internal/(catalog/pipeline|providers|sources)(/|$))|cloud\.google\.com/go/|google\.golang\.org/(genai|grpc)(/|$)|go\.opentelemetry\.io/otel(/|$))'
server_banned="$(grep -E "$server_banned_pattern" "$SERVER_DEPS" || true)"
if [ -n "$server_banned" ]; then
	printf 'server-embed consumer imports forbidden acquisition dependencies:\n%s\n' \
		"$server_banned" >&2
	exit 1
fi

printf 'read-only consumer dependency closure: %s/%s packages; forbidden families absent\n' \
	"$package_count" "$MAX_PACKAGES"
printf 'store-only consumer: external compile and publication test passed\n'
printf 'server-embed consumer dependency closure: %s/%s packages; acquisition families absent\n' \
	"$server_package_count" "$SERVER_MAX_PACKAGES"
printf 'server-embed consumer: external compile and lifecycle test passed\n'
