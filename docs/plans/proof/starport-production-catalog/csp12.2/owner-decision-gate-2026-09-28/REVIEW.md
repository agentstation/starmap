# Pending product decisions

CSP12.2 cannot select its remaining behavior without two owner decisions.
The [dependency audit](audit.json) records the current source and all fourteen unfinished tasks.

The first decision selects recovery for batch lines that durable evidence proves never started.
The recommendation resumes those lines automatically. Both choices retain completed results and prohibit automatic replay of uncertain attempts.
The [batch contract](../batch-recovery-contract-2026-09-27/CONTRACT.md) requires the owner answer before restart implementation.

The second decision selects the correction horizon for settled charges.
The recommendation permits corrections for 90 days. Other presented choices are one year or no expiry.
Every choice retains unresolved reservations until reconciliation.
These are product decisions, not requests for implementation or merge permission.

CSP14 is complete. PR #163 merged as `e7a6abdf7`.
Its [merge proof](../../csp14/merged-2026-09-16/verification.json) records the completed task.
The proposed switch to CSP14 was incorrect. No worktree or code changed for that proposed switch.

CSP13 depends on CSP12.2. CSP15 also depends on CSP13.
CSP16 depends on CSP13, and the remaining implementation and release tasks depend on that sequence.
No independent unfinished task meets its recorded prerequisites.
The existing CSP12.2 tests and measurements remain historical evidence for their recorded source.

Both questions remain pending. The third unchanged observation sets the autonomous goal to `blocked`.
After the owner answers, resume CSP12.2 in the recorded worktrees and complete its checks, review, dependency qualification, and paired merges.
No new product test, remote write, service start, or code change occurred during this audit.
