# CSP3: Scoped evidence reconciliation

CSP3 is in progress. CSP1 is complete locally, so this task can proceed independently of CSP2 qualification blockers.
The runtime now retains separate binding revisions and supports explicit active declaration sets.
Operator configuration, binding-aware batch acquisition, scoped field authority, and deletion remain incomplete.

The first implementation step validates retained provider evidence before storage or publication.
Before the correction, layer loading checked provider identifier syntax but did not bind the filename, payload provider, or payload digest.
The runtime also retained caller-owned payload bytes and could write a valid first layer before discovering an invalid later layer.
The required behavior rejects invalid evidence before any batch retention and preserves existing valid state.

This step does not establish complete source receipts or scoped reconciliation.
A08 and Starmap ingestion subcases of A20 remain UNVERIFIED until their full contracts pass.

## Provider evidence validation

`runtime/provider_evidence.go` validates the provider identity, bounded payload, required observation time, checksum, and decoded provider membership.
The payload must contain exactly one provider with the declared canonical identifier. An alias does not substitute for that identifier.
Retention copies and validates every supplied payload before it changes memory or writes any member of the batch.
Restart also verifies that each retained filename matches the provider identifier. Refusal leaves invalid files available for inspection.

The [final initial regression](csp3/provider-evidence-red-final.json) failed all 17 test events before the correction.
The [focused correction](csp3/provider-evidence-green.json) passed the same 17 events with the macOS race detector.
The [additional identity checks](csp3/provider-evidence-alias-check.json) cover aliases and payloads with multiple providers.

This is evidence-integrity validation, not complete source authentication or account-scope enforcement.
It does not make several durable writes transactional after an I/O failure. CSP5 owns publication and retention failure behavior.
Legacy provider records with missing or inconsistent evidence now cause startup refusal rather than silent acceptance.
Preserve those files before recovery. Do not manufacture a checksum for bytes whose origin is unknown.

The runtime still uses empty-field enrichment. Scoped field authority, presence, nulls, completeness, deletion, and retained source receipts remain open.
CSP3 remains in progress, and no primary acceptance case gains credit.

## Field-presence probe

The [pricing probe](csp3/provider-pricing-red.json) distinguishes a newer positive price from an explicit free price.
The positive-price case passes with the current merge implementation. The explicit-zero case fails.
It retains the baseline input price (`10`) instead of the observed zero.
The probe records one passing event and two failing events, including its parent test.
The probe runner preserved its source in evidence and removed the temporary test file after execution.

The existing `ModelPricing.Validate` contract distinguishes an absent cost pointer from a present zero-valued token-cost object.
The latter means free, including its YAML `{}` representation. An empty top-level pricing object still fails validation.
The current correction uses this existing distinction. Scoped observation metadata remains necessary for the wider authority and deletion contract.

## Acquisition fixture correction

The initial native run passed 261 runtime and 221 reconciler events. Acquisition passed 21 events and failed two.
The initial macOS run passed 271 runtime and 221 reconciler events. Acquisition had the same two failures.

The custom acquisition observer omitted `ProviderLayer.Digest`, although the production observer already supplies it.
The fixture now computes the digest from its actual payload. The implementation continues to reject missing or inconsistent digests.

## Settled verification

| Check | Result | Evidence |
| --- | --- | --- |
| macOS runtime and reconciler | 492 race events passed | [Initial package run](csp3/provider-evidence-verification.json). The acquisition failures remain recorded there. |
| macOS acquisition after fixture correction | 23 race events passed | [Fixture verification](csp3/provider-evidence-fixture-verification.json). All 117 recorded code and module inputs remained unchanged during this run. |
| Linux runtime and reconciler | 482 native events passed | [Native package records](csp3/provider-evidence-native-linux/summary.json). The acquisition failures remain recorded there. |
| Linux acquisition after fixture correction | 23 native events passed | [Final acquisition record](csp3/provider-evidence-native-acquisition-final/summary.json). |
| Static checks | Ago and whole-module lint passed | [Static results](csp3/provider-evidence-verification.json). Ago also passed after the fixture correction. |
| Windows builds | AMD64 and ARM64 runtime binaries compiled | [Build results](csp3/provider-evidence-windows-builds.json). Native execution remains UNVERIFIED. |
| CSP3 task gate | Ten selected subcases remain UNVERIFIED | [Task results](csp3/provider-evidence-fixture-verification.json). All 50 primary cases remain UNVERIFIED. |

The runner removed every Linux verification container and temporary volume after its run.
Native Linux checks use CGO-disabled binaries without race instrumentation. The macOS checks use Go 1.25.12 with the race detector.
The acquisition fixture correction changes no production behavior. The earlier runtime and reconciler input files remain unchanged.

The [store contract](../../../CATALOG_STORE_CONTRACT.md#retained-provider-evidence) now documents the custom-acquirer fields and restart refusal behavior.
CSP3 continues with field-presence metadata and scoped reconciliation. No commit, publication, or primary acceptance credit occurred.

## Explicit-zero pricing correction

The shared `MergeModels` operation now selects valid updated pricing as a complete object, including zero prices.
Missing or invalid updated pricing retains the existing object. The merge copies selected pricing before returning it.
This prevents mixed currencies, stale alternate units, and old price components from contaminating a new provider price.
It also prevents changes through the merged pricing object from mutating either input price.

The [regression run](csp3/pricing-presence-red.json) recorded nine failing and four passing test events before the correction.
The [focused correction](csp3/pricing-presence-green.json) passed all 13 events with the macOS race detector.
The checks cover JSON and YAML free-price presence, invalid observations, complete price replacement, input ownership, retention, and restart.

This change does not implement scope, timestamps, deletion receipts, or complete runtime source authority.
The generic merge validates pricing structure. The reconciler separately owns effective-time selection.
CSP3 still needs those contracts and its full acceptance evidence.

The [broader verification](csp3/pricing-presence-verification.json) passed 1,095 macOS race events on Go 1.25.12, with no failures or skips.
The package counts are 577 catalog, 274 runtime, 23 acquisition, and 221 reconciler events.
Ago and whole-module lint passed. All recorded code and module inputs remained unchanged during that run.

The first runner failed while parsing non-event JSON. Its [error record](csp3/pricing-verification-runner-error.json) grants no product verdict.
The corrected runner retained raw results before parsing them.

The CSP3 task gate still reports ten selected UNVERIFIED subcases and all 50 primary cases UNVERIFIED.
The owner decisions now permit reviewed commits and draft PR publication. No commit or publication occurred at this checkpoint.


## Provider observation order

Provider retention previously accepted an older observation over newer retained evidence.
Different payloads at an equal observation time also replaced each other according to arrival order.
The [final initial regression](csp3/provider-order-red-final.json) failed all 12 test events before the correction.

Retention now validates owned observations, orders each provider's batch by observation time, and selects its newest unambiguous observation.
A selected observation that predates retained evidence causes a typed conflict before any batch write.
Different payloads at the same observation time also cause a conflict, including conflicts against retained evidence.
An identical observation at the retained time requires no durable rewrite.

A separate retention mutex serializes selection and provider writes without holding the catalog read lock during filesystem writes.
The checks cover memory, durable retention, restart, reversed batch order, identical observations, and concurrent updates.
These changes do not make multi-file writes transactional after an I/O failure. CSP5 still owns that contract.

The [first correction attempt](csp3/provider-order-green.json) failed compilation because it used an unsupported field on `ConflictError`.
The [corrected focused run](csp3/provider-order-corrected.json) passed 12 race events and Ago.
An additional [retained-conflict regression](csp3/provider-order-retained-conflict-red.json) then failed three events.
A newer batch item hid a conflicting observation at the retained timestamp. Retention now checks that conflict before selecting the newest item.

The [broader verification](csp3/provider-order-verification.json) passed 1,110 macOS race events on Go (1.25.12).
Counts were 289 runtime, 23 acquisition, 221 reconciler, and 577 catalog events, with no failures or skips.
Ago and whole-module lint passed. All 275 recorded code and module inputs remained unchanged.
The [generated-document checks](csp3/provider-order-generated-documents.json) passed after API documentation regeneration.

The runtime still retains one record per provider. These time comparisons do not establish account scope, timestamp authenticity, or a clock-skew bound.
Field authority, completeness, deletion, and source revocation remain open. No primary acceptance credit or publication occurred.


### Cancellation during ordered retention

A [cancellation regression](csp3/provider-order-cancellation-red.json) canceled publication after its initial context check.
The operation returned cancellation but still changed provider evidence in memory and on disk.

Retention now receives the publication context. It checks cancellation before preparation and again after the retention lock becomes available.
Provider writes use the existing context-aware private-file publication path. This path checks cancellation immediately before replacing the durable record.
The [focused correction](csp3/provider-order-cancellation-green.json) passed the ordering and cancellation tests and Ago.

The [final package run](csp3/provider-order-final-verification.json) passed 1,111 race events after the context correction.
Counts were 290 runtime, 23 acquisition, 221 reconciler, and 577 catalog events, with no failures or skips.
Ago and whole-module lint passed. All 275 recorded code and module inputs remained unchanged.
The [final generated-document checks](csp3/provider-order-final-generated-documents.json) also passed.

This correction does not undo previously committed batch members. The multi-record failure contract remains with CSP5.


The [task gate](csp3/provider-order-task-final.json) still reports ten selected subcases and all 50 primary cases as UNVERIFIED.
The [document contract check](csp3/provider-order-documents-final.json) passed and preserved all 57 historical evidence files.
The writing gate still reports only three diagnostics in the two historical command-output files awaiting the owner decision.
CSP3 continues by carrying existing observation receipts through provider acquisition and retention, then applying account scope and completeness during reconciliation.


## Retained source observation receipts

The [initial receipt regression](csp3/provider-receipt-red.json) failed three test events.
The default provider observer discarded its source receipt. Restart also discarded an injected receipt and accepted a provider record without one.

`ProviderLayer` now requires a `sources.ObservationReceipt` alongside its payload, digest, provider identifier, and observation time.
`runtime.NewProviderLayer` constructs this record from an existing `sources.Observation`.
The default provider observer now uses this constructor. Custom observer fixtures construct receipts from their actual source observations.

The receipt retains source identity, revision, observation identity, completeness, status, record counts, and classified issue records.
It excludes original diagnostic messages. Those messages can contain transport details or credential material.
Restoration substitutes stable issue codes for messages and verifies the observation identity and payload checksum through the existing source validator.

Retention copies issue records as well as payload bytes. Restart verifies the receipt before accepting the provider layer.
Different receipt identities at the same observation time conflict even when the payload bytes match.
Partial receipts make acquisition health degraded without changing a successful provider request into a transport failure.

The [receipt contract checks](csp3/provider-receipt-contracts.json) cover metadata alteration, copied collections, restart, and degraded acquisition health.
The [size regression](csp3/provider-receipt-size-red.json) then reproduced a partial batch write when receipt metadata exceeded the retained-record limit.
The [size correction](csp3/provider-receipt-size-green.json) verifies the complete serialized record before any batch retention.
The 64 MiB limit includes base64 payload bytes and receipt metadata. It applies to memory-only retention too.

The [broader verification](csp3/provider-receipt-verification.json) passed 1,164 macOS race events on Go (1.25.12).
Counts were 296 runtime, 24 acquisition, 221 reconciler, 577 catalog, and 46 source events, with no failures or skips.
All 298 recorded code and module inputs remained unchanged. Ago and whole-module lint passed.
The pure-Go consumer gate passed with the approved dependency budgets and forbidden-dependency checks intact.

The [generated-document checks](csp3/provider-receipt-generated-documents.json) passed after API documentation regeneration.
Old provider files without receipts now require explicit recovery from verified source evidence. Preserve those files and do not invent missing observation metadata.
CSP18 still owns the complete upgrade and recovery procedure.

These checks establish receipt integrity, not source authentication or verified upstream coverage.
Account scope, credential-profile binding, field authority, scoped deletion, and effective-generation provenance remain open under CSP3 and its dependent tasks.
No primary acceptance credit, commit, or publication occurred.


## Unambiguous observation identity

The [identity regression](csp3/provider-receipt-identity-red.json) reproduced two different issue lists with the same legacy observation ID.
A NUL inside one issue subject encoded the boundary of another issue. Receipt restoration accepted the altered issue list.

New observations use `observation:v2:<sha256>` identities. Each field has a byte-length prefix, and the identity includes record and issue counts.
This format preserves opaque field bytes without ambiguous separators. Original diagnostic messages remain excluded.

Legacy IDs remain readable when their original hash matches and no identity field contains a NUL.
A fixture captured before the correction proves legacy receipt restoration. Ambiguous legacy evidence requires verified recovery under CSP18.
A validator that only supports legacy IDs cannot validate v2 receipts. CSP18 must also qualify downgrade procedures.

The [focused checks](csp3/provider-receipt-identity-green.json) passed the identity, receipt, retained-provider, and acquisition tests with the race detector.
Ago also passed. The tests cover changed issue boundaries, safe legacy restoration, ambiguous legacy refusal, and unknown identity versions.


The [expanded package run](csp3/provider-receipt-identity-verification.json) passed all 14 non-root packages, including 1,571 race events.
The root package reached the selected five-minute package timeout after 78 passing events. Its stack showed a catalog YAML decoder at timeout.

The aggregate command therefore failed. Its output collector also failed while parsing Ago output, before the lint command ran.
The root package requires a separate run with the repository's 20-minute race-test limit. No timeout result counts as package acceptance.


The [root rerun](csp3/provider-receipt-identity-final-verification.json) passed 92 race events with the repository's 20-minute limit.
The completed package runs therefore passed 1,663 events across 15 packages, with no failed or skipped events in those completed runs.
All 474 recorded code and module inputs remained unchanged during those checks. Ago passed.
Lint then required preallocated field capacity in the new identity encoder. This adjustment preserves the encoded fields and their order.


The [final encoder checks](csp3/provider-receipt-identity-preallocation-verification.json) passed 30 focused race events after the preallocation change.
Ago and whole-module lint passed. The recorded inputs remained unchanged during these checks.
The [generated-document checks](csp3/provider-receipt-identity-generated-documents.json) passed. No exported API changed during the preallocation correction.

The task gate still marks ten selected subcases and all 50 primary cases as UNVERIFIED.
The writing gate still reports three historical-output diagnostics awaiting the existing owner decision. No commit or publication occurred.


## Credentials within concurrent observations

The [credential regression](csp3/credential-run-red.json) reproduced a second observation replacing the first observation's selected credential profile.
The first observation completed preflight with one profile, then sent its request with the second profile.
The source stored one shared memo and cleared it at the start of every observation.

The source now creates a credential memo for each observation. Preflight and fetch share that memo through the existing public fetcher's per-call options.
The source retains the process resolver for credential caching and renewal. It retains no shared observation memo.
An initial correction violated the existing fetch-policy ownership guard. The [corrected package checks](csp3/credential-run-corrected-green.json) preserve that guard and pass Ago and code lint.

The concurrency test controls the interval between preflight and fetch without a timing delay.
Additional cases check that absent or invalid credentials in a second observation cannot affect the first observation's successful request.
These tests use injected resolvers and clients. No provider protocol client changed. These checks require no live provider calls.

The selected profile and material version do not identify an account scope.
CSP3 still requires a deployment-owned acquisition binding and its revision, selectors, field authority, and retained-evidence policy.
This correction is a prerequisite for binding source receipts to the credentials that produced them. It does not complete that binding.


The [final package checks](csp3/credential-run-verification.json) passed 232 race events on Go (1.25.12), with no failures or skips.
Counts were 26 provider-source, 53 public-source, 67 authentication, 24 acquisition, and 62 pipeline events.
All 82 recorded code and module inputs remained unchanged. Ago and whole-module code lint passed.
The [generated-document checks](csp3/credential-run-generated-documents.json) also passed.

Named acquisition bindings are the next implementation step. No primary acceptance credit, commit, or publication occurred.
The pending historical-output lint decision still blocks publication.


## Typed provider acquisition bindings

The [binding regression](csp3/provider-binding-red.json) failed two events.
Observation and receipt decoding silently discarded injected binding metadata, then accepted the original unscoped identity.

The version 1 contract declares a binding identity and revision for one canonical provider.
It also declares public or account/project scope, region, API surface, and credential role/profile.
Selectors permit at most 4,096 UTF-8 bytes and reject control characters or surrounding whitespace.
Only the catalog-acquisition role is valid. JSON decoding rejects unknown fields, unsupported schemas, invalid field types, and trailing data.
Invalid decoding preserves the receiver and reports field errors without their values.

Scoped observations use version 3 identities that include every binding field. Unscoped observations retain version 2 and checked legacy reads.
The validator requires a provider source with exactly one provider that matches the binding.
Adding, changing, or removing binding metadata invalidates the existing identity.
The constructor and receipt operations copy binding values at each mutable boundary.

The [focused contracts](csp3/provider-binding-contracts.json) cover typed validation, ownership, version separation, schema checks, and retained restart integrity.
Ago and code lint passed. The tests also prove that old identity formats cannot validate scoped observations.

Binding validation checks the declared fields. It does not authenticate account ownership.
The caller still needs to verify the credential profile used for acquisition.
Acquisition selection, active binding revisions, retention keyed by binding, field authority, and scoped deletion remain open under the CSP3 task.

The existing runtime still retains one layer per provider. No primary acceptance credit or operator-support claim follows from these checks.


The [broader checks](csp3/provider-binding-verification.json) passed 723 race events, with no failures or skips.
Counts were 93 source, 297 runtime, 24 acquisition, 221 reconciler, 62 pipeline, and 26 provider-source events.
All 175 recorded code and module inputs remained unchanged. Ago and code lint passed.
The [generated-document checks](csp3/provider-binding-generated-documents.json) passed after API regeneration.

The final constructor check validates bindings before computing their identity, which avoids encoding invalid selectors.
The [final checks](csp3/provider-binding-constructor-verification.json) passed 60 focused race events, Ago, and code lint after that change.
All 20 recorded source inputs remained unchanged during those checks.

Next, connect declared bindings to acquisition selection and runtime scope enforcement, then retain independent scopes without provider-level replacement.
The full task and primary acceptance cases remain UNVERIFIED. No commit or publication occurred.

## Local Starport pair check

The [local pair check](csp3/provider-binding-starport-pair.json) ran Starport against this Starmap worktree through a temporary module file.
The command used Go (1.26.5), the race detector, and `TestStarmapAcquisitionPublishesRefresh`.
One test failed because Starport's injected observer returned a provider layer without the required observation receipt.
The error identified `provider_layer.receipt`. All three recorded Starport input files remained unchanged.

Starport currently pins Starmap `v0.16.5`. The distribution verifier runs Starport tests against that pin without a local replacement.
Those tests do not qualify this candidate Starmap API. The local check used injected credentials and a test observer without live provider requests.

CSP8 owns the fixture correction and dependency adoption. Its observer must construct a source observation and retain the resulting receipt through `runtime.NewProviderLayer`.
The test must continue to prove that acquisition publishes a changed generation. A compatible published module must precede the dependent Starport merge.
Temporary module replacements remain local verification tools and must not enter committed module files.

Starport's task index now matches the canonical ledger. Its strict writing check passed for one file with no diagnostics.
The Starport diff check passed. No product acceptance credit, commit, publication, or module-pin change occurred.

## Acquisition profile selection for bindings

The [initial test build](csp3/binding-acquisition-test-build-error.jsonl) failed because the new test omitted the receipt method's error result.
After that correction, the [baseline check](csp3/binding-acquisition-red.jsonl) failed ten test events because the source lacked an explicit binding operation.
The first implementation passed those ten events.

`Source.ObserveBinding` now selects one provider and validates its declared acquisition profile before credential resolution.
It restricts the resolver to that profile and checks the returned profile before client creation.
The source copy and observation memo isolate each concurrent call. The observation and receipt retain the declared binding.

The first concurrency fixture declared two unauthenticated alternatives, which the existing catalog validator forbids.
The [initial contract run](csp3/binding-acquisition-contracts.jsonl) records that failure. The corrected fixture uses two API-key profiles with a synthetic credential.
The [corrected checks](csp3/binding-acquisition-corrected.jsonl) pass provider selection, invalid configuration, profile mismatch, cancellation, concurrent profiles, and receipt retention.

`Acquirer.ObserveProviderBinding` exposes explicit observation without retention or publication.
The default observer constructs the retained layer through `runtime.NewProviderLayer`, which preserves partial-source evidence.
Custom observers must implement the binding contract. The acquirer rejects a receipt that names a different binding.

These calls do not authenticate upstream account ownership or verify region and API-surface coverage.
Scheduled acquisition, active binding revisions, multiple retained scopes, and scoped deletion remain open. No operator-support or primary acceptance credit follows from this component.

The [final verification](csp3/binding-acquisition-verification.json) passed 287 race events across five packages, with no failures or skips.
Counts were 38 provider-source, 27 acquisition, 93 public-source, 67 authentication, and 62 pipeline events.
All 71 recorded code and module inputs stayed unchanged. Ago passed.

The `make lint` command passed code lint with zero issues and passed Ago.
Its final writing step failed on the three unchanged historical-output diagnostics. That aggregate command therefore remains failed.
The [corrected document checks](csp3/binding-acquisition-corrected-documents.json) also report only those historical diagnostics.
`make godoc` and `make docs-check` passed in exec session `6715`. The diff check passed.

The [task gate](csp3/binding-acquisition-task.json) still marks all 50 primary cases UNVERIFIED.
Next, connect explicit binding calls to scheduled acquisition, enforce active revisions, and retain independent scopes.
No commit, publication, operator-support claim, or primary acceptance credit occurred.

## Separate provider scope retention

The [initial regression](csp3/provider-scope-retention-red.jsonl) failed four test events.
Two scopes of one provider competed for the same memory entry and file. The runtime also accepted changed selectors under an existing revision.

Retention now keys records by provider, binding identity, and revision. Scoped records use the private `providers/bindings` directory and hashed filenames.
Unscoped records retain their provider filenames. They remain distinct from explicit public-scope declarations.
The filename hash includes byte-length fields, so field separators cannot create ambiguous identities. Raw selectors never become paths.

The [first build](csp3/provider-scope-retention-first-build.jsonl) identified existing tests that still indexed the former map key.
Those tests now use the evidence key and preserve their assertions. The receipt-tampering test uses the new scoped file location.
The [initial correction](csp3/provider-scope-retention-green.jsonl) passed five focused events.

The [selector regression](csp3/provider-scope-selector-red.jsonl) then proved that a changed provider could reuse a binding revision.
Validation now checks the complete retained set and incoming batch before writes. Each binding identity and revision must name one declaration across providers.
Restart applies the same check after validating each record's receipt and filename.

Tests cover memory and disk retention, restart, concurrent scopes, revision separation, unscoped coexistence, selector changes, filename boundaries, and invalid storage locations.
This component does not select active revisions, revoke old evidence, or enforce field authority.
Scoped records from the former provider-only location require explicit migration or verified reacquisition under CSP18.

Source inspection also found that `initializeEffective` takes its baseline from `client.CurrentCatalogState()`.
If that state contains previously merged provider evidence, filtering retained files alone might not remove revoked records.
CSP3 must verify baseline provenance and revocation after restart before qualifying active scope policy. This is a source-inspection risk, not completed behavioral proof.

The [corrected focused checks](csp3/provider-scope-retention-corrected.jsonl) passed after the cross-provider revision check.
The [broader verification](csp3/provider-scope-retention-verification.json) passed 508 race events, with no failures or skips.
Counts were 316 runtime, 27 acquisition, 21 server, 93 source, and 51 private-file events.
All 157 recorded code and module inputs stayed unchanged. Ago and code lint passed.

`make godoc` and `make docs-check` passed in exec session `10222`.
The first document gate caught an edit to the frozen storage review. The worktree now contains the original file bytes.
The [restored structure check](csp3/provider-scope-retention-restored-document-structure.json) passed with all 57 historical files preserved.
The current store contract and engineering specification contain the new scoped path. The historical storage review remains unchanged.

The aggregate lint command failed on one new prose diagnostic and the three existing historical-output diagnostics.
The maintained sentence now uses a direct verb. The historical-output decision still controls publication.
No task completion, primary acceptance credit, commit, or publication occurred.

## Baseline provenance after durable publication

The [restart regression](csp3/baseline-provenance-red.jsonl) failed one test with two related errors.
After reopening a real filesystem catalog store, the runtime's embedded baseline contained a provider added by the previous runtime.
Excluding that provider's retained layer and rebuilding still returned the provider through the stored baseline.

`Client.EmbeddedCatalogState` now exposes the compiled immutable catalog that construction already verifies.
The getter returns its original generation identity, checksum, and timestamp independently of later client updates or stored current generations.
The runtime uses this separate state as its reconstruction baseline. With no retained inputs, startup still preserves the accepted current state.

The [initial correction](csp3/baseline-provenance-green.jsonl) passed the root accessor test and the restart regression.
The accessor test uses a real memory store and counts reads through its adapter.
It verifies separate stored and embedded catalogs, unchanged identity, shared immutable data, zero getter allocations, and no extra storage reads.
These measurements cover this accessor alone. Gateway latency qualification remains separate.

The [opaque-identity regression](csp3/baseline-identity-red.jsonl) proved that suffix parsing truncated a valid baseline identity containing `.local.`.
Reconstruction now derives from the complete baseline identity. It no longer parses a committed identity to infer its origin.
The opaque-identity regression replaces the obsolete suffix-helper test. Existing durable restart and commit-count tests remain in place.

The [focused contracts](csp3/baseline-provenance-contracts.jsonl) passed after both corrections.
Active revision selection, operator revocation, and accepted-head admission after a policy change remain open.
Explicit local inputs need their own source evidence during reconstruction. This component does not replace that requirement with cached merged output.

The [broader verification](csp3/baseline-provenance-verification.json) passed 458 race events across four packages, with no failures or skips.
Counts were 93 root-library, 317 runtime, 27 acquisition, and 21 server events. All 155 recorded inputs stayed unchanged during that run.
Code lint and Ago passed. Aggregate lint failed on one test-comment term and the three historical-output diagnostics.

An edit to the test comment removed the restricted term. The [final focused checks](csp3/baseline-provenance-final-focused.json) then passed three race events.
All nine recorded inputs stayed unchanged during that check. No behavior changed after the broader run.

The [external consumer gate](csp3/baseline-provenance-consumers.json) passed all six consumer compositions.
The read-only and pinned-artifact budgets remained 37 and 38 non-standard packages on macOS. Forbidden dependency families remained absent.
`make godoc` and `make docs-check` passed in exec session `69033`.

CSP3 remains in progress. Active binding policy must still decide whether an accepted head is valid after configuration changes.
Scheduled acquisition, explicit local-source evidence, scoped field authority, and deletion remain open. No primary acceptance credit, commit, or publication occurred.

## Provider refresh windows

The [regression](csp3/provider-window-red.jsonl) recorded nine failed and one passed test events.
An early publication suppressed final evidence from another account, another revision, or an unscoped observation for the same provider.
It also suppressed a newer observation within the same binding revision and accepted a forged final receipt without validation.
The aggregate retained-provider report omitted an unanswered peer scope after another scope succeeded.

The runtime now records each published observation within its provider scope revision.
It validates and copies final evidence before duplicate checks. Repeated valid evidence causes no second final publication.
A failed final validation preserves an earlier successful window and returns an error.
The aggregate retained-provider report includes any provider with an unanswered retained scope.

The [focused correction](csp3/provider-window-green.jsonl) passed all ten race events on Go 1.25.12.
The tests read the retained files after each run. They cover independent scopes, revisions, unscoped evidence, newer observations, duplicates, forged receipts, and partial failure.
An initial type-name error remains in the [build record](csp3/provider-window-build-error.jsonl). It grants no test credit.

The [static checks](csp3/provider-window-static.json) passed Ago and code lint.
The aggregate lint command still fails on the three unchanged historical-output diagnostics.
Generated Go documents and their freshness check passed. The existing historical-output decision still controls publication.

This component does not implement active revision admission, operator revocation, binding-level attempt reports, or scheduled binding selection.
CSP3 remains in progress. No primary acceptance credit, commit, or publication occurred.

The [broader verification](csp3/provider-window-verification.json) passed 375 race events on Go 1.25.12, with no failures or skips.
Counts were 327 runtime, 27 acquisition, and 21 server events. All 117 recorded code and module inputs stayed unchanged during the run.
The final document edit separates a long specification paragraph. It changes no Go code or test input.

## Explicit active binding policy

The [initial policy regression](csp3/provider-policy-red.jsonl) recorded ten failed and three passed test events.
Inactive records returned through retained layers and through a merged current generation with missing evidence files.
Publication also accepted undeclared or old revisions. Startup accepted changed selectors under a retained revision.

`runtime.WithProviderBindings` now supplies a copied complete active declaration set. Each binding identity permits exactly one active declaration.
The runtime excludes inactive records from reconstruction and startup freshness without deleting those retained records.
It rejects inactive publication batches before writes. Changed selectors require a new revision, including during restart.
The [first correction](csp3/provider-policy-green.jsonl) passed thirteen race events.

The [report regression](csp3/provider-policy-report-red.jsonl) found that a later policy refusal hid an earlier successful publication in the whole-refresh report.
The refresh now records successful windows before final validation or publication can fail.
The existing window tests now declare active scopes. They require refusal for an inactive revision or unscoped final record.
The [prior window source](csp3/provider-policy-prior-window-test.json) preserves the preceding component test before this policy change.

The [HTTP regression](csp3/provider-policy-http-red.jsonl) proved that filtering runtime state alone left the underlying client and HTTP handler on the previous catalog.
Explicit-policy startup now aligns the runtime, client, and current store generation before returning.
The [HTTP correction](csp3/provider-policy-http-green.jsonl) passed against a filesystem store and real handler.

An explicit set gets a writable memory store unless the caller supplies another store.
The selected identity hashes the complete active declarations, original source identity, and payload checksum.
Explicitly restoring a prior selection can reactivate its retained immutable generation. The identity alone grants no permission or source authority.
A failed startup publication refuses the runtime and releases its directory.

The runtime requires `BindingAcquirer` for an explicit nonempty set. An empty set makes no provider calls.
The built-in batch acquirer still lacks that role and cannot fall back to unscoped provider I/O in this mode.
Single-binding observation remains available as an explicit operation without publication.

The [expanded contracts](csp3/provider-policy-contracts.jsonl) passed all 32 race events with the Go 1.25.12 toolchain.
These cover declaration ownership, duplicate refusal, inactive evidence, restart, and HTTP state.
They also cover immutable reactivation, failure cleanup, acquisition refusal, and refresh windows.
The subsequent Go edits only group imports and replace one passive comment. Broader verification uses those final files.

The [static checks](csp3/provider-policy-static.json) passed code lint, Ago, and generated-document checks.
The writing check found one new comment diagnostic, one long specification paragraph, and the three unchanged historical-output diagnostics.
The comment and paragraph now use the required form. Final document verification remains pending.

This is an explicit Go API contract, not qualified operator support.
Legacy construction without the option still permits unscoped behavior. CLI configuration, batch acquisition, and binding-level attempt reports remain incomplete.
A separately injected server `Syncer` can bypass the runtime policy. Server mutation must use the same active declarations during integration.

CSP4 owns configuration-omission guards, internal authority, and complete accepted-head admission. CSP18 owns receipt migration and downgrade qualification.
CSP3 remains in progress, with no primary acceptance credit, commit, or publication.

The [broader verification](csp3/provider-policy-verification.json) passed 397 race events, with no failures or skips.
Counts were 349 runtime, 27 acquisition, and 21 server events. All recorded code and module inputs stayed unchanged during that run.

A final [addressed-identity regression](csp3/provider-policy-identity-red.jsonl) failed one test after the broad run.
A store returned valid embedded bytes under a different requested identity. Startup accepted that result and published it.

The restore path now requires both the selected identity and checksum before activation. It still validates the complete generation through `Client.Activate`.
Focused publication verification covers this final guard. The broader record describes the code before the guard.

The [final publication tests](csp3/provider-policy-publication-final.json) passed all four race events after the addressed-identity guard.
Their recorded code and module inputs stayed unchanged. These tests cover HTTP state, immutable restoration, startup failure, and mismatched addressed reads.

The historical-output approval request came from an overly broad reading of the agent-rule instruction.
This correction changes file classification under the existing historical-review policy. It adds no writing rule and changes no severity.
The [classification record](csp3/historical-output-classification.json) preserves both exact paths and unchanged historical hashes.
Only those two frozen command-output files now join the existing exclusions. Maintained plan prose remains subject to strict checks.

The [final writing check](csp3/provider-policy-writing-final.json) passed all 1,047 scanned files with zero diagnostics.
Both frozen-output hashes remain unchanged. Full repository verification now precedes the authorized publication and native CI sequence.

The [first complete repository run](csp3/publication-repository-verification.json) failed on a stale workflow assertion after 111 seconds, with unchanged inputs.
The assertion expected obsolete `CATALOG_PATH` variables. The script already uses isolated canonical `STARMAP_*` settings and a temporary working directory.
The workflow test now requires that current contract, including embedded source selection, disabled acquisition, and an empty workspace path.
It retains the clean-environment, private-home, and pinned-linter checks. The verification script itself remains unchanged.

The [workflow package correction](csp3/publication-workflow-contracts.jsonl) passed all 26 race events on the release toolchain. Ago passed after the test change.
The complete repository gate must rerun before the authorized commit and review sequence.


## Publication verification checkpoint

The [repository rerun](csp3/publication-repository-verification-rerun.json) completed in 885.533 seconds with unchanged recorded inputs.
Its [log](csp3/publication-repository-verification-rerun.log) records 79 passing race-test packages, container smoke success, and all coverage thresholds.
The final writing gate found one seven-sentence paragraph in the current findings document.
A paragraph split corrected it without changing facts.
The [writing correction](csp3/publication-writing-correction.log) passed 1,047 files with zero diagnostics.
The complete final repository run remains active in `csp3/publication-repository-verification-final.json`.

Starport commit `481e71f` records the prepared documentation, UI, measurement fixture, and native verifier.
Commit `fabdc2b` integrates main at `25c1196` and preserves its responsive console changes.
The merge keeps readable settings text and the new container-query layout.
Two compact-layout tests now start on protected routes because `/docs` uses the public layout.
Their navigation assertions remain intact.

The [required Starport checks](csp3/publication-starport-required.json) passed 25 commands against the pinned Starmap module.
The earlier [repository checks](csp3/publication-starport-checks.json) cover the other eight pre-PR commands through `make verify` and `make lint`.
The [Go race check](csp3/publication-starport-go.json) passed both affected packages.
The [native verifier tests](csp3/publication-starport-ui-2.log) passed eight tests.

The [combined UI check](csp3/publication-starport-merged-final.json) passed type checking, all 457 tests across 74 files, the console build, and the UI policy check.
The [focused integration check](csp3/publication-starport-merged-focused.log) passed 16 tests.
The initial unrestricted console run failed five tests under concurrent test load.
A bounded run passed all 444 pre-merge tests without assertion or timeout changes.

The [writing attribution](csp3/publication-starport-writing-attribution.json) records 96 diagnostics on unchanged lines in `DESIGN.md` and `FirstContact.tsx`.
The remaining eight checked files passed without diagnostics.
This checkpoint does not claim whole-file writing conformance for those two existing files.
The earlier stdin comparison used the wrong TSX parser and does not establish attribution.

The required isolated Sol and Opus review is active for the committed Starport branch.
Its pending output paths are `/tmp/starport-catalog-publication-review.json` and `/tmp/starport-catalog-publication-review.log`.
No draft PR or hosted native run exists yet. CSP3 and all primary acceptance cases remain open.


### Published Starport draft

The [isolated review](csp3/publication-starport-review.json) returned zero findings from Sol high and Opus high at the configured pre-PR threshold.
The helper scanned secrets and reviewed the complete 94,304-byte bundle in one pass.
[Draft PR #366](https://github.com/agentstation/starport/pull/366) publishes commit `fabdc2b`.
GitHub rejected the first push because the inherited upstream mapping targeted `main`.
The explicit task-branch refspec then succeeded without changing `main`.

[Hosted native verification](csp0.2/native-run-34084336162-review.json) passed all six archive jobs.
The records bind to the PR test merge and report 511 models on every platform.
The [E01 revalidation](csp3/publication-docs-revalidation.json) passes nine behavior checks but rejects stale browser input hashes after the main integration.
CSP0.1 returns to `todo` for visual review. CSP0.2 returns to `todo` for E02 registration and installation-method review.

The [final Starmap run](csp3/publication-repository-verification-final.json) failed the race test `TestCloseRejectsLateSourcePublication`.
A deterministic regression now targets cancellation after the source read and before retention.
Starmap publication waits for this correction and verification. CSP3 remains the sole active task.


### Source-retention cancellation correction

The [deterministic regression](csp3/source-retention-cancellation-red.log) cancels the operation after its source read and before retention.
The old write ignored that cancellation and changed both durable and active source state.
Source retention now passes the operation context to private-file publication.
Runtime shutdown closes the run group before canceling the runtime context.

The [repeated race check](csp3/source-retention-cancellation-green.log) passed three named tests across 20 repetitions.
It covers late source results, late provider results, and cancellation between source reading and retention.
[Ago](csp3/source-retention-cancellation-ago.json) passed after the correction.
The complete repository run is active in `csp3/publication-repository-verification-source-fix.json`.


The [source-fix repository run](csp3/publication-repository-verification-source-fix.json) passed the full race suite but found an unused layer-write wrapper.
The context-aware source writer removed its last caller. The wrapper is now absent.
The pinned linter must pass before the final repository rerun.
CSP0.1 returned to `done` after renewed E01 browser evidence passed.


### Final publication checks

The pinned golangci-lint 2.12.2 check passed with zero issues.
The fifth repository run then found five test fixtures that still called the removed wrapper.
Those fixtures now pass their test context to the context-aware writer.
The [focused race check](csp3/source-retention-fixtures-check.log) passed in 238.629 seconds.
The sixth complete repository check follows this correction.

The task branch for draft PR #366 now contains Starport commit `5820a370ff9255a8148799f5b45fcaf927c2e1dc`.
Both later documentation commits passed isolated pre-PR review with zero findings.
The final review used Sol high and Opus high on the complete 94,345-byte bundle.


### Complete Starmap verification

The [sixth repository run](csp3/publication-repository-verification-attempt-6.json) passed in 672.271 seconds with unchanged inputs.
The normal and race suites each passed 79 packages.
The run also passed consumer dependencies, 30 verifier tests, container startup, pinned lint, Ago, and 15 coverage checks.
Generated documentation, strict writing, diff checks, and four isolated CLI commands passed.
The catalog performance gate passed three runs with zero bytes and zero allocations per lookup.

The review helper refuses binary changes because it cannot review their contents.
Starmap publication therefore uses an evidence base branch named `codex/catalog-evidence`.
That branch contains captured JSON, logs, screenshots, and profiles.
It excludes executable proof scripts and the three authored acceptance or profile contracts.
The code branch remains `codex/catalog-requirements` and targets that evidence branch for review.
A manual dispatch of the existing PR workflow supplies native CI for the code branch.

This mechanical split preserves every artifact and the complete code diff.
Both pull requests remain drafts. It changes no merge or release authority.


The evidence commit is `a2f390af3c43ecf91edc073236f5d0b0e5f8b345`.
It contains exactly 1,100 captured files, including the previously ignored verification logs.

The [credential scan](csp3/publication-evidence-scan.json) reported seven copies of one synthetic URL in historical test output.
Every report matches the eight-character placeholder in `TestCredentialEndpointBindingsRejectUnsafeURLValues`.
The scanner could not resolve that test hostname. The review confirmed that these reports contain no live credential.
The scan result remains recorded as exit 183.


Commit `5c6fa71a` adds the two historical-output exclusions to the evidence base.
The existing writing policy covers this classification, and the frozen hashes remain unchanged.
The evidence worktree is `/Users/jack/src/github.com/agentstation/starmap-catalog-evidence`.
Its branch is `codex/catalog-evidence`. It contains no substantive code for the automatic pre-PR gate.


### Review input split

The first isolated code review did not complete.
Its credential scan passed, and the helper divided the 2,669,359-byte bundle into seven passes.
During the third pass, Opus returned “Prompt is too long.”
The failed run grants no pre-PR approval.
The original code commit remains available locally as `codex/catalog-verified-snapshot` at `2699dafe`.

The planning documents now use a separate base branch, `codex/catalog-plan`.
Its worktree is `/Users/jack/src/github.com/agentstation/starmap-catalog-plan`.
That draft targets the evidence branch from PR #125. The code draft targets the planning branch.
The code review retains every changed source file, executable proof, generated API document, and settings schema.
The three authored acceptance and performance JSON contracts belong with the planning documents.
No production code changed during this split.


### Publication review and container correction

Planning draft [PR #126](https://github.com/agentstation/starmap/pull/126) targets the [evidence branch](https://github.com/agentstation/starmap/pull/125).
Its Sol and Opus review passed all three portions of the 998,491-byte bundle with zero findings.
Starport draft PR #366 passes all 16 current checks, including six native archive jobs.

The [two further code review attempts](csp3/publication-review-context-retries.json) each stopped with “Prompt is too long.”
Both ordinary Opus and the requested extended-context model reported a 200,000-token context window.
Neither attempt grants code publication approval.
The [prepared replay](csp3/review-smaller-prompts-replay.json) covers the complete code bundle in eight smaller prompts.
The proposed shared-helper change lowers only the prompt ceiling from 512,000 to 300,000 bytes.
The helper remains unchanged pending owner approval.

Evidence PR #125 passed five CI checks and failed container startup because its baseline image digest was unavailable.
Commit `ca0645be` updates the release, smoke script, and test pins to the same current digest.
[Signature verification and focused checks](csp3/container-base-refresh.json) passed for the replacement image.
The check uses the identity from [Chainguard's provenance instructions](https://images.chainguard.dev/directory/image/static/provenance).
All 26 workflow race tests passed, and Ago reported no findings or errors.
The local container served health requests with a read-only root and user 65532.

This correction remains on the unpublished code branch.
It does not change the failed evidence PR result or qualify native Starmap behavior.
After the shared-helper decision, complete code review before the branch push, draft PR, and native workflow dispatch.
CSP3 remains active, with batch acquisition and operator integration still open.


### Binding-aware batch acquisition

The built-in acquirer now implements `runtime.BindingAcquirer`.
The [initial regression](csp3/binding-batch-red.log) failed because that role was absent.
The batch validates all selected declarations before credential resolution or provider I/O.
Empty declarations select nothing, duplicate identities cause refusal, and provider filters cannot introduce undeclared bindings.

A shared loop records each target separately, including multiple bindings for one provider.
Attempt results and source sinks carry `BindingID` and `BindingRevision`.
The tests cover distinct concurrent credential profiles, partial failures, skipped credentials, early publication, cancellation, and late results.
Callback payload and receipt copies protect the retained result from caller changes.
A real runtime test verifies separate attempts, peer retention after failure, and both scopes after restart.

The [runtime regression](csp3/binding-batch-runtime-red.jsonl) found that enrichment dropped models without pricing or limits.
The [model regression](csp3/model-membership-red.log) isolates that membership filter.
The catalog merge now retains these records and their missing authored definitions.
Existing definitions keep precedence. The runtime still refuses offerings without a resolved canonical reference.

[Final verification](csp3/binding-batch-verification.json) passed 1,341 race events across six packages under Go 1.26.6.
All 79 tested packages passed the normal suite. Another 22 packages contain no tests.
All six external consumer compositions passed, with the read-only closure at 37/37 and pinned-artifact closure at 38/38 on macOS.
Code lint, Ago, and documentation generation passed.

The first broader run used the host's Go 1.27.0 default.
It recorded two failures and a runtime timeout. Its output remains separate from the passing pinned-toolchain run.
The corrected run kept the same five-minute package limit.
The credential-profile fixture now uses two API-key profiles because the contract forbids two unauthenticated alternatives.

The product verifier still reports zero primary passes and 50 UNVERIFIED cases.
CSP3 remains active for operator integration and the remaining scope and field-authority contracts.
Code publication still waits for required review. The proposed shared-helper change remains unapplied.
Raw results use the existing evidence branch before code review, so captured output does not enlarge the code review bundle.


The work commit is `497fd923`. Evidence commit `251378b9` preserves thirteen captured files, totaling 3,589,200 bytes.
Local planning merge `adc63446` supplies that evidence as the code review base.

All captured-file diffs against this base are empty. The integration preserved every recorded Go input hash.
Strict writing passed 1,053 files with zero diagnostics. The document validator still reports 38 tasks and 324 required subcases.
These branch updates remain local pending their required publication reviews.


### Shared provider binding settings

Commit `8a27bf91` adds `STARMAP_CATALOG_PROVIDER_BINDINGS` to the shared configuration contract.
The CLI flag accepts a JSON array, and the primary YAML file accepts binding objects as a list.
The parser rejects null values, unknown fields, invalid declarations, and duplicate binding IDs without exposing credential values.
An explicit empty array selects no connected-runtime provider acquisition. Omission retains the existing unscoped behavior.

A higher-priority array replaces the complete lower array. Changing the upstream catalog source leaves this independent policy in place.
The descriptor records deployment ownership and a required restart.
The generated settings reference, Compose comments, and environment example document these rules and the current manual-update limitation.

[Verification](csp3/binding-settings-verification.json) passed 269 race events across three packages and all 79 normal package suites.
Another 22 packages have no tests. Code lint, Ago, generated references, and strict writing passed.
The first package run found missing deployment examples. The first writing run found four prose diagnostics.
Both failures and their corrected results remain in twelve captures, totaling 543,181 bytes, on evidence commit `f995eb7a`.

Manual update integration remains open.
The standalone CLI update constructs `acquisition.Syncer` without the connected runtime.
The server update adapter also delegates to that standalone syncer, which does not enforce the runtime binding policy.
The correction must preserve dry-run previews, explicit source selection, failure retention, and workspace projection.
These settings are not evidence that those paths enforce scoped policy.

CSP3 stays in progress. All 50 primary cases remain UNVERIFIED.
The proposed shared-review-helper change remains unapplied, and these branch updates remain local.


### Scoped records in reconciliation

Commit `14233354` corrects source-type maps that discarded peer catalogs before reconciliation.
The [first regression](csp3/scoped-reconciliation-red.log) reproduced a lost offering. It also showed that input order selected an older shared model.
Collection now preserves scoped observations, and primary-source filtering includes every selected provider.

Each selected provider or model record retains its original observation and health classification.
Direct observations precede stale fallback. Observation time orders records within that classification.
Records with the same identity, time, and classification must agree. Identical records select a receipt deterministically.
The [health regression](csp3/scoped-reconciliation-health-red.log) proved that recency alone let a stale fallback displace a direct peer observation.

Field provenance and review candidates now use the selected record's receipt.
Counts report unique provider model IDs and source types.
Tests cover peer membership, shared records, separate providers, receipt integrity, equal-time conflicts, health isolation, and cancellation before validation.
The existing field-authority table still determines precedence between source types.

[Final passing package runs](csp3/scoped-reconciliation-verification.json) total 348 race events: 234 reconciler, 62 pipeline, and 52 acquisition events.
All 79 normal package suites passed, with another 22 packages containing no tests.
Lint, Ago, documentation generation, and strict writing passed.
Earlier compilation and fixture failures remain in the captures. The final binding fixture uses its provider definition's declared authentication profile.

Evidence commit `609c7f38` preserves seventeen captures, totaling 2,025,415 bytes.
The local planning base contains those exact files, so their output does not enlarge the code review diff.
The plan again separates its header paragraphs. Adjacent table rows preserve its 500-line limit.

Manual acquisition still needs separate observations for each selected binding and publication through runtime retention.
The caller remains responsible for active binding authorization. Field-presence handling, scoped deletion, and full runtime field authority remain open.
CSP3 remains in progress, with zero primary passes and all 50 cases UNVERIFIED.
The shared review helper remains unchanged, and no branch publication or native CI dispatch occurred.


### Manual source binding integration

Commit `31a27ae4` adds `acquisition.WithProviderBindings` to the manual syncer.
Construction copies the declarations without contacting providers. An explicit empty set disables provider acquisition.
Source and provider filters can restrict that set but cannot add bindings.

The pipeline validates each selected profile before source work and emits separate observations for the selected bindings.
Provider calls share the existing concurrency limit. Queued calls stop after cancellation.
Strict mode requires each selected observation and rejects missing, duplicate, mismatched, failed, incomplete, or empty results.

A strict-mode regression rejected valid serving records because the observation lacked authored definitions.
The check now accepts either form of model data. The regression remains in the [captured failure](csp3/manual-bindings-profiles.log).
Previews leave the store unchanged, and durable generation links retain both binding receipts.

Field provenance now preserves optional binding identity and revision through JSON and YAML.
The volume guard compares only matching binding history and keeps the binding when it changes observation health.
Legacy unscoped payloads omit the new fields. History without a matching binding revision supplies no scoped completeness claim.

[Final package runs](csp3/manual-bindings-verification.json) passed 398 race events: 234 reconciler, 82 pipeline, 28 provenance, and 54 acquisition events.
Normal checks cover 79 distinct passing package suites and 22 packages without tests.
The first broad normal run failed its acquisition fixture. The corrected acquisition and pipeline suites then passed.
Code lint, Ago, generated documentation, and strict writing passed. The acquisition race suite completed in 264.865 seconds under its five-minute limit.

Evidence commit `b4ace4a8` includes 37 captures, totaling 1,934,272 bytes.
The local planning base contains those same files. Their output remains outside the code review diff.
Earlier fixture, code lint, and writing failures remain recorded.

Commit `aceea1a8` also restricts volume history to the binding's canonical provider.
The [regression](csp3/manual-bindings-provider-history-regression.log) showed that a reused binding identity could import another provider's history.
Final pipeline race and normal checks, code lint, and Ago pass after that correction. The other package runs precede this three-line guard.

CLI and HTTP adapters still need shared-setting composition and runtime retention during publication.
The standalone syncer does not replace the runtime's retained layer ledger or policy enforcement.
Scoped deletion, field-presence handling, and released-pair acceptance remain open.

CSP3 remains in progress. All 50 primary cases remain UNVERIFIED.
The shared review helper remains unchanged. No branch push or native CI dispatch occurred.


### Canonical runtime reconstruction

Work commit `05ba7e26` records this component. Evidence commit `cb63e726` includes 41 captures, totaling 11,827,113 bytes.

Runtime rebuilds now restore retained provider observations and use the canonical reconciler.
The effective catalog preserves field provenance. Durable generations include the original provider links and excluded-model review candidates.
Legacy provider layers retain separate receipts. Active binding selection still precedes reconciliation.

Generated change timestamps use retained publication and observation times.
Each reconstruction evaluates pricing at the current time. Stable rejection text names the interval boundary.
Unchanged retained inputs reproduce the same payload and generation identity.
Concurrent rebuilds serialize durable publication and effective-state activation.

The first regression proved that runtime reconstruction lost provider field receipts.
The broader checks found that old runtime fixtures supplied authored definitions through provider observations alone.
Those fixtures now supply reviewed definitions through an explicit catalog source. Existing model-retention and partial-failure assertions remain unchanged.
A new regression excludes provider-only authored definitions and retains their original review evidence.

A second regression proved that the primary filter synthesized provenance for unselected providers.
The filter now runs before provider reconciliation. Reconstruction also avoids an unused copy of the full baseline.
CPU profiles identify serialization and garbage collection costs. These profiles do not measure gateway request latency.

The [verification record](csp3/runtime-reconciliation-verification.json) records exact checks, input hashes, and captured failures.
All 79 normal package suites passed, with 3,505 passing test events and 22 packages without tests.
Final race results contain 737 passing events: 358 runtime, 54 acquisition, 243 reconciler, and 82 pipeline events.
The runtime race suite completed in 435.906 seconds. Code lint, Ago, generated documentation, and corrected strict writing passed.

The five-minute race runs exceeded their suite deadline. Historical repository verification also records runtime suites above six minutes.
The plan now uses the repository verifier's existing 20-minute suite deadline for CSP2 and CSP3 runtime checks.
Every test and individual operation deadline remains unchanged. Product latency targets remain unchanged.

CLI and HTTP adapters still need a transaction that retains manual source observations before they adopt the shared binding settings.
Previews must avoid publication. The transaction must preserve source filters, fresh-mode semantics, strict acquisition, workspace projection, and retained layers.
Upstream manifest lineage, complete field presence, scoped deletion, Starport adoption, and released-pair qualification remain open.

CSP3 remains in progress. All 50 primary cases remain UNVERIFIED.
The shared review helper remains unchanged. No branch push or native CI dispatch occurred.


### Receipt-only generation identity

Work commit `dab45969` binds effective generation identity to catalog bytes, source links, and review candidates.
The regression changed an older provider receipt while a newer provider observation continued to supply the selected values.
Before the correction, publication reused the prior generation and retained the old receipt.
The corrected generation keeps the same payload checksum and publishes the new receipt under a distinct identity.

Evidence order and empty-slice representation do not change identity. Empty evidence preserves the baseline identity input.
A separate restore check confirms that the existing store rejects altered manifest evidence under a retained identity.
It returns an immutable generation conflict and preserves current state. No restore implementation changed.

The [verification record](csp3/receipt-identity-verification.json) preserves 17 captures and seven source or generated-document hashes.
Normal checks passed 79 package suites and 3,507 test events, with 22 packages without tests.
Broader race checks passed 657 events: 360 runtime, 54 acquisition, and 243 reconciler events.
Three final focused race tests passed after the restore test added an explicit conflict assertion.

Code lint, Ago, generated documentation, and strict writing passed. The initial receipt publication failure remains recorded.
The passing restore capture retains its original filename and does not count as fail-before evidence.

Identity construction occurs during catalog reconstruction. These checks do not measure inference overhead.
Manual source transactions and CLI and HTTP composition remain open. CSP3 remains in progress, and all 50 primary cases remain UNVERIFIED.
The shared review helper remains unchanged. No branch push or native CI dispatch occurred.


### Catalog acceptance before input retention

Work commit `b2e46470` adds journaled retention for source refresh and provider windows.
The regression proved that rejected provider publication could change retained files and become active after restart.
The new transaction stages immutable inputs before catalog publication. It replaces retained files only after catalog acceptance.

Startup compares prepared records with the loaded catalog identity and checksum.
It discards a transaction when the prior catalog remains current, including when prior and candidate catalogs are equal.
It completes retention when the candidate is current. A committed record requires replay, and an unresolved prepared record refuses startup.
Recovery validates every input before it writes retained files. Invalid records remain unchanged, and migration refuses pending publication.

An accepted catalog stays active when later retention fails. Reports retain its generation ID and show degraded health.
Cancellation before acceptance leaves the prior inputs intact. After acceptance, a bounded completion attempt continues despite caller cancellation.
Pending recovery blocks further updates in the process. Reopening resolves the recorded outcome or returns a conflict.

The [verification record](csp3/input-publication-verification.json) preserves 28 captures, totaling 7,109,494 bytes, and twelve input hashes.
Broader race checks passed 674 events: 377 runtime, 54 acquisition, and 243 reconciler events.
The runtime race suite completed in 514.635 seconds. Twenty final race events cover recovery, publication reporting, and cancellation.
The final focused run includes the later prior-head-first recovery condition.

Normal verification combines 78 non-runtime suites with the complete corrected runtime suite.
These checks contain 3,524 passing test events across 79 package suites, with 22 packages without tests.
The initial runtime suite reached its deadline because its concurrency fixture awaited retention during a blocked commit.
The fixture now issues two complete publication requests. Its catalog, generation, and receipt assertions remain unchanged.

The agent stopped the superseded race process after the normal suite proved the fixture failure. The subsequent complete race run passed.

Code lint, Ago, generated documentation, all six consumer compositions, and corrected strict writing passed.
Earlier fixture, compile, unused-helper, and writing failures remain recorded.

Manual non-provider observations still need retained representation and CLI and HTTP composition.
Completed immutable inputs remain on disk. CSP5 owns bounded safe collection, and CSP11 owns shared fleet recovery.
Native interruption, downgrade procedures, and released-pair qualification remain open. CSP3 remains in progress, and all 50 primary cases remain UNVERIFIED.

The shared review helper remains unchanged. No branch push or native CI dispatch occurred.
