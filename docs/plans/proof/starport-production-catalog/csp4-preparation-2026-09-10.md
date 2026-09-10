# CSP4 preparation

CSP4 remains pending while CSP3 completes qualification and merge.
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
The [baseline report](csp4-preparation-2026-09-10.json) retains the exact result.
Register real Starmap behavior checks for this task and repeat consumer contracts during Starport integration.
