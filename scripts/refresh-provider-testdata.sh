#!/usr/bin/env bash
set -euo pipefail

provider="${1:-}"
if [[ ! "$provider" =~ ^[a-z0-9-]+$ ]]; then
  echo "provider must use lowercase letters, digits, or hyphens" >&2
  exit 2
fi

if [[ ! -d "internal/providers/${provider}" ]]; then
  echo "provider package is missing for ${provider}" >&2
  exit 3
fi

if [[ -n "${STARMAP_GO_RUN_BIN:-}" ]]; then
  "$STARMAP_GO_RUN_BIN" run ./cmd/starmap-provider-testdata-refresh --provider "$provider"
elif command -v devbox >/dev/null 2>&1; then
  devbox run go run ./cmd/starmap-provider-testdata-refresh --provider "$provider"
else
  go run ./cmd/starmap-provider-testdata-refresh --provider "$provider"
fi
