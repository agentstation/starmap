# Owner approvals on 2026-09-07

The owner approved both requests: the shared review-helper patch and explicit acquisition-scope links for account-specific removals.
This decision resolves the prerequisites in the [earlier audit](owner-blocked-audit.md). That audit remains historical evidence.

## Review helper

The shared helper now limits each prompt to 300,000 bytes instead of 512,000 bytes.
Its Python syntax check and deterministic self-tests pass. The [approval record](owner-approvals-2026-09-07.json) records the exact file hashes.
The existing eight-pass limit and complete-diff review requirement remain unchanged. A successful model review is still required before publication.

## Account-specific removals

An account-specific removal affects only Starport inference profiles explicitly linked to the observation's acquisition scope.
It must preserve canonical model discovery and routes through unrelated accounts. Neither a shared provider name nor credential material establishes that link.
A partial or failed observation cannot authorize removal. Internal Starmap authority remains binding on every subscriber.

CSP3 owns scoped evidence and deletion semantics. Starport integration must enforce those semantics through explicit profile links.
D25 records the requirement. Implementation, migration behavior, and acceptance evidence remain open.

Resume the publication checks and required review, then run native qualification and continue CSP3.
The complete released-pair acceptance and merge-triggered cleanup requirements remain unchanged.
