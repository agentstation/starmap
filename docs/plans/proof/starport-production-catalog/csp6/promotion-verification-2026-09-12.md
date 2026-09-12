# CSP6 exact promotion verification

Local commit `3ae364518a24d19182934319715a8b93b487f309` adds an exact promotion check to the release command.
The [verification record](promotion-verification-2026-09-12/verification.json) binds four source files and thirteen recorded checks to their source inputs.

`--verify-promotion-dir` requires `--promotion-release-dir` and rejects incompatible modes.
The command validates local release assets, canonical catalog bytes, generation metadata, and the complete endpoint projection.
It emits a stable report only after every comparison passes.
The documentation distinguishes this local check from provenance verification and GitHub merge verification.

Both Go 1.26.6 and Go 1.25.12 pass thirteen focused race test events, with no skips.
The complete release-command suite passes 48 race test events on Go 1.26.6, with no skips, in 95.750 seconds.
Final package lint reports zero issues. Repository Go policy reports no findings, stale ignores, or errors.
Maintained prose passes across 1,660 files with zero diagnostics.

The final regression test fails against the parent command through a recorded Go overlay.
That overlay removes the new implementation file and restores the exact parent command bytes.
The failure reports the missing promotion flag. The same permanent test passes with the new implementation.

The record also preserves the first build error and the initial fixture mismatch.
The corrected fixture stages canonical bytes read from its projected YAML.
The implementation retains its exact payload comparison.

Source admission, immutable receipts, checked bot promotion, workflow integration, and complete A05 qualification remain unfinished.
This commit has no PR, required pre-PR review, native qualification, or merge.
CSP5 dependency qualification now takes priority after both Windows jobs failed.
