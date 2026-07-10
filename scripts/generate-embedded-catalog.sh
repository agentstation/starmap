#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CATALOG_DIR="${STARMAP_EMBEDDED_CATALOG_PATH:-${ROOT}/internal/embedded/catalog}"
MANIFEST_PATH="${STARMAP_EMBEDDED_MANIFEST_PATH:-${CATALOG_DIR}/generation.json}"
REPORT_PATH="${STARMAP_GENERATION_REPORT_PATH:-${TMPDIR:-/tmp}/starmap-catalog-generation-report.json}"
REFRESH_BIN="${STARMAP_MODELSDEV_REFRESH_BIN:-${ROOT}/scripts/refresh-embedded-modelsdev.sh}"
PROVIDER="${1:-}"

if [[ -n "$PROVIDER" && ! "$PROVIDER" =~ ^[a-z0-9-]+$ ]]; then
  printf 'provider must use lowercase letters, digits, or hyphens\n' >&2
  exit 2
fi
if (( $# > 1 )); then
  printf 'usage: %s [provider]\n' "$0" >&2
  exit 2
fi

run_starmap() {
  if [[ -n "${STARMAP_BIN:-}" ]]; then
    "$STARMAP_BIN" "$@"
    return
  fi
  cd "$ROOT"
  "${STARMAP_GO_BIN:-go}" run ./cmd/starmap "$@"
}

run_manifest() {
  if [[ -n "${STARMAP_BOOTSTRAP_MANIFEST_BIN:-}" ]]; then
    "$STARMAP_BOOTSTRAP_MANIFEST_BIN" "$@"
    return
  fi
  cd "$ROOT"
  "${STARMAP_GO_BIN:-go}" run ./cmd/starmap-bootstrap-manifest "$@"
}

"$REFRESH_BIN"

UPDATE_ARGS=(--output-dir "$CATALOG_DIR" --force -y)
if [[ "${STARMAP_GENERATION_NONINTERACTIVE:-}" == "1" ]]; then
  UPDATE_ARGS+=(--skip-dep-prompts)
fi

if [[ -n "$PROVIDER" ]]; then
  run_starmap update "$PROVIDER" "${UPDATE_ARGS[@]}"
else
  run_starmap update "${UPDATE_ARGS[@]}"
fi

run_manifest --catalog-dir "$CATALOG_DIR" --output "$MANIFEST_PATH" > "$REPORT_PATH"
run_starmap validate catalog
