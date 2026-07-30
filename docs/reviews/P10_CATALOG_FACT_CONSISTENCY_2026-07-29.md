# P10 Catalog Fact Consistency Review

Date: 2026-07-29 (America/Chicago)

Scope: checked-in authored-model and provider-serving YAML, the source
normalization and reconciliation paths that produce those records, canonical
catalog validation, and the generated embedded catalog.

## Outcome

The reviewed catalog has no mechanically provable contradiction in the four
high-signal classes covered by this review:

- lifecycle timestamps are ordered;
- a reasoning control or subordinate reasoning capability never contradicts
  the base reasoning capability;
- operational tags have the modalities required by their operation; and
- exact researched release facts are supported by primary vendor sources.

The review does not turn weak heuristics into validation rules. In particular,
a rolling model alias can outlive and route beyond its first release, so its
knowledge cutoff can be later than the alias's original release date.
`qwen-plus` is the two-view instance of that deliberate condition in the
current corpus. Unknown date precision remains zero/null; Starmap does not
invent the first day of a month.

## Authority and semantics

The source-of-truth rules used here are:

1. A provider's direct API or documentation is authoritative for its service
   identity, current price, availability, endpoint behavior, limits, and
   request controls.
2. An author/vendor model card, release post, or official repository is
   authoritative for intrinsic authorship, first release, weights, architecture,
   and model capabilities.
3. A concrete source control surface is stronger evidence than a contradictory
   summary boolean. For example, non-empty `reasoning_options` establishes
   reasoning support even when an upstream `reasoning` summary is false.
4. `created_at` and `updated_at` are ordered lifecycle evidence, not a claim
   that both timestamps came from the provider. They bound the earliest and
   latest known source observation or Starmap change. Zero means unknown.
5. `release_date` is the first known public release of the model identity. For
   a dated immutable snapshot, it is directly comparable with that snapshot's
   knowledge cutoff. For a rolling alias, the cutoff may advance after the
   alias's first release.

## Mechanical corpus audit

The pre-correction corpus contained:

| Invariant | Before | After |
| --- | ---: | ---: |
| `created_at > updated_at` | 165 | 0 |
| Reasoning controls/subfeatures with base reasoning false | 123 | 0 |
| Operational tag contradicting required modalities | 155 | 0 |
| Knowledge cutoff after release | 2 | 2 deliberate rolling-alias views |

The YAML diff was decoded structurally against the prior committed tree. All
382 changed source YAML files fall into the intended categories:

- 165 paired `created_at`/`updated_at` swaps, retaining both exact values;
- 123 `features.reasoning` corrections from false to true;
- 145 output-modality corrections and 10 input-modality additions;
- 16 researched release-date corrections;
- one removed meaningless Whisper knowledge cutoff;
- one Whisper open-weights correction;
- one GPT-4 Turbo description and one authoritative feature block.

No model identity, provider identity, price, limit, endpoint, request override,
or author membership changed in the mechanical repair.

## Researched corrections

Primary sources and resulting corrections:

- OpenAI's
  [GPT-4 Turbo model page](https://developers.openai.com/api/docs/models/gpt-4-turbo)
  establishes the `gpt-4-turbo` model identity/snapshot, April 9, 2024 release,
  December 1, 2023 knowledge cutoff, text/image input, text output, function
  calling and streaming support, and lack of structured-output support.
- OpenAI's
  [Whisper model card](https://github.com/openai/whisper/blob/main/model-card.md)
  and
  [large-v3 announcement](https://github.com/openai/whisper/discussions/1762)
  establish open weights, speech-recognition/audio input, and the November 6,
  2023 release. The Groq serving view no longer carries a fabricated
  knowledge-cutoff date.
- The official [Qwen3 release post](https://qwenlm.github.io/blog/qwen3/) and
  [Qwen3 repository](https://github.com/QwenLM/Qwen3) establish April 29, 2025
  for the reviewed Qwen3 base models.
- The official
  [Qwen3-Coder announcement](https://qwenlm.github.io/blog/qwen3-coder/)
  establishes July 22, 2025 for the 480B variant. The official
  [Qwen3-Coder 30B initial model commit](https://huggingface.co/Qwen/Qwen3-Coder-30B-A3B-Instruct/commit/87fefa3b0acba13e79fe3481ed808601c4e87e80)
  establishes July 31, 2025 for that identity.
- Alibaba Cloud's
  [Model Studio model and pricing documentation](https://www.alibabacloud.com/help/en/model-studio/model-pricing)
  documents rolling aliases as aliases for dated snapshots. This supports
  retaining the `qwen-plus` initial alias release separately from a later
  routed revision's knowledge cutoff.

## Prevention

Strict final-catalog construction now rejects:

- reversed non-zero lifecycle timestamps;
- reasoning controls or subordinate reasoning flags without base reasoning;
- embedding, speech-to-text, text-to-speech, image-generation, or
  video-generation tags without their required modalities.

Mutable builder observation remains tolerant so independently malformed source
records can still be quarantined or reconciled at the established boundaries.
Models.dev normalization treats a concrete reasoning control as direct
capability evidence. OpenAI-compatible provider normalization makes operational
tags and modalities coherent. New-record reconciliation orders mixed-source
lifecycle evidence before publication.

The existing embedded bootstrap build is the corpus-level regression: it
constructs every authored definition and provider offering through the strict
final publication path, then verifies the complete identity graph and exact
OpenRouter endpoint projection.

## Generated result

- Generation: `catalog-20260730T021502Z-e2d069200db0`
- Semantic checksum:
  `sha256:e2d069200db0beed420c78a248fa6351c4452480d6fbe3321e47c9cc59c34799`
- Payload checksum:
  `sha256:ffa21fc703df369b80f012353559f417aab7b6eb2147a2220b4aa266916ecf74`
- Payload size: `2,604,102` bytes

The generation and `endpoints.yaml` projection were regenerated from the
checked-in source YAML. No live provider refresh or release publication was
performed.

## Verification

Verification passed:

- `go test ./pkg/catalogs ./internal/sources/modelsdev ./internal/providers/openai ./internal/catalog/reconciler -count=1`
- `go run ./cmd/starmap validate catalog` — 11 providers, 104 authors, 610
  models, and all cross-references valid
- `go test ./cmd/starmap-bootstrap-manifest ./internal/bootstrap ./internal/embeddedbudget -count=1`
- the race-enabled catalog-fact matrix passed for catalogs (`46.866s`),
  models.dev (`70.082s`), OpenAI-compatible normalization (`8.060s`),
  reconciliation (`1.399s`), bootstrap (`59.208s`), bootstrap manifest
  (`45.201s`), and embedded budget (`57.795s`)
- the complete ordinary repository suite passed with root `54.374s`,
  acquisition `17.950s`, internal server `30.691s`, catalogs `22.905s`,
  models.dev `13.339s`, remote `9.942s`, and every package green
- `go vet ./...`, generated GoDoc/OpenAPI, and `make docs-check`
- `git diff --check`
