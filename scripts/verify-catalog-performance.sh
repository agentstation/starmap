#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MAX_NS_PER_OP="${STARMAP_CATALOG_MAX_NS_PER_OP:-10000}"
EXPECTED_RUNS="${STARMAP_CATALOG_BENCH_RUNS:-3}"

cd "$ROOT"

output="$(go test . -run '^$' -bench '^BenchmarkClientCatalog$' -benchmem -count="$EXPECTED_RUNS" 2>&1)"
printf '%s\n' "$output"

printf '%s\n' "$output" | awk \
	-v max_ns="$MAX_NS_PER_OP" \
	-v expected="$EXPECTED_RUNS" '
$1 ~ /^BenchmarkClientCatalog-/ {
	runs++
	ns = $3 + 0
	bytes = $5 + 0
	allocs = $7 + 0
	if (ns > max_ns) {
		printf "catalog performance gate failed: %.2f ns/op exceeds %.2f ns/op\n", ns, max_ns > "/dev/stderr"
		failed = 1
	}
	if (bytes != 0 || allocs != 0) {
		printf "catalog performance gate failed: %.0f B/op and %.0f allocs/op, want zero\n", bytes, allocs > "/dev/stderr"
		failed = 1
	}
}
END {
	if (runs != expected) {
		printf "catalog performance gate failed: found %d benchmark runs, want %d\n", runs, expected > "/dev/stderr"
		failed = 1
	}
	exit failed
}'

printf 'catalog performance gate passed: <= %s ns/op, 0 B/op, 0 allocs/op across %s runs\n' \
	"$MAX_NS_PER_OP" "$EXPECTED_RUNS"
