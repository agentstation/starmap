# Storage review resolution

Updated: 2026-09-05. The PRD, engineering specification, repository findings, and plan now incorporate the storage recommendations.
The user requested these document and plan changes. Implementation remains unstarted and the plan remains proposed.

## Contract changes

The specification keeps Starmap `config.yaml` and Starport `config.env` as their primary local files.
It defines platform roots, exact managed files, relative-path anchors, service mounts, owner locks, and migration journals.
Runtime identity belongs to one process. A new replica gets a separate identity and seed.
Effective path reports must show the selected leaf settings across CLI, API, UI, and generated documentation.

Badger and Valkey remain alternative durable KV backends. SQLite and PostgreSQL own relational records separately.
Redis and MySQL require their own qualification. Blob bytes remain in filesystem or object storage.
Shared configuration still uses the proposed PostgreSQL revision authority after bootstrap and explicit initialization.
A store outage cannot reactivate stale local configuration.


The proposed cache default uses bounded process memory. Optional shared caches use a separate cache-only service.
Cache writes, cleanup, and eviction cannot alter durable catalog, account, credential, budget, or usage records.
Refill preserves the original expiry. Unprovable remaining lifetime prevents local refill.

Starport owns cache and storage descriptors. Starmap's shared schema owns catalog settings only.

The proposed production Badger default enables synchronous writes.
This changes the inspected default and therefore needs upgrade diagnostics, workload measurements, and durability evidence.
Exact secure Valkey schemes, supported versions, capacity limits, and recovery objectives remain explicit implementation-profile inputs.
A descriptor must either affect its actual adapter or reject the unsupported setting.

Temporary development uses memory databases and uniquely owned scratch blobs and runtime state.
Conflicting persistent selectors fail before store access and identify the persistent setup procedure.
Early README work discloses current exceptions. Final README and recording evidence must follow A43.
Container recipes must recover linked KV, SQL, and blob records after container removal and recreation.

D16 through D18 record these recommendations as adopted draft contracts, not as implemented guarantees.
D6, D7, D11, and D15 keep their earlier proposal status. Existing confirmed decisions remain unchanged.

## Finding dispositions

All fourteen findings have contract and task coverage. This table does not close implementation findings.
The [acceptance map](../acceptance-map.json) names each required behavioral subcase and its task roster.

| Finding | Priority | Required correction | Tasks | Acceptance |
| --- | --- | --- | --- | --- |
| SR01 | P1 | Runtime directories default per user, not per process. Give every persistent replica a distinct seed and directory. | CSP2, CSP8, CSP11 | A03, A04, A14 |
| SR02 | P1 | Valkey construction strips URI prefixes without complete URL or TLS setup. Define credentials, CA verification, timeouts, and supported endpoint forms. | CSP12, CSP15 | A41 |
| SR03 | P1 | Fixed KV keys can collide across deployments sharing the default keyspace. Define deployment scope or require dedicated services. | CSP11, CSP12, CSP13, CSP15 | A13, A41 |
| SR04 | P1 | Durable and cache records share one backend. Specify eviction isolation, capacity, retention, and acknowledged-write loss. | CSP12.1, CSP13, CSP15 | A15, A16, A42 |
| SR05 | P2 | Platform roots remain inconsistent. Windows default data lives under roaming configuration, and macOS runtime state uses an XDG-style path. | CSP2, CSP8 | A03, A04 |
| SR06 | P2 | Path normalization differs by field. SQLite and runtime overrides do not use the same config-root resolution as Badger and blobs. | CSP2, CSP8 | A03, A04 |
| SR07 | P2 | `config paths` reports default managed paths, not every effective override. Its text output omits SQL, blobs, tokens, and catalog state. | CSP8, CSP16, CSP17 | A03, A24 |
| SR08 | P2 | Some storage knobs do not reach adapters. Wire them, reject them, or remove their advertised effect. | CSP12, CSP15 | A41 |
| SR09 | P2 | Cache capacities and TTLs are internal defaults in application composition. Expose bounded settings through the canonical descriptor contract. | CSP8, CSP12.1, CSP16, CSP17 | A24, A42 |
| SR10 | P2 | Hybrid cache refill grants a fresh local TTL without checking the backing entry's remaining lifetime. Add expiry-preserving refill tests. | CSP12.1, CSP15 | A42 |
| SR11 | P2 | Starmap dotenv order conflicts with its comment, and YAML does not cover the complete canonical catalog schema. | CSP1, CSP8 | A24 |
| SR12 | P2 | Target file names omit several operational artifacts and leave alternate configuration formats ambiguous. Complete the file manifest before migration. | CSP2, CSP8, CSP16, CSP18 | A03, A04, A24 |
| SR13 | P1 | The Starport Compose example persists Valkey but leaves SQL, blob, token, and runtime files in the container. Qualify a complete persistence recipe. | CSP12, CSP19, CSP22 | A31 |
| SR14 | P1 | Development mode preserves an explicit object-store backend while creating local scratch. Guard or clearly declare that persistent-storage exception. | CSP0.2, CSP8, CSP12, CSP20 | A34, A43 |

SR09 now routes Starport cache descriptors to Starport-owned tasks instead of adding them to Starmap's public catalog schema.
SR11's file-loading contract belongs to configuration tasks. Credential conflict behavior must still preserve the role and payer rules.
CSP12.1 separates cache work from storage connection and budget admission work.
The original review's task suggestions remain historical evidence. The current acceptance map owns routing.

## Plan and verification changes

The ledger contains 31 tasks, all todo. Existing IDs and relative task order remain stable.
CSP12.1 follows CSP12 and precedes migration and backend qualification.
The plan retains its single outcome and final archive task.

The primary roster grows from 40 to 43 cases. A41 covers effective storage and secure backend behavior.
A42 covers cache isolation and expiry. A43 covers temporary development storage.
Existing path, migration, configuration, recipe, and README cases gain explicit storage subcases.
The required subcase count grows from 194 to 241, an increase of 47.

Candidate qualification requires 36 complete primary cases.
A06, A29, and A35 through A39 still need publication and remain outside the candidate pass count.
Only the final 43-case result supports the complete production claim.
Early first-use and candidate recording checks retain separate evidence.

The storage review's macOS run passed 19 top-level tests and 46 subtests across four packages with the race detector.
That run did not execute external backend or native Linux/Windows qualification.
It does not satisfy the new A41 through A43 contracts.
This document revision runs writing, structure, mapping, source-reference, and evidence-preservation checks only.
No product acceptance case receives credit from document validation.

## Evidence and document ownership

- [PRD](../../../../design/catalog-lifecycle/PRD.md)
- [Engineering specification](../../../../design/catalog-lifecycle/ENGINEERING_SPEC.md)
- [Repository findings](../../../../design/catalog-lifecycle/REPOSITORY_FINDINGS.md)
- [Original storage review](../../../../design/catalog-lifecycle/STORAGE_REVIEW.md)
- [Canonical plan](../../../starport-production-catalog-plan.html)
- [README and demonstration brief](../readme-demo-brief.md)
- [Input and historical evidence hashes](input-manifest.json)
- [Revision verification](verification.json)

Original audits, attributed Fable feedback, screenshots, and raw test outputs remain unchanged.
The current findings append this revision to their dated history.
Current indexes route readers to the same plan. Product READMEs, GIF bytes, code, and deployment manifests remain implementation tasks.
No commits, publication, provider calls, or external migrations occur in this revision.
