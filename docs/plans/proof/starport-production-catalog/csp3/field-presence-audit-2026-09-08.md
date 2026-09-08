# Model field presence audit

CSP3 remains incomplete. The limit repair preserves unknown values without a known fallback. Independent probes identify lost composite receipts and lost explicit null records. Scoped membership and removal review remain separate open requirements.

## Evidence scope

The audit examines `pkg/catalogs/model.go`, `model_presence_codec.go`, and `internal/catalog/reconciler/model_policy.go` at the recorded source revision. The authority policy remains in `pkg/catalogs/authority/authority.go`. [Probe captures](unknown-limits-2026-09-08/README.md) retain the executable tests and results.

| Surface | Current selection and representation | Evidence and remaining work |
| --- | --- | --- |
| Six numeric limits | Known values precede unknown values. Each selected dimension has separate provenance. | Runtime coverage includes positive, zero, unknown, and missing values through restart and source refresh. Unknown without fallback now retains its receipt. |
| Boolean capabilities and modalities | Per-capability and per-modality evidence preserves contributing receipts. | The capability repair covers ten recovery scenarios. Every published Boolean capability uses the same selector. This does not prove every combination of controls. |
| Description | A private presence state preserves explicit empty and null values. | The JSON null probe passes for description. A separate reconciler probe loses an unknown description when no known fallback exists. |
| Model metadata | Values merge across sources. One aggregate provenance entry identifies the leading source. | A partial reply supplies open weights. A complete reply supplies tags and omits open weights. The retained value survives, but its receipt disappears and degradation clears. |
| Modes | Named modes merge pricing and request overrides from fallback sources. One aggregate entry records the result. | The runtime probe retains an older header but drops its receipt. Mode pricing must remain an atomic commercial record. |
| Authors | The merger accumulates distinct author identities and records one aggregate entry. | Multi-source contributions need provenance and membership verification. Authorship must not change provider-serving identity. |
| Extensions | Namespaced fields merge by authority and retain one aggregate entry. | The runtime probe retains an older extension field but drops its receipt. Do not treat arbitrary extension keys as canonical facts. |
| Pricing | One validated commercial record wins atomically. | Existing recovery tests cover free, paid, omitted, and retained pricing. Whole-record null presence still collapses during JSON decoding and encoding. |
| Other optional records | Pointer fields omit nil values. | JSON round trips lose explicit nulls for limits, features, metadata, lineage, attachments, generation, reasoning, reasoning tokens, verbosity, tools, response, and lifecycle dates. |
| Identity, status, lineage, and timestamps | Each policy defines authority and empty-value behavior. | Preserve required identity and change-aware timestamps. The audit does not authorize empty identifiers or null timestamps. |

## Confirmed failures

The metadata probe records two failure events: its test and package. It verifies that the old open-weights value remains known while the old receipt and degraded state disappear. A successful complete observation must not retire evidence that still supplies accepted metadata.

The expanded probe covers metadata, mode headers, and extension fields. All three retain an older value but drop its receipt and clear degradation. The capture records four failure events, including the package. This change does not repair those cases.

The JSON probe checks fifteen fields. Description passes. Fourteen optional fields lose explicit nulls, producing sixteen failure events with the parent test and package. This is a codec result, not a deletion or authorization result.

Missing data makes no claim. Unknown data reports the absence of knowledge. An explicit null must not silently become permission to remove an offering. Preserve the original observation and apply the selected field policy during reconciliation.

The description probe records two failure events: its test and package. JSON retains the unknown description, but reconciliation converts it to missing when no known fallback exists. The field selector currently discards unknown description claims.

## Required next work

Repair composite provenance at each owning concept. Cover metadata, mode fields, authorship contributions, and extension fields through publication, restart, and later source refresh. Do not assign one receipt to a result that still depends on another observation.

Define and preserve whole-record presence through JSON, YAML, observation copies, and immutable payloads. Keep unknown claims separate from missing records and scoped tombstones. Verify compatibility before publishing a changed payload contract.

Repeat the existing A08 and Starmap A20 checks. The 50-case and 324-subcase roster remains unchanged. The [scope contract](scoped-membership-contract.md) still records pending global removal authority and default review policy.

The CSP3 component verifier selects ten subcases and reports each as unverified. The registry contains no behavior checks for those selections. After the implementation meets each contract, register its behavior check. Focused regression tests alone do not establish A08 or A20 completion.
