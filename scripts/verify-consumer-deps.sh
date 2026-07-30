#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
READ_ONLY_MODULE="$ROOT/testdata/consumers/read-only"
STORE_ONLY_MODULE="$ROOT/testdata/consumers/store-only"
PINNED_ARTIFACT_MODULE="$ROOT/testdata/consumers/pinned-artifact"
SERVER_EMBED_MODULE="$ROOT/testdata/consumers/server-embed"
REMOTE_SUBSCRIBER_MODULE="$ROOT/testdata/consumers/remote-subscriber"
SERVER_STORAGE_MODULE="$ROOT/testdata/consumers/server-storage"
MAX_NON_STANDARD_PACKAGES=32
PINNED_MAX_NON_STANDARD_PACKAGES=32
SERVER_MAX_PACKAGES=260
REMOTE_MAX_PACKAGES=240
SERVER_STORAGE_MAX_PACKAGES=340
DEPS="$(mktemp "${TMPDIR:-/tmp}/starmap-consumer-deps.XXXXXX")"
NON_STANDARD_DEPS="$(mktemp "${TMPDIR:-/tmp}/starmap-consumer-non-standard-deps.XXXXXX")"
STORE_DEPS="$(mktemp "${TMPDIR:-/tmp}/starmap-store-consumer-deps.XXXXXX")"
PINNED_DEPS="$(mktemp "${TMPDIR:-/tmp}/starmap-pinned-consumer-deps.XXXXXX")"
PINNED_NON_STANDARD_DEPS="$(mktemp "${TMPDIR:-/tmp}/starmap-pinned-consumer-non-standard-deps.XXXXXX")"
SERVER_DEPS="$(mktemp "${TMPDIR:-/tmp}/starmap-server-consumer-deps.XXXXXX")"
REMOTE_DEPS="$(mktemp "${TMPDIR:-/tmp}/starmap-remote-consumer-deps.XXXXXX")"
SERVER_STORAGE_DEPS="$(mktemp "${TMPDIR:-/tmp}/starmap-server-storage-deps.XXXXXX")"
trap 'rm -f "$DEPS" "$NON_STANDARD_DEPS" "$STORE_DEPS" "$PINNED_DEPS" "$PINNED_NON_STANDARD_DEPS" "$SERVER_DEPS" "$REMOTE_DEPS" "$SERVER_STORAGE_DEPS"' EXIT

(
	cd "$READ_ONLY_MODULE"
	GOWORK=off go test ./...
	GOWORK=off go list -deps -f '{{.ImportPath}}' . | LC_ALL=C sort -u >"$DEPS"
	GOWORK=off go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' . |
		sed '/^$/d' | LC_ALL=C sort -u >"$NON_STANDARD_DEPS"
)

(
	cd "$STORE_ONLY_MODULE"
	GOWORK=off go test ./...
	GOWORK=off go list -deps -f '{{.ImportPath}}' . | LC_ALL=C sort -u >"$STORE_DEPS"
)

(
	cd "$PINNED_ARTIFACT_MODULE"
	GOWORK=off go test ./...
	GOWORK=off go list -deps -f '{{.ImportPath}}' . |
		LC_ALL=C sort -u >"$PINNED_DEPS"
	GOWORK=off go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' . |
		sed '/^$/d' | LC_ALL=C sort -u >"$PINNED_NON_STANDARD_DEPS"
)

(
	cd "$SERVER_EMBED_MODULE"
	GOWORK=off go test ./...
	GOWORK=off go list -deps -f '{{.ImportPath}}' . | LC_ALL=C sort -u >"$SERVER_DEPS"
)

(
	cd "$REMOTE_SUBSCRIBER_MODULE"
	GOWORK=off go test ./...
	GOWORK=off go list -deps -f '{{.ImportPath}}' . | LC_ALL=C sort -u >"$REMOTE_DEPS"
)

(
	cd "$SERVER_STORAGE_MODULE"
	GOWORK=off go test ./...
	GOWORK=off go list -deps -test -f '{{.ImportPath}}' . |
		LC_ALL=C sort -u >"$SERVER_STORAGE_DEPS"
)

total_package_count="$(wc -l <"$DEPS" | tr -d '[:space:]')"
non_standard_package_count="$(wc -l <"$NON_STANDARD_DEPS" | tr -d '[:space:]')"
if [ "$non_standard_package_count" -gt "$MAX_NON_STANDARD_PACKAGES" ]; then
	printf 'read-only consumer non-standard dependency closure is %s packages; budget is %s\n' \
		"$non_standard_package_count" "$MAX_NON_STANDARD_PACKAGES" >&2
	exit 1
fi

banned_pattern='^(github\.com/agentstation/starmap/(acquisition|internal/(catalog/pipeline|providers|server|sources)(/|$)|pkg/(catalogremote|catalogscheduler|sources|sync)(/|$))|github\.com/aws/(aws-sdk-go-v2|smithy-go)(/|$)|cloud\.google\.com/go/|google\.golang\.org/(genai|grpc)(/|$)|go\.opentelemetry\.io/otel(/|$)|github\.com/gorilla/websocket(/|$)|github\.com/spf13/cobra(/|$)|modernc\.org/sqlite(/|$)|github\.com/(mattn|ncruces)/go-sqlite3(/|$))'
banned="$(grep -E "$banned_pattern" "$DEPS" || true)"
if [ -n "$banned" ]; then
	printf 'read-only consumer imports forbidden implementation dependencies:\n%s\n' \
		"$banned" >&2
	exit 1
fi

store_banned_pattern='^(database/sql$|github\.com/agentstation/starmap/(acquisition|cmd|internal/(providers|server|sources)(/|$)|remote(/|$)|server(/|$)|pkg/(catalogremote|sources|sync)(/|$))|cloud\.google\.com/go/|google\.golang\.org/(genai|grpc)(/|$)|go\.opentelemetry\.io/otel(/|$)|github\.com/(mattn/go-sqlite3|ncruces/go-sqlite3|go-sql-driver/mysql|lib/pq|jackc/pgx)(/|$)|modernc\.org/sqlite(/|$))'
store_banned="$(grep -E "$store_banned_pattern" "$STORE_DEPS" || true)"
if [ -n "$store_banned" ]; then
	printf 'store-only consumer imports forbidden application or database implementations:\n%s\n' \
		"$store_banned" >&2
	exit 1
fi

pinned_non_standard_package_count="$(
	wc -l <"$PINNED_NON_STANDARD_DEPS" | tr -d '[:space:]'
)"
if [ "$pinned_non_standard_package_count" -gt "$PINNED_MAX_NON_STANDARD_PACKAGES" ]; then
	printf 'pinned-artifact consumer non-standard dependency closure is %s packages; budget is %s\n' \
		"$pinned_non_standard_package_count" "$PINNED_MAX_NON_STANDARD_PACKAGES" >&2
	exit 1
fi
pinned_banned_pattern='^(github\.com/agentstation/starmap/(acquisition|cmd|remote(/|$)|server(/|$)|internal/(catalog/pipeline|providers|server|sources)(/|$)|pkg/(catalogremote|sources|sync|catalogstore/s3)(/|$))|github\.com/aws/(aws-sdk-go-v2|smithy-go)(/|$)|cloud\.google\.com/go/|google\.golang\.org/(genai|grpc)(/|$)|go\.opentelemetry\.io/otel(/|$)|github\.com/gorilla/websocket(/|$)|github\.com/spf13/cobra(/|$)|modernc\.org/sqlite(/|$)|github\.com/(mattn|ncruces)/go-sqlite3(/|$))'
pinned_banned="$(
	grep -E "$pinned_banned_pattern" "$PINNED_DEPS" || true
)"
if [ -n "$pinned_banned" ]; then
	printf 'pinned-artifact consumer imports forbidden online/acquisition dependencies:\n%s\n' \
		"$pinned_banned" >&2
	exit 1
fi
for required in \
	github.com/agentstation/starmap \
	github.com/agentstation/starmap/pkg/catalogartifact \
	github.com/agentstation/starmap/pkg/catalogstore; do
	if ! grep -Fxq "$required" "$PINNED_DEPS"; then
		printf 'pinned-artifact consumer does not exercise required package %s\n' \
			"$required" >&2
		exit 1
	fi
done

server_package_count="$(wc -l <"$SERVER_DEPS" | tr -d '[:space:]')"
if [ "$server_package_count" -gt "$SERVER_MAX_PACKAGES" ]; then
	printf 'server-embed consumer dependency closure is %s packages; budget is %s\n' \
		"$server_package_count" "$SERVER_MAX_PACKAGES" >&2
	exit 1
fi
server_banned_pattern='^(github\.com/agentstation/starmap/(acquisition|internal/(catalog/pipeline|providers|sources)(/|$))|github\.com/aws/(aws-sdk-go-v2|smithy-go)(/|$)|cloud\.google\.com/go/|google\.golang\.org/(genai|grpc)(/|$)|go\.opentelemetry\.io/otel(/|$))'
server_banned="$(grep -E "$server_banned_pattern" "$SERVER_DEPS" || true)"
if [ -n "$server_banned" ]; then
	printf 'server-embed consumer imports forbidden acquisition dependencies:\n%s\n' \
		"$server_banned" >&2
	exit 1
fi

remote_package_count="$(wc -l <"$REMOTE_DEPS" | tr -d '[:space:]')"
if [ "$remote_package_count" -gt "$REMOTE_MAX_PACKAGES" ]; then
	printf 'remote-subscriber consumer dependency closure is %s packages; budget is %s\n' \
		"$remote_package_count" "$REMOTE_MAX_PACKAGES" >&2
	exit 1
fi
remote_banned_pattern='^(github\.com/agentstation/starmap/(acquisition|internal/(catalog/pipeline|providers|server|sources)(/|$)|server(/|$))|github\.com/aws/(aws-sdk-go-v2|smithy-go)(/|$)|cloud\.google\.com/go/|google\.golang\.org/(genai|grpc)(/|$)|go\.opentelemetry\.io/otel(/|$)|github\.com/gorilla/websocket(/|$)|github\.com/spf13/cobra(/|$)|modernc\.org/sqlite(/|$)|github\.com/(mattn|ncruces)/go-sqlite3(/|$))'
remote_banned="$(grep -E "$remote_banned_pattern" "$REMOTE_DEPS" || true)"
if [ -n "$remote_banned" ]; then
	printf 'remote-subscriber consumer imports forbidden implementation dependencies:\n%s\n' \
		"$remote_banned" >&2
	exit 1
fi

server_storage_package_count="$(
	wc -l <"$SERVER_STORAGE_DEPS" | tr -d '[:space:]'
)"
if [ "$server_storage_package_count" -gt "$SERVER_STORAGE_MAX_PACKAGES" ]; then
	printf 'server-storage consumer dependency closure is %s packages; budget is %s\n' \
		"$server_storage_package_count" "$SERVER_STORAGE_MAX_PACKAGES" >&2
	exit 1
fi
server_storage_banned_pattern='^(database/sql$|github\.com/agentstation/starmap/(acquisition|cmd|internal/(providers|sources)(/|$))|cloud\.google\.com/go/|google\.golang\.org/(genai|grpc)(/|$)|go\.opentelemetry\.io/otel(/|$)|github\.com/(mattn/go-sqlite3|ncruces/go-sqlite3|go-sql-driver/mysql|lib/pq|jackc/pgx)(/|$)|modernc\.org/sqlite(/|$))'
server_storage_banned="$(
	grep -E "$server_storage_banned_pattern" "$SERVER_STORAGE_DEPS" || true
)"
if [ -n "$server_storage_banned" ]; then
	printf 'server-storage consumer imports forbidden acquisition or database implementations:\n%s\n' \
		"$server_storage_banned" >&2
	exit 1
fi
for required in \
	github.com/agentstation/starmap/pkg/catalogstore/s3 \
	github.com/agentstation/starmap/remote \
	github.com/agentstation/starmap/server \
	github.com/aws/aws-sdk-go-v2/service/s3; do
	if ! grep -Fxq "$required" "$SERVER_STORAGE_DEPS"; then
		printf 'server-storage consumer does not exercise required package %s\n' \
			"$required" >&2
		exit 1
	fi
done

printf 'read-only consumer dependency closure: %s/%s non-standard packages (%s total on this platform); forbidden families absent\n' \
	"$non_standard_package_count" "$MAX_NON_STANDARD_PACKAGES" "$total_package_count"
printf 'store-only consumer: caller-owned adapter contract and publication passed; application/database implementations absent\n'
printf 'pinned-artifact consumer: %s/%s non-standard packages; offline verified activation passed; online/acquisition families absent\n' \
	"$pinned_non_standard_package_count" "$PINNED_MAX_NON_STANDARD_PACKAGES"
printf 'server-embed consumer dependency closure: %s/%s packages; acquisition families absent\n' \
	"$server_package_count" "$SERVER_MAX_PACKAGES"
printf 'server-embed consumer: external compile and lifecycle test passed\n'
printf 'remote-subscriber consumer dependency closure: %s/%s packages; forbidden families absent\n' \
	"$remote_package_count" "$REMOTE_MAX_PACKAGES"
printf 'remote-subscriber consumer: external compile and reactive lifecycle test passed\n'
printf 'server-storage consumer dependency closure: %s/%s packages; filesystem/S3 server and reactive restart matrix passed\n' \
	"$server_storage_package_count" "$SERVER_STORAGE_MAX_PACKAGES"
