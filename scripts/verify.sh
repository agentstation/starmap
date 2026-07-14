#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMPDIR="$(mktemp -d "${TMPDIR:-/tmp}/starmap-verify.XXXXXX")"
trap 'rm -rf "$TMPDIR"' EXIT
VERIFY_CATALOG_PATH="$ROOT/internal/embedded/catalog"
VERIFY_CATALOG_DATABASE_PATH="$TMPDIR/catalog"
VERIFY_HOME="$TMPDIR/home"
mkdir -p "$VERIFY_HOME"

# Verification must never exercise live provider credentials. Derive every
# configured environment input from the canonical provider configuration,
# remove it from this process, and disable repository-local dotenv loading.
while IFS= read -r name; do
	[ -n "$name" ] && unset "$name"
done < <(
	{
		sed -nE 's/.*(env:|name:|base_url_env:)[[:space:]]+([A-Z][A-Z0-9_]+)$/\2/p' "$VERIFY_CATALOG_PATH/providers.yaml"
		sed -nE 's/^[[:space:]]*-[[:space:]]+([A-Z][A-Z0-9_]+)$/\1/p' "$VERIFY_CATALOG_PATH/providers.yaml"
	} | sort -u
)

# Cloud-chain SDKs also inspect conventional environment variables and files
# below HOME that are intentionally provider-inferred rather than catalog
# configuration. Keep verification from observing developer/CI credentials.
for name in \
	AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN AWS_PROFILE \
	AWS_CONFIG_FILE AWS_SHARED_CREDENTIALS_FILE AWS_WEB_IDENTITY_TOKEN_FILE \
	AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_TENANT_ID AZURE_FEDERATED_TOKEN_FILE \
	GOOGLE_APPLICATION_CREDENTIALS GOOGLE_CLOUD_PROJECT GOOGLE_CLOUD_LOCATION \
	OCI_CLI_CONFIG_FILE OCI_CONFIG_FILE OCI_CLI_PROFILE; do
	unset "$name"
done
export AWS_EC2_METADATA_DISABLED=true
export STARMAP_DISABLE_DOTENV=1

cd "$ROOT"

run() {
	printf '\n==> %s\n' "$*"
	"$@"
}

run_optional_lint() {
	if command -v golangci-lint >/dev/null 2>&1; then
		run golangci-lint run
		return
	fi
	if command -v devbox >/dev/null 2>&1; then
		run devbox run golangci-lint run
		return
	fi
	printf '\n==> skipping golangci-lint: command not found\n'
}

check_coverage() {
	local pkg="$1"
	local min="$2"
	local profile
	local output coverage
	profile="$TMPDIR/$(printf '%s' "$pkg" | tr '/.' '__').out"

	printf '\n==> coverage %s >= %s%%\n' "$pkg" "$min"
	output="$(go test -covermode=atomic -coverprofile="$profile" "$pkg" 2>&1)"
	printf '%s\n' "$output"
	coverage="$(printf '%s\n' "$output" | awk '/coverage:/ { for (i = 1; i <= NF; i++) if ($i ~ /%$/) { gsub("%", "", $i); print $i; exit } }')"

	if [ -z "$coverage" ]; then
		printf 'coverage check failed: no coverage percentage found for %s\n' "$pkg" >&2
		exit 1
	fi

	awk -v got="$coverage" -v want="$min" 'BEGIN { if ((got + 0) < (want + 0)) exit 1 }' || {
		printf 'coverage check failed: %s has %s%% coverage, want at least %s%%\n' "$pkg" "$coverage" "$min" >&2
		exit 1
	}
}

check_critical_coverage() {
	check_coverage ./internal/attribution 85
	check_coverage ./internal/attribution/matcher 75
	check_coverage ./internal/catalog/pipeline 70
	check_coverage ./internal/catalog/query 75
	check_coverage ./internal/providers/registry 80
	check_coverage ./internal/sources/providers 75
	check_coverage ./internal/server/events 70
	check_coverage ./internal/server/middleware 90
	check_coverage ./internal/server/params 95
	check_coverage ./internal/server/response 95
	check_coverage ./internal/server/sse 90
	check_coverage ./internal/server/websocket 85
	check_coverage ./internal/transport 40
	check_coverage ./pkg/authority 90
	check_coverage ./pkg/catalogs 55
	check_coverage ./pkg/errors 80
	check_coverage ./pkg/reconciler 75
	check_coverage ./pkg/sources 35
}

if [ "${STARMAP_VERIFY_COVERAGE_ONLY:-}" = "1" ]; then
	check_critical_coverage
	printf '\ncritical seam coverage passed\n'
	exit 0
fi

run go test ./...
run go test ./... -race -short
run go vet ./...
run ./scripts/verify-catalog-performance.sh
run_optional_lint

check_critical_coverage

run make docs-check
run git diff --check

run go build -o "$TMPDIR/starmap" ./cmd/starmap
run env HOME="$VERIFY_HOME" "$TMPDIR/starmap" version
run env HOME="$VERIFY_HOME" CATALOG_PATH="$VERIFY_CATALOG_DATABASE_PATH" CATALOG_EXPORT_PATH="$VERIFY_CATALOG_PATH" \
	"$TMPDIR/starmap" validate catalog
printf '\n==> isolated credential-free provider listing\n'
(
	cd "$TMPDIR"
	env HOME="$VERIFY_HOME" CATALOG_PATH="$VERIFY_CATALOG_DATABASE_PATH" \
	CATALOG_EXPORT_PATH="$VERIFY_CATALOG_PATH" \
	"$TMPDIR/starmap" providers --output json --limit 1
)
run env HOME="$VERIFY_HOME" CATALOG_PATH="$VERIFY_CATALOG_DATABASE_PATH" CATALOG_EXPORT_PATH="$VERIFY_CATALOG_PATH" \
	"$TMPDIR/starmap" models list --limit 5

printf '\nrepository verification passed\n'
