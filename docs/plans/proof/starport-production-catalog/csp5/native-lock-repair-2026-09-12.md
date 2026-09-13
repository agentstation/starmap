# CSP5 Windows ownership-lock correction

Commit `d15c6ef4c` validates empty files without reading a locked byte range.
The [verification record](native-lock-repair-2026-09-12/verification.json) binds all checks to the five corrected source files.
The [native failure record](native-locks-2026-09-12.md) preserves both failing Windows runs on the parent commit.

The reader retains its access, file identity, size, and final change checks.
A regression test holds a real exclusive lock and confirms that another writer cannot enter after the read.
A second test rejects file growth during final validation.

The workspace snapshot fixture retains empty locked entries and detects changed file contents after lock release.
The retention serialization test now reports a collector error before the retirement hook instead of waiting indefinitely.
Its serialization assertions and timeout remain unchanged.

| Local check | Result |
| --- | --- |
| Four affected packages, Go 1.26.6, race detector | 769 passing test events, zero failures or skips |
| Focused regressions, Go 1.25.12, race detector | Five passing test events across three packages, zero failures or skips |
| Restriction policy | Zero findings, stale ignores, or errors |
| Affected-package lint | Zero issues |
| Maintained prose | 1,678 files, zero diagnostics |
| Windows amd64 build | All four affected test packages compile |
| Selected CSP5 contract | Twelve passing subcases, 21 commands, 42 passing test events |

The initial prose check failed on an obsolete scratch copy of historical output.
The cleanup verified identical bytes in the committed compressed artifact before deleting only that duplicate.
The verification record preserves the failure, cleanup receipt, and passing retry.

Eight task subcases retain Starport consumer requirements.
The selected component report does not qualify the complete product.
Compilation does not qualify Windows behavior.

Review session `57502` failed during its fifth section because the Codex engine exited with status 1.
It supplied no final verdict. The captured error does not establish the underlying engine cause.
Required retry `87440` uses the unchanged source, complete branch, and required cross-lab profile.
Native CI and merge remain open for this correction.
Shared catalog cleanup and its pending coordinator decision remain required for CSP5 completion.
