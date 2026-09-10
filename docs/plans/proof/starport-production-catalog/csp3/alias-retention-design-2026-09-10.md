# Canonical model aliases

CSP3 owns this design under D30. It defines required behavior and gives no implementation or qualification credit.

## Identity boundary

A canonical model alias maps a former canonical ID to its successor ID. Provider IDs, author aliases, and exact provider model IDs remain separate concepts.
Starmap owns canonical alias records and their source authority. Starport resolves aliases before selecting an eligible provider offering.
The provider request still uses the exact provider model ID from that offering.

A rename must publish the new target and old-ID alias atomically when the old canonical entry retires.
No accepted generation may leave the old client ID between those states.
An alias cannot shadow a live canonical ID for a different model.
Catalog model counts must count canonical definitions once. Diagnostics can list their aliases separately.

Alias edges represent the same model across canonical renames. Reject cycles, ambiguous mappings, and attempts to assign a retired ID to another model.
A later rename can extend the existing lineage. It must not rewrite an earlier alias to an unrelated target.
Resolve each accepted chain into a terminal target during generation activation.
Request-time resolution must read an immutable index without storage access or graph traversal.

## Source and removal boundary

Every explicit alias record needs an attributable publisher and accepted source evidence.
A matching publisher string alone cannot authorize a mapping. Apply the same authenticated source boundary as catalog membership.
Provider acquisition cannot create a canonical alias merely by reporting another provider model ID.

Elapsed time, provider omission, failed refresh, and ordinary acquisition cannot remove an alias.
An operator action or an authoritative replacement baseline can explicitly remove it.
A legacy payload that cannot express aliases must not imply alias removal.
A complete alias inventory or explicit removal record must distinguish removal from unsupported or missing metadata.

Removing an alias must not remove the target model or its other aliases.
Restore must retain the original model lineage and current authority checks.
Retention and compaction must preserve retired-ID protection as well as active aliases.

## Admission boundary

A retained alias grants no permission. Resolve its target, then apply current membership, scope removal, provider availability, and operation checks.
An authoritative withdrawal blocks new attempts through every alias of the withdrawn target.
Scoped removal blocks only the affected entry and preserves other eligible entries reached through the alias.
Retries and queued requests must repeat current admission under CSP10.

## Verification

A08.alias_cycle_scope must cover these cases through production catalog and runtime APIs:

- An old ID resolves after time advances beyond 30 days and after restart.
- Provider omission changes scoped routing availability while preserving the alias.
- Explicit alias removal preserves the target and other aliases.
- Authoritative baseline removal succeeds, while unsupported legacy metadata cannot imply removal.
- Cycles, ambiguous mappings, retired-ID reuse, and untrusted publisher claims fail before publication.
- A failed rename publication leaves the previous generation and old ID usable.
- Alias chains preserve model lineage and resolve to one permitted terminal target.
- Scoped and canonical removals apply through aliases without granting broader access.

CSP10 qualifies client errors, SDK behavior, concurrent requests, and withdrawal through A18.
CSP5 qualifies durable storage, crash recovery, and compaction.
The existing 38 tasks, 50 primary cases, and 324 required subcases remain unchanged.
