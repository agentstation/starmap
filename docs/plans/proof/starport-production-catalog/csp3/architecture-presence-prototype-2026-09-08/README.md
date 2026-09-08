# Architecture presence prototype

The candidate preserves missing, null, false, and true claims for `Quantized` and `FineTuned`.
It remains outside repository production source.
The [verification record](verification.json) binds every candidate variant, test, overlay, and result.

The original source fails twelve leaf cases across JSON, YAML, and definition views.
Its captures contain fourteen failing test results, including parent tests.
A codec-only candidate exposes four additional view failures for null and false.
Reconciliation then needs the same presence-aware selection.

The final candidate passes 81 test results and three package results on Go `1.26.6` and Go `1.25.12`.
Those suites cover codecs, read views, copies, definition equality, reconciliation, publication, restart, and source refresh.
Twelve runtime scenarios preserve original receipts and observation history.
No final suite reports a failure or skip.

The initial map encoder changes legacy field order and breaks both byte-identity cases.
The ordered encoder preserves both legacy byte sequences exactly.
The original encoder also passes those cases.
Stored observation checksums require this compatibility.
These two fragments do not qualify a complete stored-generation upgrade.

The equality method compares serialized facts and presence.
It resolves the existing comparison failure without ignoring the new private state.
The prototype does not cover other optional null records or every architecture property.
Source precedence, rejected receipts, state transitions, full package checks, and HTTP schema review remain open.

To reproduce a capture, extract its files and rewrite the two filesystem prefixes in its overlay.
The source prefix names the checked repository. The replacement prefix names the extracted candidate files.
Use `ordered-overlay.json` for the final candidate. Earlier overlays retain their own source variants.
