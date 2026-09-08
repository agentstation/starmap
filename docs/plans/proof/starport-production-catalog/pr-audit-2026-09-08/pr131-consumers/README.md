# PR 131 consumer dependency repair

PR #131 selects AWS SDK v1.46.0 and Secrets Manager v1.48.0.
The server-storage consumer still pins the prior SDK module graph.
CI stops with `go: updates to go.mod needed`.

The repair synchronizes that consumer with the selected root dependencies.
All six consumer checks pass. No dependency limit or assertion changes.
The [verification record](verification.json) binds the published head and captures.

Explicit SDK credential tests and S3 tests pass 87 test results and two packages on each supported Go toolchain.
The credential tests use local TLS services and real SDK credential chains.
These checks do not qualify live cloud policies.
The first overlay used an unresolved temporary path and omitted the additional tests.
The corrected overlay uses the actual directory and requires all nine credential scenarios to pass.
Both initial and corrected captures remain available.

The pre-PR gate skips model review because only module and checksum files change.
The repair now runs native and required CI in the existing PR.
Merge remains pending.
