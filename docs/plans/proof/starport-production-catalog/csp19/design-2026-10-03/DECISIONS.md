# CSP19 decisions

Recorded 2026-10-03. The lead asked the owner the first two questions through the question tool and records the answers here. The lead derived the other decisions from the specification, the registry, and the merged task evidence.

## 19-1 Latency profiles state engineering targets with a CSP22 pointer

Owner answer: "Engineering targets, CSP22 pointer (Recommended)".

Each recipe page states the current engineering targets from `docs/performance-targets-v1.json` and labels them UNVERIFIED. The page names CSP22 as the qualifier. A docs-structure Go test backs `A31.recipe_latency_profile` now. CSP22 reruns the subcase with measured values. Consequence: CSP19 closes before any measured profile exists, and the targets page says so.

## 19-2 CSP19 adds local-to-shared migration tooling

Owner answer: "Add local-to-shared migration tooling".

The owner selected the larger scope for `A31.upgrade_restore_workload`. The release gains a tested procedure that moves a populated local recipe into the shared recipe. The local recipe holds Badger, SQLite, and local file bytes. The shared recipe holds Valkey, PostgreSQL, and object storage. Consequences:

- This is a product feature, not a documentation change. It needs its own design survey, acceptance contract, Starport code PR, and Starmap registry entries.
- The lead opens a sibling task for the tooling, with the CSP16.1 to CSP16.2 pattern. CSP19 step 2 depends on that task for the migration procedure.
- The existing limits in `docs/site/storage/migration.md` lines 31 and 32 stay in place until the tooling merges. The fail-before record keeps the current text.
- An existing shared deployment upgrade stays a separate question. The survey reports whether the same tooling covers it.

## 19-3 Target status comes from merged evidence

The targets table marks a target "Supported" only when a merged task proof covers its durability, recovery, and replica behavior. T2 and T7 keep "Supported". T4 moves to the fleet qualification that CSP13 and CSP15 proved, with the CSP15 `master_replid` failover limit and the D42 promotion RPO limit stated beside it. T3 states the single-process recovery evidence that CSP13 and the storage recipe exercise provide. T1 is a Starmap recipe and lives in the Starmap guides.

T5 and T6 keep their current availability statements. No target gains "Supported" without a named proof.

## 19-4 Redis and MySQL stay unqualified

The specification says Redis and MySQL need their own compatibility results before documentation labels them supported. CSP15 qualified Valkey 7.2.14 and PostgreSQL 16.15 only. The fleet refuses MySQL. `docs/PRODUCTION-STATUS.md` line 9 drops the MySQL claim for replicas. No declared recipe uses MySQL or Redis.

## 19-5 The shipped Compose recipes become read-only

`A31.readonly_container_mounts` requires a read-only image with only its declared writable mounts. Both Starport Compose recipes gain `read_only: true` and tmpfs for scratch paths, as the Starmap Compose file already has. The change note states that a deployment that writes outside the declared mounts stops at start. A Go test proves the recipe through the recipe image.

## 19-6 The docs PR carries its runnable checks

The CSP19 acceptance says each recipe has runnable checks. The verifier runs only `go_test`, `all`, `vitest`, `native_ci`, and reviewed-record kinds. The five unregistered subcases therefore need named Go tests in the Starport slice, with the recipe image pattern of `TestContainerRecipePersistence`. The slice is a substantive-code PR and takes the autoreview gate.

## 19-7 Landing order

Starport #410, then #407, then the CSP18 slice 3 PR, then the Starmap CSP18 registry PR land first. The Starport CSP19 slice branches from main after the slice 3 merge. The Starmap CSP19 docs slice runs now on `csp19-docs` from `344eb64cf` and rebases before its PR. The registry entries for CSP19 wait for the merged Starport test names.
