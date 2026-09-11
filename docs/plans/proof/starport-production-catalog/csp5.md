# CSP5 update controls and retained state

CSP5 is active after the CSP4 implementation merges. Starmap starts at `f9951ee6` on branch `codex/catalog-update-controls`.
Its worktree is `/Users/jack/src/github.com/agentstation/starmap-catalog-update-controls`.
Starport merge `cb6f03a2` has the same tree as the consumer worktree at `ba9b0d8e`.

The [baseline](csp5/baseline-2026-09-11/verification.json) returns exit 1 and reports twelve unverified subcases.
The registry contains no behavior checks for those subcases. This registry result does not establish a product defect.
Add behavior-specific red tests before implementation. The retained preflight identifies the existing owners and constraints.

The [task contract](../../starport-production-catalog-plan.html#task-CSP5) owns all acceptance requirements.
Source scheduling, catalog network permission, and generation selection need separate controls.
A generation pin must not bypass an authoritative permission withdrawal. Offline mode cannot renew permission through a network request.

Retention work must preserve selected generations, original receipts, omitted offerings, and reviewed inputs across restart.
Collection must exclude active writers and unknown files. Ambiguous publication outcomes need reopen and idempotent-retry evidence.
No CSP5 product behavior passes yet.
