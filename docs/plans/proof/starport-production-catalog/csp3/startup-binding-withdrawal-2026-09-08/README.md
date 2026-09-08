# Startup after provider binding removal

The repair closes two startup paths that expose withdrawn provider scope.
It changes connected runtime selection and publication.
The offline library continues to read explicitly supplied stores without acquiring sources.

Without retained inputs or an explicit binding set, startup refuses stored scoped provider evidence.
The typed conflict returns no runtime and preserves accepted stored bytes.
An opaque generation ID cannot hide the binding scope recorded in provenance.
An unscoped store-only catalog remains available.

With retained inputs, startup rebuilds the permitted catalog after binding removal.
The runtime-owned client, HTTP endpoint, and durable current head now select that catalog before startup completes.
The repair restores an existing immutable baseline generation instead of rewriting its manifest.
A failed store commit prevents startup and releases the runtime directory for a later attempt.

## Behavioral evidence

The final regression file tests these outcomes:

- Refuse withdrawn scope under both derived and opaque generation IDs when retained inputs are absent.
- Align the runtime, client, HTTP provider endpoint, and stored head when retained inputs permit a rebuild.
- Apply an explicit empty binding set without retained inputs.
- Preserve an unscoped store-only catalog without retained inputs.
- Refuse failed startup publication, preserve the stored head, and permit recovery after the store accepts writes.

The final before capture restores both original production callers from ad2c09e3.
The new scope classifier remains unused in that capture.
It records two passing events and six failure events.
The failures include exposure of the withdrawn provider and HTTP status 200 instead of 404.

The final focused suite passes 31 events with no failures or skips.
The Go 1.25.12 suite passes eight events with no failures or skips.
Counts include test, subtest, parent, and package events.
The verification record separates those counts.

The full runtime, acquisition, and reconciler suite passes 1,007 test events and three package results.
It records no failures or skips.
The resumed session verified terminal package events after the process handle expired.

Ago reports zero findings, stale ignores, and errors.
Runtime package lint reports zero issues.
The runtime documentation generator completes without changing its generated README.

## Rejected legacy recovery candidate

The identity-only probe passes unchanged input, changed bindings, and changed baseline generation-ID cases.
It fails to detect changed baseline bytes and missing original history when it reuses manifest evidence.
Its three passing and four failing events include parent and package results.
This is a rejected recovery decision, not a changed production generation-ID contract.
Missing history alone does not invalidate every unscoped store-only catalog.
It cannot establish continuity when a recovery decision requires that evidence.

A source display identity also cannot prove authority continuity.
`fileSource.Identity` returns `file` for every configured path.
Legacy recovery must verify current authority, selected baseline content, and required input continuity before reusing accepted bytes.
The earlier legacy-provenance prototype remains unapplied.

## Capture validity

The first withdrawal probe uses the permitted default memory-only retention mode.
The expanded draft incorrectly supplied an explicit empty state-directory option in three scenarios.
That option fails configuration validation.
Those scenarios provide no behavior qualification.
The retained-input HTTP scenario in that draft exposes the real publication mismatch.

The corrected isolated captures use separate private temporary state directories.
Their four scenarios produce two passes and three failures on the original callers, including the package result.
All five candidate events pass.
The final regression adds opaque identities and failed-publication recovery.
All final behavioral captures use private temporary directories and in-memory catalog stores.
No provider API call or paid inference occurs.

## Remaining scope

This repair does not establish general legacy restart admission.
It adds no baseline or configuration fingerprint and changes no retained format.
Internal-source authority continuity, fleet-wide revocation, live configuration replacement, and diagnostic availability remain subject to their existing plan gates.
No native, released-pair, or CSP3 component acceptance follows from these local checks.

The [verification record](verification.json) binds the retained captures to their source bytes.
The common credential pattern scan reports zero matches across 32 captures.
This pattern scan is not a complete secret audit.
