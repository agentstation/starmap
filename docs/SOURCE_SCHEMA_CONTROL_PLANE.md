# Starmap Source Schema Control Plane

Last updated: 2026-07-09

This document is the durable control plane for closing Starmap source-field and schema gaps. It is written so a future agent can resume after compaction without relying on chat history.

## Mission

Make Starmap a source-complete AI model catalog. Every attribute from every catalog source must have one explicit outcome:

- mapped into canonical Starmap schema;
- mapped into a controlled provider/source extension field;
- intentionally ignored with a documented reason and a regression test.

The goal is not to mirror every provider's JSON verbatim. The goal is canonical, queryable, reliable Starmap data with no silent source attrition.

## Repository

Path: `/Users/jack/src/github.com/agentstation/starmap`

Current relevant implementation seams:

- Canonical model schema: `pkg/catalogs/model.go`
- Pricing schema: `pkg/catalogs/model_pricing.go`
- models.dev parser: `internal/sources/modelsdev/parser.go`
- OpenAI-compatible provider client: `internal/providers/openai/client.go`
- Anthropic provider client: `internal/providers/anthropic/client.go`
- Google provider client: `internal/providers/google/client.go`
- Provider source fan-out: `internal/sources/providers/providers.go`
- Reconciliation field rules: `pkg/reconciler/field_rules.go`
- Reconciliation complex merge logic: `pkg/reconciler/merger.go`
- Diff support: `pkg/differ/differ.go`
- Copy boundaries: `pkg/catalogs/copy.go`
- Verification model: `docs/TESTING.md`

## Status Legend

- `DONE`: Acceptance criteria satisfied and verified.
- `IN_PROGRESS`: Active implementation or review is underway.
- `PENDING`: Not started.
- `BLOCKED`: Cannot proceed without user input or external state.
- `DEFERRED`: Explicitly postponed because a higher-priority phase must land first.

## Live Audit Baseline

Audit date: 2026-07-09.

models.dev live baseline from `https://models.dev/api.json`:

- Providers: 153
- Models: 5,380
- Models with `cost`: 4,983
- Models with descriptions: 5,380
- Models with structured output: 1,987 to 2,682 depending on null/false/value counting
- Models with reasoning options: 2,113 to 3,283 depending on type/value counting
- Models with tiered or context-threshold pricing: 193

Live provider list endpoints were reachable for:

- OpenAI
- Anthropic
- Google AI Studio
- Groq
- DeepSeek
- Cerebras
- Alibaba Model Studio / DashScope-compatible endpoint
- Fireworks AI
- DeepInfra
- Moonshot AI

Secrets must not be persisted in this document, test fixtures, logs, or generated reports.

## Phase Ledger

| ID | Phase | Status | Scope | Acceptance Gate |
| --- | --- | --- | --- | --- |
| P0 | Control plane and goal setup | DONE | Persist this control plane, review it, create the active goal, and start P1 | This file exists, has ledgers, acceptance criteria, execution log, and `/goal` prompt; active goal is created |
| P1 | Source shape coverage harness | DONE | Add deterministic tests that enumerate known source attributes and fail on unclassified drift | Focused shape tests pass; each current source path is mapped, extension-preserved, or explicitly ignored |
| P2 | Canonical schema foundation | DONE | Add missing Starmap schema for source-complete limits, lifecycle, lineage, tiered pricing, modes, provider invocation metadata, and controlled raw extensions | Schema fields round-trip through JSON/YAML, copy, diff, validation, docs, and zero-value behavior tests |
| P3 | models.dev full-field mapping | DONE | Parse and map all current models.dev provider/model/cost/limit/capability/experimental attributes | models.dev parser coverage proves 100% of current live scalar paths are classified and expected fields survive conversion |
| P4 | Provider API full-field mapping | DONE | Parse and map every reachable provider list endpoint field by provider family | Provider fixture tests prove every known provider response path is classified and important fields survive conversion |
| P5 | Reconciliation and authority completion | DONE | Ensure new fields survive merge with explicit source authority and provenance | Multi-source integration tests prove no new field is dropped during reconciliation |
| P6 | Output, query, and docs completion | DONE | Expose new fields through YAML, CLI/server query surfaces where appropriate, and architecture/testing docs | Catalog output and relevant query tests pass; docs describe source completeness policy |
| P7 | Enterprise verification and closeout | DONE | Run full deterministic gates, live opt-in checks where credentials are available, and autoreview if requested | `make verify` or documented equivalent passes; focused live checks are logged separately from deterministic tests |

## Attribute Ledger

| Source | Attribute(s) | Current Status | Target Outcome | Verification |
| --- | --- | --- | --- | --- |
| models.dev provider | `id`, `name`, `models` | mapped | keep canonical mapping | parser/provider conversion tests |
| models.dev provider | `doc`, `env`, `api`, `npm` | parsed but dropped by `ToStarmapProvider` | map to provider catalog docs/env/API/SDK metadata or extension field | provider conversion tests assert fields survive |
| models.dev model identity | `id`, `name`, `description` | mapped | keep canonical mapping | parser conversion tests |
| models.dev lifecycle | `release_date`, `last_updated`, `status` | dates mapped, `status` unparsed | add canonical model status/lifecycle | parser conversion and YAML round-trip tests |
| models.dev lineage | `family` | parsed but dropped | add canonical family/lineage field | parser conversion, diff, copy tests |
| models.dev metadata | `open_weights`, `knowledge` | mapped | keep canonical mapping | parser conversion tests |
| models.dev capabilities | `attachment`, `reasoning`, `reasoning_options`, `structured_output`, `temperature`, `tool_call` | mostly mapped | map `reasoning_options.min/max` and non-effort option types where canonical | parser conversion tests |
| models.dev modalities | `modalities.input[]`, `modalities.output[]` | mapped for known modalities | keep; fail on unknown modalities unless classified | shape contract test |
| models.dev limits | `limit.context`, `limit.output`, `limit.input` | context/output mapped, input dropped | add `limits.input_tokens` | parser conversion, reconciler, YAML tests |
| models.dev base pricing | `cost.input`, `cost.output`, `cost.reasoning`, `cost.cache`, `cost.cache_read`, `cost.cache_write`, `cost.input_audio`, `cost.output_audio` | mapped | keep canonical mapping | parser conversion tests |
| models.dev tiered pricing | `cost.tiers[]`, `cost.context_over_200k` | unparsed | add conditional/tiered pricing schema | parser conversion and YAML tests |
| models.dev model provider overrides | `provider.npm`, `provider.api`, `provider.shape` | unparsed | add per-model provider invocation metadata or extension | parser conversion tests |
| models.dev interleaved reasoning | `interleaved`, `interleaved.field` | unparsed | add response/reasoning delivery metadata | parser conversion tests |
| models.dev experimental modes | `experimental.modes.fast.cost.*`, `experimental.modes.fast.provider.*` | unparsed | add mode-specific pricing and request override schema | parser conversion tests |
| OpenAI-compatible common | `id`, `object`, `created`, `owned_by`, `root`, `parent` | id/authors mapped; root/parent mostly dropped | map lineage and created time where meaningful; classify `object` | provider client tests |
| OpenAI-compatible configured fields | `context_window`, `max_completion_tokens`, `metadata.description`, `metadata.context_length`, `metadata.tags` | partially mapped by allow-list | keep and expand allow-list with schema-backed destinations | provider client tests |
| Anthropic | `max_tokens`, `max_input_tokens` | live endpoint returns; client drops | map to output/input/context limits | Anthropic fixture test |
| Anthropic | `capabilities.*` | live endpoint returns; client drops | map image/PDF input, structured output, thinking, effort, citations, batch, code execution, context management | Anthropic fixture test |
| Google | `inputTokenLimit`, `outputTokenLimit`, `supportedGenerationMethods` | mapped partially | keep and add input-token limit | Google conversion tests |
| Google | `version`, `temperature`, `topP`, `topK`, `maxTemperature`, `thinking` | dropped or heuristic | map version/generation ranges/reasoning | Google conversion tests |
| Groq | `context_window`, `context_length`, `max_completion_tokens`, `max_output_length` | partially mapped | map all relevant limits with precedence | OpenAI-compatible/Groq tests |
| Groq | `input_modalities[]`, `output_modalities[]`, `supported_features[]`, `supported_sampling_parameters[]` | dropped | map modalities/features/generation controls | Groq fixture tests |
| Groq | `pricing.*` | dropped | map provider pricing where models.dev lacks value or preserve as provider-reported pricing | Groq fixture tests |
| Fireworks | `context_length`, `supports_tools`, `supports_chat`, `supports_image_input`, `kind` | dropped | map limits/features/model kind | Fireworks fixture tests |
| DeepInfra | `metadata.pricing.*` | dropped | map token/media pricing | DeepInfra fixture tests |
| DeepInfra | `metadata.default_width`, `metadata.default_height`, `metadata.default_iterations` | dropped | map image/media defaults or extension field | DeepInfra fixture tests |
| Moonshot | `context_length`, `supports_image_in`, `supports_video_in`, `supports_reasoning` | dropped | map limits/modalities/reasoning | Moonshot fixture tests |
| Moonshot | `permission[]`, `root`, `parent` | dropped | map lineage where useful; classify permissions as extension or ignored | Moonshot fixture tests |
| Local/embedded catalog | all canonical fields | current authoritative override source | update schema, YAML comments, validation, docs | catalog round-trip and validation tests |

## Phase Details

### P0: Control Plane and Goal Setup

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P0.1 | Write dedicated control-plane doc | DONE | `docs/SOURCE_SCHEMA_CONTROL_PLANE.md` exists with mission, ledgers, acceptance criteria, and execution log |
| P0.2 | Review plan against audit findings | DONE | Attribute ledger includes models.dev and all reachable provider families audited on 2026-07-09 |
| P0.3 | Create active goal | DONE | Codex active goal references this file as source of truth |
| P0.4 | Start implementation | DONE | P1 has a concrete code/test change and execution log entry |

### P1: Source Shape Coverage Harness

Build a deterministic harness that records source paths and classifies each path as canonical, extension, or intentionally ignored.

Tasks:

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P1.1 | Add path-classification helpers for JSON fixtures | DONE | Tests can normalize nested JSON paths with array indices collapsed to `[]` |
| P1.2 | Add models.dev current-shape classification test | DONE | Test fixture includes all current models.dev paths from audit or representative shape; every path is classified |
| P1.3 | Add provider fixture classification tests | DONE | Provider response families are covered with minimal fixtures without secrets |
| P1.4 | Add drift workflow | DONE | Docs explain how to refresh live shape reports without committing secrets |

Acceptance gate:

```bash
go test ./internal/sources/modelsdev ./internal/providers/...
go test ./...
```

Live shape refresh workflow:

1. Save live payloads only under `/tmp`, never under the repository.
2. Load credentials from `.env` without printing values.
3. Print normalized path/count summaries, not raw payloads.
4. Update the deterministic source-shape fixtures and classification maps only after deciding whether each new path is canonical, extension-preserved, or intentionally ignored.

Safe command pattern:

```bash
bash -lc 'set -a; [ -f .env ] && . ./.env; set +a; curl -fsS https://models.dev/api.json -o /tmp/starmap-models-dev-api-current.json'
jq -r 'def normpath: map(if type=="number" then "[]" else tostring end) | join("."); [to_entries[] | select((.value|type)=="object" and (.value.models != null)) | .value.models | to_entries[] | .value] | .[] | paths(scalars) as $p | ($p|normpath)' /tmp/starmap-models-dev-api-current.json | sort | uniq -c | sort -nr
```

### P2: Canonical Schema Foundation

Add the minimum schema needed before broad parser/client changes.

Tasks:

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P2.1 | Add `ModelLimits.InputTokens` | DONE | JSON/YAML tags, copy, diff, reconciler provenance, parser mapping, and tests exist |
| P2.2 | Add model lifecycle/status | DONE | Status enum supports active/beta/preview/deprecated/unknown and round-trips |
| P2.3 | Add model family/lineage | DONE | models.dev `family`, provider `root`, and provider `parent` have a canonical destination |
| P2.4 | Add tiered pricing schema | DONE | Context-threshold and generic tier pricing can represent `cost.tiers[]` and `context_over_200k` |
| P2.5 | Add mode-specific pricing/request overrides | DONE | `experimental.modes.fast` can be represented without raw JSON loss |
| P2.6 | Add controlled source extension bucket | DONE | Unknown-but-preserved fields are copied deeply, marshaled predictably, and excluded from authority confusion |

Acceptance gate:

```bash
go test ./pkg/catalogs ./pkg/differ ./pkg/reconciler ./pkg/authority
go test ./...
go vet ./...
```

### P3: models.dev Full-Field Mapping

Tasks:

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P3.1 | Extend parser structs for all current paths | DONE | No live audited models.dev path is unparsed unless explicitly ignored |
| P3.2 | Map provider-level metadata | DONE | `doc`, `env`, `api`, and `npm` survive provider conversion |
| P3.3 | Map new model metadata and limits | DONE | `family`, `status`, `limit.input`, and reasoning option ranges survive model conversion |
| P3.4 | Map pricing tiers and modes | DONE | Tier/context/mode pricing survives model conversion and YAML output |
| P3.5 | Add fixture coverage for edge shapes | DONE | Tests cover explicit zero prices, null reasoning values, tiered pricing, interleaved fields, and experimental fast mode |

Acceptance gate:

```bash
go test ./internal/sources/modelsdev
go test ./pkg/catalogs ./pkg/reconciler
go test ./...
```

### P4: Provider API Full-Field Mapping

Tasks:

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P4.1 | Expand OpenAI-compatible response model | DONE | Known provider-specific fields are typed and unknown fields are classified |
| P4.2 | Map Anthropic live capabilities | DONE | Token limits and `capabilities.*` survive conversion |
| P4.3 | Map Google generation and thinking fields | DONE | Version, generation ranges, limits, and thinking metadata survive conversion |
| P4.4 | Map Groq capabilities/pricing | DONE | Modalities, supported features, supported sampling parameters, limits, and pricing survive conversion |
| P4.5 | Map Fireworks fields | DONE | Context length and support flags survive conversion |
| P4.6 | Map DeepInfra pricing/media metadata | DONE | Token/media pricing and media defaults survive conversion |
| P4.7 | Map Moonshot capabilities/lineage | DONE | Context length, modality support, reasoning support, root/parent survive conversion |
| P4.8 | Classify OpenAI/Alibaba/Cerebras/DeepSeek common fields | DONE | Common fields are mapped or intentionally ignored with tests |

Acceptance gate:

```bash
go test ./internal/providers/...
go test ./internal/sources/providers
go test ./...
```

### P5: Reconciliation and Authority Completion

Tasks:

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P5.1 | Extend field rule catalog | DONE | New fields have authority/provenance rules |
| P5.2 | Update complex merge behavior | DONE | Limits, pricing, metadata, extensions, lineage, status, modes, and provider invocation metadata survive merge |
| P5.3 | Add multi-source integration tests | DONE | Tests prove models.dev, provider APIs, and local catalog fields merge without unwanted loss |
| P5.4 | Validate no pointer alias regressions | DONE | Copy and race tests cover new nested slices/maps/pointers |

Acceptance gate:

```bash
go test ./pkg/authority ./pkg/reconciler ./pkg/catalogs -race
go test ./...
```

### P6: Output, Query, and Docs Completion

Tasks:

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P6.1 | Update YAML formatting/comments | DONE | New sections are readable and zero values do not create noisy output |
| P6.2 | Update CLI/server query surfaces where useful | DONE | Users can inspect status, lineage, input limits, and pricing tiers through existing detail/list surfaces as appropriate |
| P6.3 | Update architecture/testing docs | DONE | Docs describe source completeness, classification tests, and live refresh workflow |
| P6.4 | Update generated package docs | DONE | `make docs-check` passes |

Acceptance gate:

```bash
go test ./cmd/starmap/... ./internal/server/... ./internal/catalog/query
make docs-check
go test ./...
```

### P7: Enterprise Verification and Closeout

Tasks:

| ID | Task | Status | Acceptance Criteria |
| --- | --- | --- | --- |
| P7.1 | Run deterministic gates | DONE | `make verify` passes or exact environment blocker is documented with equivalent command results |
| P7.2 | Run live opt-in provider checks | DONE | Live checks use existing env vars without printing secrets; results are logged separately from deterministic gates |
| P7.3 | Run autoreview if requested or before final ship | DONE | Actionable findings are fixed or explicitly rejected with evidence |
| P7.4 | Final ledger update | DONE | This file records final status, verification commands, and remaining risks |

Acceptance gate:

```bash
make verify
starmap providers --test
```

`starmap providers --test` is a live optional gate and must not block deterministic correctness when credentials or provider services are unavailable.

## `/goal` Prompt

Use this as the active goal prompt:

```text
/goal Execute docs/SOURCE_SCHEMA_CONTROL_PLANE.md to completion. Treat that file as the durable source of truth. Work through the phase ledger in order unless a later task is required to unblock the current phase. After every meaningful code or test change, update the execution log and task statuses. Preserve unrelated dirty work. Do not print or persist secrets. For every source attribute, ensure it is mapped into canonical schema, preserved in a controlled extension, or intentionally ignored with a documented reason and regression coverage. Run focused gates after each phase and deterministic enterprise gates before closeout. Continue autonomously until all phases are DONE or a real blocker is recorded with exact evidence.
```

## Execution Log

| Time | Entry |
| --- | --- |
| 2026-07-09 America/Chicago | Source/schema audit completed from local code plus live models.dev/provider endpoint shapes. Major gaps: `models.dev` tiered pricing/status/interleaved/experimental/provider metadata, `limit.input`, provider capability fields, provider pricing fields, lineage/root/parent, and reconciliation support for new schema. |
| 2026-07-09 America/Chicago | Dedicated control-plane document created at `docs/SOURCE_SCHEMA_CONTROL_PLANE.md`. P0.1 and P0.2 marked `DONE`; P0.3 and P0.4 pending active goal creation and first implementation slice. |
| 2026-07-09 11:30 CDT | Active goal created from this document's `/goal` prompt. P1 started with `internal/sources/modelsdev/source_shape_test.go`, a deterministic path-classification harness for representative models.dev provider/model/cost/limit/capability/experimental shapes. Focused gate passed: `go test ./internal/sources/modelsdev`. |
| 2026-07-09 11:32 CDT | P1.3 started with `internal/providers/openai/source_shape_test.go`, a deterministic path-classification harness for OpenAI-compatible provider families covering common OpenAI fields plus Groq, DeepInfra, Fireworks, and Moonshot-style extensions. Focused gate passed: `go test ./internal/providers/openai ./internal/providers/groq ./internal/providers/moonshot-ai ./internal/providers/cerebras ./internal/providers/deepseek`. |
| 2026-07-09 America/Chicago | P1.3 and P1.4 implementation completed with Anthropic and Google source-shape classification tests plus a documented live shape refresh workflow. Focused gate passed: `go test ./internal/providers/...`. P1 final gate is pending. |
| 2026-07-09 11:33 CDT | P1 completed. Acceptance gates passed: `go test ./internal/sources/modelsdev ./internal/providers/...` and `go test ./...`. P2 started with `ModelLimits.InputTokens`. |
| 2026-07-09 11:35 CDT | P2.1 completed. Added `limits.input_tokens` to canonical schema, models.dev conversion, OpenAI-compatible field mappings, Google conversion, validation, diffing, and reconciler provenance/merge logic. Gates passed: `go test ./pkg/catalogs ./internal/sources/modelsdev ./internal/providers/openai ./internal/providers/google ./pkg/differ ./pkg/reconciler ./pkg/authority ./cmd/starmap/cmd/validate`, `go test ./...`, and `go vet ./...`. |
| 2026-07-09 11:38 CDT | P2.2 completed. Added top-level `ModelStatus` with active/beta/preview/deprecated/unknown states, models.dev status conversion, authority/reconciler field rules, diff support, YAML coverage, and merge tests. Gates passed: `go test ./pkg/catalogs ./internal/sources/modelsdev ./pkg/differ ./pkg/reconciler ./pkg/authority`, `go test ./...`, and `go vet ./...`. P2.3 started for model family/lineage. |
| 2026-07-09 11:40 CDT | P2.3 completed. Added canonical `ModelLineage` with family/root/parent, deep-copy support, models.dev family mapping, OpenAI-compatible root/parent mapping, diff support, authority/reconciler field rules, and complex merge logic that combines models.dev family with provider root/parent. Gates passed: `go test ./pkg/catalogs ./internal/sources/modelsdev ./internal/providers/openai ./pkg/differ ./pkg/reconciler ./pkg/authority`, `go test ./...`, and `go vet ./...`. P2.4 started for tiered pricing. |
| 2026-07-09 11:43 CDT | P2.4 completed. Added canonical pricing tiers with type/size/name, token/operation tier pricing, deep-copy support, models.dev `cost.tiers[]` and `cost.context_over_200k` conversion, diff coverage, YAML coverage, and parser tests. Gates passed: `go test ./pkg/catalogs ./internal/sources/modelsdev ./pkg/differ ./pkg/reconciler ./pkg/authority`, `go test ./...`, and `go vet ./...`. P2.5 started for mode-specific pricing/request overrides. |
| 2026-07-09 11:48 CDT | P2.5 completed. Added canonical `Model.Modes` with mode-specific pricing and provider request overrides, models.dev `experimental.modes.*` conversion, deep-copy support for nested maps/pricing, diff detection, models.dev authority/provenance for `modes`, YAML coverage, and merge regression coverage. Gates passed: `go test ./pkg/catalogs ./internal/sources/modelsdev ./pkg/differ ./pkg/reconciler ./pkg/authority`, `go test ./...`, and `go vet ./...`. P2.6 started for controlled source extensions. |
| 2026-07-09 11:51 CDT | P2.6 completed. Added canonical `SourceExtensions` buckets on models and providers for controlled non-canonical source fields, deep-copy support, JSON/YAML round-trip coverage, model/provider YAML assertions, diff detection, and a reconciler guard proving extensions stay out of authority field rules. Gates passed: `go test ./pkg/catalogs ./pkg/differ ./pkg/reconciler ./pkg/authority`, `go test ./...`, and `go vet ./...`. P3 started for full models.dev field mapping. |
| 2026-07-09 11:55 CDT | P3 completed. models.dev provider metadata now maps through direct conversion and the real `processFetch` path: `doc` to catalog docs, `api` to endpoint URL, `env` to provider env vars, and `npm` to controlled source extensions. Model conversion now maps reasoning token bounds to `ReasoningTokens` and preserves model-level provider overrides plus interleaved metadata in `extensions.models.dev`. Source-shape classifications were updated for extension-backed fields. Gates passed: `go test ./internal/sources/modelsdev ./pkg/catalogs ./pkg/reconciler`, `go test ./...`, and `go vet ./...`. P4 started for provider API field mapping. |
| 2026-07-09 12:03 CDT | P4 completed. OpenAI-compatible provider conversion now maps common and provider-specific limits, names, creation times, active status, modalities, feature/sampling capability lists, provider pricing, metadata pricing, Fireworks/Moonshot support flags, lineage, and controlled extensions for provider-only fields such as `hugging_face_id`, media defaults, kind, permissions, and raw support flags. Anthropic now maps token limits and capabilities, preserving non-canonical variants in extensions. Google AI Studio now has a raw REST list path before SDK fallback so version, supported generation methods, generation ranges, and thinking survive conversion. Gates passed: `go test ./internal/providers/... ./internal/sources/providers`, `go test ./...`, and `go vet ./...`. P5 started for reconciliation and authority completion. |
| 2026-07-09 12:05 CDT | P5 completed. Reconciler now explicitly merges controlled source extensions for models and providers while keeping them out of authority field rules; local extension fields win conflicts and other sources fill missing extension keys. Added multi-source tests for model/provider extension merging. Gates passed: `go test ./pkg/authority ./pkg/reconciler ./pkg/catalogs -race`, `go test ./...`, and `go vet ./...`. P6 started for output, query, and docs completion. |
| 2026-07-09 12:10 CDT | P6 completed. Query and HTTP params now expose status and input token range filters, fail closed when required nested data is absent, and have parser/query regression coverage. CLI model details now surface status, lineage, max input tokens, pricing tier counts, mode names, and extension source names with helper-level tests. Architecture/testing docs now describe source completeness, source-shape classification, and live refresh rules. Generated package docs were refreshed. Gates passed: `go test ./cmd/starmap/... ./internal/server/... ./internal/catalog/query`, `make docs-check`, `go test ./...`, and `go vet ./...`. P7 started for deterministic enterprise verification and closeout review. |
| 2026-07-09 16:37 CDT | P7 review remediation completed for source-attribution and provider-scope issues. Fixes covered optional provider env hints not gating provider fetches, provider-scoped reconciliation provenance and enhanced counts, active-state fallback for provider `active:false`, provider canonical-field diffs for environment variables and policies, models.dev environment-variable authority, dotted model IDs in provenance-backed counts, POST search release-date mapping, limit overlays, additive models.dev enrichment semantics, provider-scoped baseline model indexing, same-ID cross-provider enrichment prevention, Google AI Studio empty REST fallback, OpenAI-scoped system author attribution, pricing authority behavior, source extension ownership, provider-scoped change grouping, and typed-nil field mapping. |
| 2026-07-09 16:37 CDT | P7 structured-review findings were fixed and covered with regressions: provider-filter `SetProvider` failures now propagate instead of silently dropping source data; models.dev `open_weights` is parsed presence-aware and metadata merge preserves known `true` values; Groq and DeepInfra pricing fixtures pin token prices as USD per 1M tokens; source extensions are normalized for JSON/YAML-stable dynamic types so sync/diff/reconcile logic does not churn on type-only reload changes. Focused gate passed: `go test ./pkg/catalogs ./internal/sources/modelsdev ./internal/providers/openai ./pkg/differ ./pkg/reconciler`. |
| 2026-07-09 16:37 CDT | P7.1 deterministic enterprise gate passed with `make verify`: full tests, race short tests, `go vet`, `golangci-lint`, package coverage thresholds, `make docs-check`, `git diff --check`, build, catalog validation, and CLI smoke checks. P7.2 live provider gate passed without printing secrets via redacted `go run ./cmd/starmap providers --test`: configured providers succeeded for Alibaba, Anthropic, Cerebras, DeepSeek, Fireworks AI, Google AI Studio, Groq, Moonshot AI, and OpenAI; DeepInfra was skipped because no credentials were configured; Google Vertex was skipped because no project was configured. |
| 2026-07-09 16:37 CDT | P7.3 final structured autoreview retry was queued after the Claude session-limit reset, but was stopped before execution to avoid consuming additional Claude credits. This retry was not required for closure because multiple earlier autoreview findings had already been investigated and fixed, focused regressions passed, and current-tree `make verify` passed. |
| 2026-07-09 16:46 CDT | P7 and the source-schema control plane are complete. Remaining risk is limited to the intentionally skipped final clean autoreview rerun; deterministic enterprise verification and focused review-regression gates are green, and live provider checks were logged separately from deterministic correctness. |
