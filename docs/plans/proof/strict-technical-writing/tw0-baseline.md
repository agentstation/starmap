# TW0 baseline

Date: 2026-08-30

Starmap baseline: `main@b36d8041392ebee923e790737835a56a1fb98831`

Skills baseline: `main@dd86deda63730848820ff1cf7e80712a26f99ef2`

## Starmap strict scan

The scan excluded `docs/reviews/**` from the report.

```text
Scanned files: 1,887
Affected files: 295
Diagnostics: 1,519
```

The five largest rule counts were 516 passive-voice findings, 389 semicolons,
219 long sentences, 104 contractions, and 101 nominalizations.

## Generated documentation marker matrix

The focused `unittest` command ran two tests. The negative case passed.
The generated marker matrix failed and exposed all eight generated fixtures.

```text
Ran 2 tests
FAILED (failures=1)
Unexpected discovered files: automatic.md, doxygen.html, javadoc.html,
jsdoc.html, mkdocs.html, rustdoc.html, sphinx.html, typedoc.html
```

The repository's documented `pytest` command could not run because the active
Python installation does not include pytest. The standard-library test runner
supplied the fail-before result.
