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

Keep each original rename edge after an operator or baseline removes the alias. An explicit state records whether clients can use the retired ID.
Later renames extend that edge chain. They must not replace the earlier target or publisher.
Removing an intermediate alias must preserve other active aliases that reach the same terminal model.

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

A removed canonical ID must block convenience lookup. A current provider model ID must not silently restore that removed alias.
Reject an alias that gives one public request name a different canonical target from an existing provider route.
This applies the existing ambiguous-identity requirement to the shared request namespace.

## Admission boundary

A retained alias grants no permission. Resolve its target, then apply current membership, scope removal, provider availability, and operation checks.
An authoritative withdrawal blocks new attempts through every alias of the withdrawn target.
Scoped removal blocks only the affected entry and preserves other eligible entries reached through the alias.
Retries and queued requests must repeat current admission under CSP10.

## Consumer integration findings

At Starmap `129e3f68`, `pkg/catalogs/readonly.go` derives convenience aliases from current slugs and provider IDs.
`Catalog.FindModel` has no durable canonical rename inventory. `Definition` and `DefinitionOfferings` use exact canonical IDs.
The rename index must remain separate from those derived convenience names.

`internal/catalog/reconciler/authored_corpus.go` can add embedded definitions during reconstruction.
`offering_quarantine.go` validates exact canonical references and can carry a prior reviewed link.
Reconciliation must apply accepted rename history when it derives references from retained evidence.
Older embedded records must not recreate a retired canonical ID. Preserve the original observations and their receipts.
An unverified or unmatched rename remains a review candidate. Provider model names alone cannot establish that two models share an identity.

At Starport `d45bb30`, `internal/catalog/snapshot.go` accepts provider route IDs and canonical definition IDs.
`Names`, `ResolveRoute`, and `ResolveOperation` do not resolve retained canonical renames.
`LowestSearchUnitPrice` compares names separately. Updating route resolution alone would leave alias cost checks inconsistent.
`PagePriceFor` uses `ResolveOperation` and must retain the same operation and generation.

CSP8 and CSP10 must use one accepted identity result for discovery, cost checks, routing, and provider request construction.
The adapter still receives the exact selected provider model ID. Alias resolution cannot substitute another operation or skip scoped admission.
Tests must cover removed names that collide with convenience names and provider routes.

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
