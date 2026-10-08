# CSP4 preparation

CSP4 implementation starts from reviewed CSP3 dependency `b5af1e9c` while its final CI runs.
CSP3 must merge before CSP4 publication. The current resume state owns execution status.
This inspection changes no product code and gives no task completion credit.

The inspected Starmap source is `3609f17174e4d42478b0bba47c2d2f34bcb3e2c0`.
The worktree is `/tmp/starmap-canonical-aliases-2026-09-10`.
The engineering specification owns the authority contract in sections 2, 3.2, and 8.2.

## Current behavior

`runtime/policy.go` defines `prefer_source`, `require_source`, and `prefer_local`.
It has no retained-authority startup policy or configured authority identity.
`runtime/runtime.go` requires an online read for `require_source` when this process owns the refresh lease.
An existing lease owner causes the other process to skip that read.
The lease does not prove that the retained generation belongs to an approved authority.

`runtime/scheduler.go` treats the strict online policy as already checked during startup.
The new authority policy needs its own startup and reconnect behavior.
`runtime/source.go` selects one source and prevents implicit public fallback.
Preserve that source-selection boundary.

`remote/source.go` reports a safe source name.
`pkg/catalogs/remote/chain.go` validates bounded instance identities and source chains.
Those identities do not establish authenticated catalog authority or permission validity.
Keep source-chain identity, catalog publisher identity, and permission authority distinct.

## Required implementation boundaries

Bind retained generations and scope links to the configured, authenticated authority.
Use that binding for warm startup and explicit authority transitions.
Keep catalog freshness separate from permission validity.
An approved endpoint migration can retain the authority only while its transport trust remains valid.

Publish the required permission revision separately from catalog payload compatibility.
Commit the envelope with the authority publication head.
Expose the required revision, enforced revision, and remaining permission validity.
Preserve the approved five-minute bound and 30-second clock uncertainty limit.
Unknown clock validity must block new attempts.

Test cold refusal, warm retained startup, changed authority, and lease followers without accepted state.
Test incompatible payloads, unknown mandatory permission semantics, expired receipts, replay, and missed notifications.
Starport owns request admission, cached-response enforcement, retries, and queued work under its later task criteria.

## Baseline verification

The existing product verifier exits 1 for `--task CSP4 --json` at the inspected source.
The verifier reports UNVERIFIED for all eight selected subcases because the registry has no behavior checks for them.
This result identifies missing qualification and does not prove that every underlying behavior fails.

The selected checks are the four A07 cases, A09 excluded membership, A10 cold refusal and warm startup, and A21 mixed-schema permission envelopes.
The [baseline report](https://github.com/agentstation/starmap/blob/af32c6ce2468bcb2260345881516e99a049ac892/docs/plans/proof/starport-production-catalog/csp4-preparation-2026-09-10.json) retains the exact result.
Register real Starmap behavior checks for this task and repeat consumer contracts during Starport integration.

## Combined-source transport inspection

This inspection uses combined CSP3 source `b5af1e9c67e065ebd688a19a7563bcc430f724bb` before its merge.
It changes no product code. The original baseline report remains valid for the missing registry checks.

The current remote client decodes a compatible generation manifest before it returns a catalog read.
The `runtime/source.Read` value has no independent permission envelope.
The `runtime.readSource` function returns immediately on source errors.
It treats an unchanged catalog as a completed source check.
These paths cannot learn a permission withdrawal when a newer payload or manifest exceeds the consumer's supported schema.

CSP4 must verify the small permission envelope independently of catalog decoding.
An unchanged catalog can still carry a renewed receipt or a higher required permission revision.
An unsupported mandatory permission version must block admission without depending on the catalog parser.
A notification or HTTP 304 response cannot renew permission without a verified receipt.

The current remote `Publication` event carries only generation identity and sequence.
The catalog store commits the complete generation and compares its expected generation identity.
The authority's head must bind the required permission revision to its committed catalog reference.
Subscribers need a separate retained record of the highest authenticated required revision, including when catalog activation fails.
A failed payload activation cannot lower that record or restore earlier admission.

The `runtime.Status.Usable` field reports whether any catalog exists, including the embedded baseline.
CSP4 must report catalog availability separately from authority readiness and permission validity.
The application must retain diagnostics when a cold internal runtime lacks an accepted catalog.
It must not infer authorization from a lease holder or a source's safe display identity.

The next design must cover a permission-retention write failure followed by restart.
An older retained receipt cannot prove that the process never learned a later withdrawal.
Treat uncertain retained permission state as a refusal until fresh authority evidence resolves it.
Preserve retained catalog metadata for diagnostics throughout that refusal.

## Startup behavior probes

The [baseline probes](https://github.com/agentstation/starmap/blob/af32c6ce2468bcb2260345881516e99a049ac892/docs/plans/proof/starport-production-catalog/csp4/startup-baseline-2026-09-10/verification.json) fail both target authority expectations against the existing strict online policy.
The warm probe first accepts and retains a catalog, then fails to reopen while the configured source is offline.
The cold probe refuses the refresh lease but still reports the embedded catalog as usable without accepted internal state.

These probes establish the need for a distinct authority policy. Preserve strict online startup semantics.
The proof retains the temporary test source and raw race output. The worktree no longer contains those temporary probes.
Permanent authority tests must add authenticated identity, permission validity, retained receipts, and publication failure coverage.
