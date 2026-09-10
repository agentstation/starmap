#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_REF="${CATALOG_ALIAS_BASE_REF:-HEAD^}"
BASE_COMMIT="$(git -C "$ROOT" rev-parse --verify --end-of-options "${BASE_REF}^{commit}")"
ALIAS_PROOF_DIR="$(mktemp -d "${TMPDIR:-/tmp}/starmap-alias-history.XXXXXX")"
trap 'rm -rf "$ALIAS_PROOF_DIR"' EXIT

git -C "$ROOT" archive --format=tar "$BASE_COMMIT" -- internal/embedded/catalog > "$ALIAS_PROOF_DIR/previous.tar"
tar -xf "$ALIAS_PROOF_DIR/previous.tar" -C "$ALIAS_PROOF_DIR"

cd "$ROOT"
go run ./cmd/starmap-bootstrap-manifest \
  --catalog-dir "$ROOT/internal/embedded/catalog" \
  --previous-catalog-dir "$ALIAS_PROOF_DIR/internal/embedded/catalog" \
  --output "$ALIAS_PROOF_DIR/generation.json" > "$ALIAS_PROOF_DIR/report.json"
printf 'Canonical rename history preserves predecessor %s\n' "$BASE_COMMIT"
