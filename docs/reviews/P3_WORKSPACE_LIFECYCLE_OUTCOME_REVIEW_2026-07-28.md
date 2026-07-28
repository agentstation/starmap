# P3 Workspace Lifecycle Outcome Review

Date: 2026-07-28

## Scope

The frozen review baseline was the complete
`codex/catalog-workspace-lifecycle` branch against
`origin/main@60f0cd3c6eecf0ecb9be7dc76961abf97919324d`: 81 changed files,
`+4,802/-640` total lines, and `+1,996/-503` non-test lines before review
remediation. The reviewed outcome was the complete P3 human-workspace
lifecycle, including:

- one provider-YAML workspace;
- embedded/local observation separation;
- first-run seed and embedded E1→E2 upgrades;
- generation-store commit followed by repairable YAML projection;
- multi-process writer exclusion;
- semantic-edit conflict detection;
- retained-generation rollback; and
- explicit legacy-layout migration, restart recovery, and downgrade rejection.

The review prioritized data loss, atomicity, restart recovery, rollback,
concurrency, public behavior, and avoidable complexity. F-049 fixture slimming
and the bounded CI race timeout were treated as product-neutral harness changes.

## Method

The autoreview helper built and validated the complete 358,055-byte branch
bundle in one pass, then correctly refused to launch its default Codex engine
inside an existing Codex-managed session. The skill's documented in-session
fallback was therefore used: a repository-grounded audit of the full production
diff, adjacent persistence and publication code, failure tests, public options,
and operator documentation.

The material remediation below received a second focused audit of its ownership
and failure paths. The review was not repeated for documentation or evidence
changes.

## Findings and dispositions

### F-050 — migration rollback could delete a concurrently recreated path

Severity: high

Status: fixed in P3

After the old generation store was atomically moved to its new state root,
`rollbackLegacyMove` unconditionally removed the vacated legacy path before
moving the store back. An obsolete binary or operator that recreated that path
during the migration window could therefore have its new data deleted by a
later rollback.

The fix permits rollback deletion only when the visible directory is the exact
semantic YAML projection produced by this migration. A missing path is safe to
restore over. Any unexpected, machine-layout, symlinked, non-directory, invalid,
or checksum-mismatched path returns a typed conflict and preserves both the
recreated path and the relocated generation store for explicit recovery.

Focused proof covers:

- post-move failure restoring the byte-identical old store;
- post-projection marker failure deleting only the migration-owned projection
  and restoring the old store; and
- concurrent path recreation preserving the new data and the exact relocated
  generation while returning a joined typed conflict.

Operator help and architecture documentation now require all old Starmap
processes to be stopped before migration and never restarted against the path's
new human-workspace meaning.

### F-051 — `WithEmbeddedCatalog` is now an inert public option

Severity: medium

Status: accepted follow-up for P5.8/P6.5

P3 correctly makes the verified embedded catalog an unconditional,
lowest-authority observation, but the older `WithEmbeddedCatalog` option and CLI
configuration flag still claim that embedded behavior is opt-in. The private
boolean has no production reader, so the surface is misleading and adds no
substitutability.

Removing a public option and its CLI configuration is a public-API cleanup
outside the migration failure boundary. It is explicitly assigned to the
existing prelaunch-compatibility/deletion work in P5.8/P6.5, where the golden
consumer journeys and generated API documentation can change together. It is
not a P3 correctness blocker.

## Result

No other accepted P3 blocker remained after F-050 remediation. Exact full local
verification and both hosted required checks on the final committed PR head
remain the authoritative merge gate. F-051 remains visible in the durable
finding ledger and cannot be lost at phase transition.
