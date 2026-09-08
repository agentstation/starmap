# PR 133 allocation hint repair

CodeQL reports an integer-overflow risk in `len(canonical)+3` at `catalog_aliases.go:20`.
The repair uses `len(canonical)` for the map capacity hint.
Map insertion handles additional legacy entries without this arithmetic.
Canonical precedence, copied values, and source authority remain unchanged.

The [verification record](verification.json) retains the original alert and exact check counts.
Go 1.26.6 passes 221 test results and two packages with the race detector.
Go 1.25.12 passes 23 focused test results and one package with the race detector.
Neither suite skips tests. Lint and Ago report zero findings.

A runtime overflow reproduction would require an impractically large map.
The original CodeQL finding supplies failure evidence.
The pushed head must pass a fresh scan before merge.
