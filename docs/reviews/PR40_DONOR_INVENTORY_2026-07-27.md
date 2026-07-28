# PR #40 Read-Only Donor Inventory

Date: 2026-07-27

Pull request: [#40](https://github.com/agentstation/starmap/pull/40)

Exact donor head: `a14d22497479cb944932274088cf806cb25e993b`

Merge base: `9508ee7866e4683e001e7ad153319d348433045d`

Reviewed protected main: `a87b64252f022c398589c5aad8652357ba8a174a`

Branch range: 576 files, 45,142 insertions, 10,593 deletions.

## Decision

PR #40 must not be merged, rebased, or reused as an implementation base. Its
persisted definitions/offerings schema conflicts with the selected architecture
in `docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md`, and its breadth prevents a
reliable conceptual review.

This inventory preserves useful research without preserving the rejected
architecture. `SALVAGE` means only the named concept, primary-source research,
or fixture is a candidate for deliberate reimplementation on a fresh phase
branch. It does not authorize a cherry-pick or wholesale copy.

Statuses:

- `SALVAGE`: retain the named evidence or idea for a later owning task; recheck
  it against current provider APIs, the provider-YAML contract, dependency
  budgets, and file-size limits before writing code.
- `ALREADY_LANDED`: protected main contains an equal or stronger current result.
- `SUPERSEDED`: the current control plane owns the need with a more precise
  contract; do not carry the donor implementation forward.
- `REJECT`: the change is coupled to the rejected schema, adds incidental
  complexity, or conflicts with the target product.

## Production Go module inventory

The inventory unit is every Go package directory with at least one changed
non-test `.go` file in the PR range. The root `starmap` package is one module.
All 66 modules produced by the reproducible command below appear exactly once.

| Changed production module | Status | Rationale and owning follow-up |
| --- | --- | --- |
| `cmd/provider-fixtures` | `SALVAGE` | The governed import/replay workflow and source-keyed fixture layout can reduce duplicated provider test tools. Reassess after P4/P6 defines source identity; retain only secret-free raw fixtures with a named verification use. |
| `cmd/starmap/app` | `SUPERSEDED` | Donor wiring is coupled to schema-v2 acquisition and broad application interfaces. P6.3–P6.4 owns opt-in acquisition and narrow command interfaces. |
| `cmd/starmap/cmd/auth` | `SALVAGE` | Configuration-owned Vertex environment precedence and secret-safe command errors are useful. Reimplement against the final provider-YAML source contract; do not load the catalog merely to infer ambient credentials. |
| `cmd/starmap/cmd/deps` | `SUPERSEDED` | Configured-source enumeration depends on the final acquisition composition. P6.3–P6.4 owns it after the library dependency boundary is fixed. |
| `cmd/starmap/cmd/providers` | `SALVAGE` | One normalized acquisition path for normal, raw, and stats modes plus hostile-origin rejection are valuable. Reapply after P4.8 and P6.3; arbitrary URLs and a second parser remain forbidden. |
| `cmd/starmap/cmd/serve` | `SUPERSEDED` | The donor continues the internal server shape. P7.1 owns a public embeddable server with explicit lifecycle. |
| `cmd/starmap/cmd/update` | `SUPERSEDED` | Update semantics depend on the P3 durable-commit/YAML projection contract and P4 authority model. |
| `cmd/starmap/cmd/validate` | `SUPERSEDED` | Validation of persisted definitions/offerings is rejected. P3/P4/P9 own provider-YAML, observation, generation, and artifact validation. |
| `internal/acquisition` | `SALVAGE` | The immutable request/result vocabulary, configured endpoint ownership, and normalized transport evidence are candidates for P6.3. Keep it only if two real adapters or a concrete test substitution justify the seam. |
| `internal/acquisition/testsource` | `SALVAGE` | A test adapter is useful only if the retained acquisition boundary needs it for behavior/failure tests; otherwise delete it under P8.6. |
| `internal/attribution` | `REJECT` | Replacing a narrow reader input with mutable `*catalogs.Builder` moves in the wrong direction and is tied to persisted definitions. |
| `internal/auth` | `SALVAGE` | Request-scoped credential resolution, deterministic alias conflict handling, and secret-safe diagnostics are valuable for opt-in acquisition. They must remain outside read-only library construction and never persist resolved values. |
| `internal/auth/adc` | `SUPERSEDED` | Deleting special-case ADC parsing may be correct, but the final decision belongs with P6.3’s provider composition and official-SDK adapters. Do not transplant the deletion independently. |
| `internal/auth/cloudchains` | `SALVAGE` | Delegation to official AWS/Azure/Google/OCI SDK credential chains is preferable to reimplementing precedence. Retain only for selected providers and behind the P6 dependency-closure boundary. |
| `internal/bootstrap` | `SALVAGE` | Exact/duplicate-key boundary validation and rejection before publication are useful patterns. Reimplement for the provider-YAML and generation contracts in P4.8/P9.3; reject schema-v2 payload coupling. |
| `internal/catalog/pipeline` | `REJECT` | The donor retains the store-only empty-path save defect and the wrong YAML-before-store commit order while adding schema-v2 coupling. P3.6a/P3.6b/P3.8 replace this flow. |
| `internal/catalog/query` | `SUPERSEDED` | Derived definition/offering queries are required, but donor queries assume those objects are persisted. P5 derives them from canonical provider models. |
| `internal/cli/completion` | `REJECT` | The change is incidental constant extraction and does not justify donor carry-forward. |
| `internal/cli/hints` | `SALVAGE` | Provider-configured credential names are better than a hard-coded provider list. Rework after provider YAML is stable and avoid secret discovery during ordinary catalog reads. |
| `internal/cli/notify` | `REJECT` | Ambient environment probing through embedded catalog construction adds hidden behavior and couples notifications to credentials. |
| `internal/cli/table` | `SALVAGE` | Source-scoped authentication/readiness presentation is useful operator DX. Rebuild over final observation/source health types without exposing secret values. |
| `internal/connectors/anthropic` | `SALVAGE` | Shared Anthropic wire behavior can remove duplicate clients. Reapply with bounded per-record decode/fuzzing and a real second consumer. |
| `internal/connectors/google` | `SALVAGE` | Shared Google pagination/normalization is useful, but the donor’s large file must be split by wire, pagination, normalization, and error concepts where that improves locality. P4.8 quarantine is mandatory. |
| `internal/connectors/openai` | `SALVAGE` | A shared OpenAI-compatible connector is canonical for many providers. Do not copy the donor’s 1,565-line file; extract bounded wire decode, normalization, and error concepts with focused tests. |
| `internal/providers/azurefoundry` | `SALVAGE` | Preserve API/version research, pricing normalization cases, and source topology. Revalidate against current primary docs and P4 authority before implementation. |
| `internal/providers/bedrock` | `SALVAGE` | Preserve bounded regional discovery/pricing research and official-SDK fake fixtures. Keep credential-scoped observations out of public generations. |
| `internal/providers/clients` | `SUPERSEDED` | Donor deletes/replaces the factory as part of a larger schema break. P6.2–P6.3 owns inversion behind an injected opt-in provider factory. |
| `internal/providers/cloudflare` | `SALVAGE` | Preserve Workers AI endpoint/auth research and fixtures; require a real protocol delta before retaining a custom client. |
| `internal/providers/cohere` | `SALVAGE` | Preserve Cohere list-model wire fixtures and normalization research; validate current API shape and quarantine malformed siblings. |
| `internal/providers/databricks` | `SALVAGE` | Preserve the public-versus-workspace source distinction and no-auth public endpoint evidence. Never attach workspace credentials to a documentation or caller-supplied origin. |
| `internal/providers/fixtures` | `SALVAGE` | Central source-keyed fixture loading and deterministic policy can replace per-provider protocol retests. Retain only high-value fixtures with provenance and secret scanning. |
| `internal/providers/fixtures/responses` | `SALVAGE` | Refresh metadata, bounded capture, and verification are useful. The final store must not treat fixture timestamps as semantic catalog changes. |
| `internal/providers/huggingface` | `SALVAGE` | Preserve provider discovery/topology research and fixtures; revalidate pagination, provider identity, and availability semantics. |
| `internal/providers/novita` | `SALVAGE` | Preserve YAML/source facts and any demonstrated wire delta. Prefer the shared OpenAI-compatible connector if no delta remains. |
| `internal/providers/nvidia` | `SALVAGE` | Preserve the public catalog versus credential-scoped NIM separation and fixtures. They must be distinct logical sources with explicit completeness. |
| `internal/providers/oci` | `SALVAGE` | Preserve official-SDK discovery, region binding, and fake fixtures. Keep it opt-in and outside the root library closure. |
| `internal/providers/registry` | `SALVAGE` | A configuration-driven connector/provider selection point may be useful for P6.3. Keep it only if it is a deep module with multiple real adapters; otherwise compose directly. |
| `internal/providers/snowflake` | `SALVAGE` | Preserve Cortex API research and fixtures; explicitly model account/region scope and non-public observations. |
| `internal/providers/testhelper` | `SUPERSEDED` | Donor replaces this duplicate fixture helper with the centralized fixture concept. Any retained behavior belongs in the reviewed P4/P6 test seam. |
| `internal/providers/together` | `SALVAGE` | Preserve serverless/dedicated source separation and fixtures. Do not hide two logical acquisitions inside one `ListModels` call. |
| `internal/providers/watsonx` | `SALVAGE` | Preserve API/version, project/space scope, pagination, and fixture research; revalidate current IBM contracts before use. |
| `internal/providers/xai` | `SALVAGE` | Preserve provider configuration and the observed 403 evidence. Prefer the shared OpenAI-compatible connector unless a current wire delta is proven. |
| `internal/server/handlers` | `SUPERSEDED` | Donor handlers expose schema-v2 definitions/offerings through the internal server. P7 owns public composition, immutable manifest/payload, SSE, and health contracts. |
| `internal/server/params` | `SUPERSEDED` | Filters are coupled to persisted definition/offering views. P5/P7 owns query semantics over derived views. |
| `internal/sources/modelsdev` | `SALVAGE` | Advisory environment metadata isolation, bounded Git/HTTP behavior, and source identity are useful. The monolithic decode still violates P4.8 and must be replaced with record quarantine. |
| `internal/sources/nativeproviders` | `SALVAGE` | A configured adapter for selected native/cloud providers may serve P6.3. Retain only if it is deeper than direct composition and reports scoped completeness. |
| `internal/sources/providers` | `SALVAGE` | Independent logical-source execution and peer-failure isolation are valuable. Rebuild over P4 observation health and non-authoritative absence rules. |
| `internal/transport` | `SALVAGE` | Configured-origin auth, cross-origin redirect rejection, bounded bodies, retry metadata, and closed-body ownership are production security requirements for P4/P6/P10. |
| `pkg/authority` | `SUPERSEDED` | The donor policy is coupled to persisted offerings and does not resolve the selected provider-YAML evidence model. P4.1–P4.6 owns one executable field policy. |
| `pkg/catalogartifact` | `SUPERSEDED` | Exact artifacts remain required, but P9 must publish the exact committed provider-model generation with semantic/evidence digests, not donor schema-v2 bytes. |
| `pkg/catalogdistribution` | `SUPERSEDED` | P2.8 decides whether this seam has a real production composition; P7/P9 then implement one selected flow or delete it. |
| `pkg/catalogmeta` | `SALVAGE` | Source observation identity, status, issues, and safe summaries are useful inputs to P4.7/P4.9. Rework provider/model scoping and semantic timestamp behavior. |
| `pkg/catalogremote` | `SUPERSEDED` | Donor remote changes remain polling-oriented and schema-v2-coupled. P7 defines verified initial fetch plus SSE notification and mandatory catch-up. |
| `pkg/catalogs` | `REJECT` | This is the center of the rejected persisted definitions/offerings schema. Keep the current concrete immutable catalog and provider YAML; P5 derives read views without compatibility aliases. |
| `pkg/catalogscheduler` | `SUPERSEDED` | P2.8 must first name a scheduler owner/use case or delete it. Do not retain donor schema/source changes by inertia. |
| `pkg/catalogstore` | `REJECT` | Donor removes migration support and persists schema-v2 while the current plan requires typed legacy-layout detection, one CAS commit point, retained generations, rollback, and projection repair. |
| `pkg/differ` | `SUPERSEDED` | Definition/offering diffs are coupled to rejected persistence. P8.6 separately decides the dead `Filter`/`ApplyAdditive` surface after canonical provider-model semantics land. |
| `pkg/errors` | `SALVAGE` | Secret-safe stable error summaries and typed source/acquisition errors are useful. Add only errors demanded by retained contracts and keep `errors.Is/As` behavior. |
| `pkg/logging` | `REJECT` | The donor change is incidental string-constant extraction and adds no architectural value. |
| `pkg/reconciler` | `SUPERSEDED` | Donor reconciliation assumes persisted offerings and still requires the authority/provenance redesign. P4 owns one provider/model-scoped evidence implementation and non-authoritative absence. |
| `pkg/sourceevidence` | `SALVAGE` | Bounded, secret-safe source evidence and exact observation identity are useful. Rework it around one P4 provenance model and avoid duplicate persisted representations. |
| `pkg/sourcepayload` | `SUPERSEDED` | Donor payload rejection is tied to schema-v2 contextual offerings. P9 owns the exact committed generation and public/private boundary. |
| `pkg/sources` | `SALVAGE` | Explicit status, completeness, issues, source scope, retry/volume evidence, and schema-drift signals are required by P4.7–P4.10. Simplify to the minimum contract used by real sources. |
| `pkg/sync` | `SALVAGE` | Source-granular outcomes and safe issue reporting can support P4.7/P4.9. Publication order and workspace writes must instead follow P3’s commit-point contract. |
| `pkg/types` | `SUPERSEDED` | Removing prelaunch aliases is directionally correct, but P5.8/P6.5 owns deletion after the retained public surface is proven. |
| `starmap (root)` | `REJECT` | Donor root wiring combines the rejected schema with acquisition and publication changes. Preserve the current concrete immutable `Catalog()` DX; recompose lifecycle changes through P3/P6/P7. |

## Non-Go production and evidence groups

| Changed area | Status | Rationale and follow-up |
| --- | --- | --- |
| `.github/workflows/pr.yaml` action pins | `ALREADY_LANDED` | Protected main has newer reviewed checkout/setup-go pins through PR #47 and exact structural assertions. Donor pins must not be restored. |
| `.github/workflows/pr.yaml` single-cache-owner change | `SALVAGE` | The minimum-version setup owning the cache and the release setup using `cache: false` addresses duplicate extraction. Revalidate with current setup-go v7 and add exact structural proof in P10.1 if still beneficial. |
| `go.mod`, `go.sum`, `devbox.json`, `devbox.lock` | `REJECT` | The donor graph restores vulnerable/older modules and pulls cloud SDKs into the main module. Add only dependencies required by selected opt-in adapters after P6.2 enforces the read-library closure. |
| `internal/embedded/catalog/providers.yaml` and provider model YAML | `SALVAGE` | Provider/source descriptions, endpoint research, and missing model observations are candidate evidence. Re-fetch current dynamic facts, validate every record, preserve source provenance, and write only the one provider-YAML representation. |
| Donor embedded generation, authors/definitions/offerings, manifests, and hashes | `REJECT` | They materialize the rejected schema-v2 truth and are stale artifacts rather than current authoritative observations. |
| `internal/embedded/openapi` generated schema | `REJECT` | It documents the rejected persisted schema. Regenerate only after the final provider-YAML/read-view contract lands. |
| Provider raw response fixtures and testdata | `SALVAGE` | Keep only source-keyed, secret-free fixtures that prove a current wire delta, pagination, quarantine, or normalization rule; refresh or label stale fixtures. |
| Provider-specific research documents | `SALVAGE` | Primary-source URLs, API versions, auth/scope distinctions, and known limitations are useful review inputs. Re-verify temporally unstable claims before implementation. |
| Donor architecture/control-plane/ADR documents | `SUPERSEDED` | They remain available at the immutable donor commit as historical evidence. Current prescriptive architecture is `docs/STARMAP_ARCHITECTURE_CONTROL_PLANE.md`. |
| README, CONTRIBUTING, AGENTS, CLI/API docs | `SUPERSEDED` | Donor prose teaches the rejected schema and source composition. P10.5 rewrites current documentation after behavior is proven. |
| `scripts/refresh-provider-testdata.sh`, fixture Make targets | `SALVAGE` | Source-keyed, bounded, secret-safe refresh is useful after fixture ownership is decided. Never make network refresh part of ordinary tests. |
| `scripts/verify.sh` credential isolation | `SALVAGE` | Empty `HOME`, disabled metadata, dotenv isolation, and scrubbed cloud inputs are valuable CI safety. Replace YAML-regex discovery with a robust reviewed mechanism and prove no live credentials are used in P10.1/P10.6. |
| Other Makefile/script changes | `SUPERSEDED` | Targets tied to schema-v2 generation or rejected package names must be redesigned with their owning P3–P10 tasks. |
| Changelog and generated package READMEs | `SUPERSEDED` | They describe donor-only public surfaces. Regenerate from the final API instead of carrying them forward. |

## Salvage boundaries

The useful donor work forms four evidence queues, not four code drops:

1. **Provider acquisition research:** provider API versions, logical source
   topology, auth/scope distinctions, pagination, and raw fixtures. Owner:
   P4.7–P4.10 and P6.3.
2. **Shared connector and transport behavior:** OpenAI/Anthropic/Google wire
   reuse, bounded reads, configured-origin auth, redirect safety, secret-safe
   results, and record quarantine. Owner: P4.8, P6.2–P6.3, P8.3, P10.6.
3. **Observation and operator semantics:** status, completeness, issues,
   provider/model-scoped evidence, source-granular CLI status, and strict mode.
   Owner: P4.4–P4.10 and P10.5.
4. **Verification mechanics:** governed source-keyed fixtures, credential-free
   CI, and single-cache-owner evaluation. Owner: P10.1–P10.3.

Every queue must be refreshed from current protected main. None permits donor
definitions/offerings persistence, a compatibility layer, a root-library cloud
SDK dependency, or wholesale branch/file cherry-picking.

## Reproduce completeness

```bash
git diff --name-only \
  9508ee7866e4683e001e7ad153319d348433045d...\
a14d22497479cb944932274088cf806cb25e993b |
  awk '/\.go$/ && $0 !~ /_test\.go$/ {
    module=$0
    sub("/[^/]+$", "", module)
    if (module == $0) module="starmap (root)"
    print module
  }' |
  sort -u
```

Expected count: `66`.

The table was also compared against the direct donor-to-current-main tree, not
only the old merge base. No production Go module from the donor is
byte-identical to current main; the only already-landed group is the newer
workflow/dependency maintenance described above.

## Terminal disposition

PR #40 is a read-only historical donor. After this inventory is linked from the
PR:

- close the draft without claiming its review threads were resolved;
- remove the checked-out worktree before deleting its local branch;
- verify the remote branch is absent after deletion; and
- carry forward only the four named evidence queues through their owning
  control-plane tasks.

