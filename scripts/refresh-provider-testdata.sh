#!/usr/bin/env bash
set -euo pipefail

provider="${1:-}"
if [[ ! "$provider" =~ ^[a-z0-9-]+$ ]]; then
  echo "provider must use lowercase letters, digits, or hyphens" >&2
  exit 2
fi

fixture="internal/providers/${provider}/testdata/models_list.json"
metadata="internal/providers/${provider}/testdata/models_list.metadata.json"
if [[ ! -f "$fixture" || ! -f "$metadata" ]]; then
  echo "provider fixture or metadata is missing for ${provider}" >&2
  exit 3
fi

before="$(cksum "$fixture" "$metadata")"
if [[ -n "${STARMAP_GO_TEST_BIN:-}" ]]; then
  "$STARMAP_GO_TEST_BIN" test "./internal/providers/${provider}" -update -v
elif command -v devbox >/dev/null 2>&1; then
  devbox run go test "./internal/providers/${provider}" -update -v
else
  go test "./internal/providers/${provider}" -update -v
fi
after="$(cksum "$fixture" "$metadata")"

if [[ "$before" == "$after" ]]; then
  echo "provider refresh completed without updating payload/metadata for ${provider}" >&2
  exit 4
fi
