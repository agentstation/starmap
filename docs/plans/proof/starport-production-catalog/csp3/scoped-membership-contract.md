# Scoped membership contract

CSP3 must preserve scoped membership evidence through publication, transport, restart, and Starport admission.
The existing catalog payload carries model records and provenance. It carries no scoped inventory or withdrawal record.
A provider record that disappears from one account must not disappear from unrelated accounts.

## Verified boundaries

| Boundary | Current evidence | Missing contract |
| --- | --- | --- |
| Acquisition | `pkg/sources/provider_binding.go` declares binding identity, revision, provider, account or project, region, API surface, and credential role. | A public binding does not prove complete provider-wide coverage. |
| Retention | `runtime/layers.go` retains each binding revision separately. Manual history preserves original receipts and payloads. | Complete omission cannot yet establish a durable scoped restriction. |
| Publication | `pkg/catalogs/generation_manifest.go` carries small source links. | The link omits scope selectors and effective membership. |
| Transport and restart | `runtime/layers.go` retains the upstream catalog payload. | Manifest-only additions would not survive this reconstruction path. |
| Routing | D25 requires an explicit inference-profile link to account-specific acquisition scope. | Starport needs an immutable scope result bound to the accepted generation. |

The existing public-omission probe expects global removal from `Public=true`, `Region=global`, and `APISurface=models.list`.
The current runtime cannot apply the probe's expected deletion. Those selectors alone do not establish authority over every provider API surface.
The final regression must include the selected authority policy. It must not infer that policy from the probe's convenience values.

## Required behavior

1. Keep canonical model identity, global offering membership, and account-specific availability separate.
2. Bind each scoped membership result to its publisher, binding identity, binding revision, and original source evidence.
3. Persist the result in the accepted generation. Include its meaning in the catalog digest and compatibility contract.
4. Preserve the result through source replacement, restart, and derivative publication.
5. Apply account-specific restrictions only to explicitly linked inference profiles.
6. Refuse an affected route when its configured link cannot resolve the required current scope.
7. Preserve unrelated accounts, regions, API surfaces, providers, and authored model definitions.
8. Let a complete accepted observation replace membership only within its declared authority.
9. Preserve accepted membership after failed or partial replies. Apply independently valid positive evidence without inventing complete coverage.
10. Retain original observation identities when effective facts depend on several observations.
11. Apply removal review before activation. An unaccepted candidate must not erase accepted evidence.
12. Distinguish provider availability from internal-server permission withdrawals. A review threshold must not bypass the accepted withdrawal fence.

A source receipt alone cannot replace the effective membership contract. Source links prove identity and integrity, but consumers also need the scope's meaning.
A consumer that cannot enforce a new restriction format must reject that generation under the compatibility contract.
The prior accepted generation remains available under the configured authority and outage rules.

## Pending owner decisions

The following recommendations remain proposals. They grant no implementation or acceptance credit.

| Decision | Recommendation | Reason |
| --- | --- | --- |
| Global removal authority | Require an explicit declaration of complete provider-wide inventory authority. | Public access, a global region, and one API operation do not prove coverage of every offering. |
| Default removal review | Require review above 10% removal or for removal of the whole inventory. Permit an explicit configured threshold. | Ordinary small updates can proceed while large omissions require examination. |

The agent requested both decisions on 2026-09-08. The existing D25 account-link decision remains unchanged.

## Required verification

Use the existing A08 and A20 subcases. Preserve the 50-case and 324-subcase roster.

Cover complete empty inventories, one omitted offering, partial empty replies, and valid records with omitted optional fields.
Cover equal IDs across accounts, regions, API surfaces, and publishers.
Cover first acquisition, repeated acquisition, withdrawal, restoration, restart, source refresh, and explicit resets.
Cover unresolved profile links, changed binding revisions, rejected review candidates, and unsupported consumer schemas.
Verify the complete flow through the Starmap CLI and server, followed by Starport discovery and inference admission.

The expanded pricing regression covers 24 scenarios through durable runtime publication, restart, and a later source refresh.
It preserves accepted free or paid pricing when a partial reply omits a record or its pricing field.
For a present record, it also checks that the new context limit survives while the prior price remains intact.
This regression does not prove scoped deletion, explicit null handling, HTTP transport, or Starport admission.
