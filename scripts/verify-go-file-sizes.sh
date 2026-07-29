#!/usr/bin/env bash
set -euo pipefail

ROOT="${STARMAP_GO_FILE_SIZE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
REVIEWS="${STARMAP_GO_FILE_SIZE_REVIEWS:-$ROOT/docs/reviews/GO_FILE_SIZE_REVIEWS.tsv}"
REVIEW_THRESHOLD=1000
JUSTIFICATION_THRESHOLD=1500
HARD_LIMIT=2000
failed=0

has_justification() {
	local path="$1"
	[ -f "$REVIEWS" ] || return 1
	awk -F '\t' -v path="$path" '
		$0 !~ /^#/ && $1 == path && length($2) > 0 { found = 1 }
		END { exit(found ? 0 : 1) }
	' "$REVIEWS"
}

while IFS= read -r -d '' file; do
	if head -n 20 "$file" | grep -Eq '^// Code generated .* DO NOT EDIT\.$'; then
		continue
	fi
	lines="$(awk 'END { print NR }' "$file")"
	path="${file#"$ROOT"/}"
	if [ "$lines" -gt "$REVIEW_THRESHOLD" ]; then
		printf '%s\t%s\n' "$lines" "$path"
	fi
	if [ "$lines" -ge "$HARD_LIMIT" ]; then
		printf '%s has %s lines; repository-authored Go files must stay below %s\n' \
			"$path" "$lines" "$HARD_LIMIT" >&2
		failed=1
		continue
	fi
	if [ "$lines" -gt "$JUSTIFICATION_THRESHOLD" ] &&
		! has_justification "$path"; then
		printf '%s has %s lines and needs a durable rationale in %s\n' \
			"$path" "$lines" "$REVIEWS" >&2
		failed=1
	fi
done < <(
	find "$ROOT" \
		\( -path "$ROOT/.git" -o -path "$ROOT/vendor" -o -path "$ROOT/dist" \) -prune \
		-o -type f -name '*.go' -print0 |
		LC_ALL=C sort -z
)

if [ "$failed" -ne 0 ]; then
	exit 1
fi

printf 'Go file-size verification passed: review >%s, justify >%s, fail >=%s lines\n' \
	"$REVIEW_THRESHOLD" "$JUSTIFICATION_THRESHOLD" "$HARD_LIMIT"
