#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
URL="${STARMAP_MODELS_DEV_URL:-https://models.dev/api.json}"
TARGET="${STARMAP_EMBEDDED_MODELSDEV_PATH:-${ROOT}/internal/embedded/sources/models.dev/api.json}"
CURL_BIN="${STARMAP_CURL_BIN:-curl}"
GO_BIN="${STARMAP_GO_BIN:-go}"

mkdir -p "$(dirname "$TARGET")"
CANDIDATE="$(mktemp "$(dirname "$TARGET")/.api.json.download.XXXXXX")"
trap 'rm -f "$CANDIDATE"' EXIT

"$CURL_BIN" --fail --silent --show-error --location --output "$CANDIDATE" "$URL"

if [[ -n "${STARMAP_MODELSDEV_PROMOTER_BIN:-}" ]]; then
  "$STARMAP_MODELSDEV_PROMOTER_BIN" --input "$CANDIDATE" --output "$TARGET"
else
  cd "$ROOT"
  "$GO_BIN" run ./cmd/starmap-modelsdev-promote --input "$CANDIDATE" --output "$TARGET"
fi
