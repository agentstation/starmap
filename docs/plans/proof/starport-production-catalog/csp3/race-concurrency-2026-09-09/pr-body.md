Catalog reconstruction can lose the distinction between missing, unknown, false, and zero values. It can also attribute retained facts to a newer source that did not supply them.

This change preserves descriptions, architecture flags, optional model records, and token-price units through reconciliation and runtime reconstruction. It retains their original source receipts and clears facts whose receipts no longer satisfy policy. Partial provider replies retain permitted prices while accepting independent limit updates. JSON, YAML, and generated OpenAPI properties preserve the supported presence states.

Legacy YAML empty price mappings retain their free-price meaning. New prices with no known units remain invalid. The embedded manifest, endpoint projection, and pinned artifact fixture match the corrected catalog bytes.

Validation:

- The preceding candidate passed all 39 repository verification stages, including 80 ordinary packages and 80 race packages.
- The repair passes 2,133 affected race test events across five packages. Both Go toolchains pass 110 focused events. All six consumers pass.
- The ordinary rerun passed 78 packages. The test compared new files with an old index. The local disk also filled. The failed contracts pass after staging the files and removing clean temporary checkouts.
- Ago and lint report zero findings. Documentation freshness passes. The final prose check passes 1,239 files.

The native runtime suite now uses the same twenty-minute package timeout as repository verification. Windows AMD64 reached the previous ten-minute deadline with tests still advancing. The thirty-minute job limit allows setup and artifact retention. All native packages and assertions remain in place.

The complete race suite now limits concurrent package processes to two, matching the passing local run. CI reached the root package deadline while parsing a full YAML workspace. The focused writable-store test passes with race detection in 193 seconds. All tests and the twenty-minute deadline remain. Fresh CI must verify the resource limit.

This PR covers field presence and receipt recovery within CSP3. Nested range presence, scoped inventory withdrawal, and the remaining CSP3 acceptance cases stay open. Task completion requires implementation merge and passing assigned acceptance checks.
