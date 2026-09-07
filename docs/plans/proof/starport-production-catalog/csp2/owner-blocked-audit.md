# Owner decision audit

The same owner decisions remain unanswered across three consecutive goal turns.
Each preceding turn completed available preparation. The service configuration, native Windows fixtures, and remaining Starport integration changes are now committed.

The [audit record](owner-blocked-audit.json) records the dependency check. No remaining `todo` task is eligible under the current dependency graph.
CSP2 still needs native qualification and required review. CSP3 still needs the acquisition-scope link decision.
The plan remains active with CSP2 as its current task. Blocking the thread goal does not mark the plan complete.

The shared review helper still uses its 512,000-byte prompt ceiling. Earlier code review attempts failed because their prompts exceeded the reviewer's context window.
The [prepared review replay](../csp3/review-smaller-prompts-replay.json) and existing one-line patch support the proposed 300,000-byte ceiling.
Changing the shared installation remains outside the authorized repository changes and requires the pending approval.

The second decision selects whether account-specific removals affect only explicitly linked inference profiles.
No answer authorizes that policy yet. Do not apply account-specific removals to unrelated profiles through an assumed scope link.

Resume after the owner answers. Apply only the approved helper change, complete publication checks and review, then run native CI.
Use the confirmed scope policy to resume CSP3. The complete released-pair acceptance and merge-triggered cleanup requirements remain unchanged.
