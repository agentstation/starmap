#!/usr/bin/env bash
set -euo pipefail

provider="${1:-}"
source_id="${2:-}"
if [[ ! "$provider" =~ ^[a-z0-9-]+$ ]]; then
  echo "provider must use lowercase letters, digits, or hyphens" >&2
  exit 2
fi

args=(run ./cmd/provider-fixtures refresh --provider "$provider")
if [[ -n "$source_id" ]]; then
  args+=(--source "$source_id")
fi

if [[ -n "${STARMAP_GO_RUN_BIN:-}" ]]; then
  "$STARMAP_GO_RUN_BIN" "${args[@]}"
elif command -v devbox >/dev/null 2>&1; then
  devbox run go "${args[@]}"
else
  go "${args[@]}"
fi
