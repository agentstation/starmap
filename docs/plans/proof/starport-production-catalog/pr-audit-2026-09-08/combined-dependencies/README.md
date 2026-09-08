# Combined Starmap dependency check

The isolated branch applies dependency PRs #128, #129, #130, and #131 to catalog plan commit `14ccae87`.
The combined dependency head is `9575418e`.
No dependency update reached the canonical plan branch or a remote branch.

## Integration repairs

Overlapping AWS module edits required two conflict resolutions.
The combined tree retains these AWS versions:

| Module | Version |
| --- | --- |
| config | 1.33.2 |
| S3 | 1.110.0 |
| Secrets Manager | 1.47.0 |

The server-storage consumer retains the catalog branch's purego requirement for native filesystem checks.
No source, test, or module policy changes accompany those resolutions.

All six consumer modules initially retained old dependency checksums.
Their tidy checks failed with checksum differences.
Commit `9575418e` regenerates those six checksum files.
The root module and all six consumer modules now pass tidy checks.

## Verification

The six consumer integration checks pass after the checksum repair.
The filesystem and S3 consumer covers startup, conditional publication, and reactive restart.
Module verification reports that all modules verify.
Ago reports zero findings, stale ignores, and errors.
The Go 1.25.12 authentication and S3 suite passes 76 test events and two package results without failures or skips.

The combined authentication, S3, provider-source, and acquisition suite passes 169 test events and four package results.
It records no failures or skips.

## Coverage limits

The authentication tests exercise Starmap adapters with injected cloud SDK reads.
They do not prove live AWS or Azure authentication or the complete default credential chain.
The S3 tests exercise real SDK HTTP requests against local test endpoints.
No cloud credential or provider API call occurs.

The four dependency PRs remain separate merge candidates.
This combined check does not replace their current-head checks or authorize default-branch merges.
Starport #368 still needs its separate SQLite, MySQL, credential, documentation, and catalog checks.

The [verification record](verification.json) binds each capture to its exact bytes and source commit.
