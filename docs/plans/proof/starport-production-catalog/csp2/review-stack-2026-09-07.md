# Stacked code review preparation

The current complete code bundle contains 2,297,084 bytes after proof documents move to the existing evidence branch.
It exceeds the helper's eight-pass limit with the approved 300,000-byte prompt ceiling. Neither limit needs another shared-tool change.

## Publication structure

The code work uses two stacked draft PRs. The first preserves the existing foundation commit and the verified container correction.
The second contains subsequent acquisition, reconciliation, reset, lookup, and service-configuration changes.
Both require complete diff reviews and repository checks before publication. Executable proof scripts remain under code review.

| Branch | Base branch | Worktree |
| --- | --- | --- |
| `codex/catalog-foundation` | `codex/catalog-plan` | `/Users/jack/src/github.com/agentstation/starmap-catalog-foundation` |
| `codex/catalog-requirements` | `codex/catalog-foundation` | `/Users/jack/src/github.com/agentstation/starmap-catalog-requirements` |

The [stack record](review-stack-2026-09-07.json) identifies the foundation source and container correction.
The final product tree matches the earlier combined tree exactly outside planning and design documents.
No source file or executable verifier leaves the review scope. Both prompt preflights pass.

The foundation retains the verified container correction because its earlier pinned image was unavailable.
Its native workflow fixtures pass 26 race results. Full repository verification and actual model reviews remain open.
CSP2 remains in progress. This publication split gives no task completion or product acceptance credit.
