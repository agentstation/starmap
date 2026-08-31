# TW1 generated-document detection proof

Date: 2026-08-30

The shared technical-writing linter now detects generated documentation from
common language and documentation tools. It checks the first 16 KiB for exact
generated-file notices, generated annotations, XML auto-generated blocks, and
known HTML generator metadata.

The test matrix covers Doxygen, Javadoc, JSDoc, MkDocs, rustdoc, Sphinx,
TypeDoc, `@generated`, and late HTML metadata. Negative tests keep ordinary
prose and unknown generator metadata in scope.

## Evidence

- The baseline implementation discovered eight generated fixtures that the new
  marker cases expected it to skip.
- The complete technical-writing unit suite passed 88 tests.
- The Agent Skills validator passed all five skills in the repository.
- Strict lint reported no diagnostics in the changed skill documentation.
- Commit `c4397a3` is on branch
  `codex/technical-writing-generated-docs` in pull request #10.
- A recursive comparison, excluding Python cache files, found no difference
  between the repository skill and the global installation.
