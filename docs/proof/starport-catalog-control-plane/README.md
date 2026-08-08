# Starmap Catalog Control Plane Proof

This directory archives proof for the completed Starmap catalog control plane.
All 130 tasks and all 108 findings reached a terminal `DONE` state before plan
removal.

## Current baseline

- Repository baseline: `main@9c5c3175b6a03a1259147e40af15d4ba9d6e84b7`.
- Application release: `v0.3.0` is stable and immutable with 14 assets.
- Release recovery run: <https://github.com/agentstation/starmap/actions/runs/30881177476>.
- Fresh Homebrew recovery job: <https://github.com/agentstation/starmap/actions/runs/30881177476/job/91902984302>.

## P12.9 fail-before evidence

- Hosted catalog-generation run: <https://github.com/agentstation/starmap/actions/runs/31239182604>.
- Hosted source payload: 181 providers, 6,227 accepted models, four rejected
  records, and 3,612,245 bytes.
- Hosted reconciliation detected 115 additions and 10 updates, then exited 1:

```text
failed to publish catalog candidate: failed to index catalog read views:
validation failed for field provider_model.model: explicit canonical
author/model reference is required
```

An isolated local run copied `internal/embedded/catalog` into a temporary
directory and bound all catalog, state, store, report, and home paths to that
directory. It downloaded the current models.dev payload, detected 115 additions,
and reproduced the same exit-1 validation failure. The repository worktree
remained unchanged.

The accepted architecture keeps final catalog validation strict. Source
observations can preserve unresolved provider records. Reconciliation must
quarantine only offerings that still lack a canonical model reference after
authority and baseline resolution. Reconciliation must emit a typed,
deterministic issue for each provider/model subject. It must not infer
authorship from the serving provider.

The first invalid live subject was `azure-openai/claude-fable-5`. Provider
metadata supplied the raw offering, while the provider-specific model result
was empty after source filtering. The old composition path retained that raw
map because it replaced provider models only for a nonempty result.

## P12.9 local recovery evidence

Reconciliation now applies a final record-local resolution pass after it
composes all providers. It preserves an explicit current authored-model
reference, carries an exact valid baseline reference, or quarantines only the
unresolved provider/model record and its provenance. It emits a sorted typed
`unresolved_model_reference` issue with the exact provider and opaque model ID.
Malformed nonempty references still fail validation. No path infers an author
from the serving provider.

The first fixed live run exposed a second fail-closed boundary after commit.
Human-workspace YAML did not reproduce immutable payload bytes. Quoted model
descriptions and typed provenance values caused the difference. Regressions now cover quoted scalar
decoding, JSON/YAML price spelling, capability presence, nil rejection slices,
and scientific numeric values. Existing bootstrap JSON stays byte-stable, and
source identity survives a save and reload.

The final isolated live run used a detached temporary worktree, empty
credentials, and private home, state, generation-store, and report paths. It
completed with this evidence:

- The source included 181 providers and 6,228 accepted models. It rejected four
  source records.
- Reconciliation quarantined exactly 115 unresolved offerings from Azure
  OpenAI and Mistral.
- It retained 611 valid published offerings and applied ten valid updates. It
  added no unresolved offering.
- The generation ID was `efc4403d-5867-46e6-ad7e-75e62ca660b0`.
- The payload checksum was
  `sha256:18458cff139ba774beda565d8bbc0e2a0776dc59bdd1d2e992b80a700a95f5ba`.
- The semantic checksum was
  `sha256:e58d574ff2cf72410872523d693bb4d49dd6c534d73bff4be5722128698750a6`.
- The run regenerated the bootstrap manifest. Strict provider, author, model,
  and cross-reference validation passed.

## P12.9 local gate evidence

Release-candidate commit `2d273b0e` passes:

- `GOTOOLCHAIN=go1.25.12 go test ./...` passed.
- `GOTOOLCHAIN=go1.26.5 make verify` passed. It covered unit and external
  consumer contracts, pure-Go execution, race, vet, lint, coverage, docs,
  build, catalog validation, and CLI smoke.
- `BenchmarkClientCatalog` measured 8.591-9.178 ns/op with 0 B/op and 0
  allocs/op.
- Strict validation passed for 14 providers, 104 authors, 611 models, and all
  cross-references.
- `GOTOOLCHAIN=go1.26.5 make release-check` passed. It covered tests, vet, CLI
  generation, and GoReleaser configuration validation.
- `actionlint` and `git diff --check` passed.

## P12.10 protected merge evidence

- Pull request: <https://github.com/agentstation/starmap/pull/66>.
- Reviewed head: `a6a4db05de4da74f1ba4a6401edac838f854633b`.
- Protected-main merge: `f4a296e401afab1ce6b629940dd4cc92878c10e0`.
- Exact-head verification run:
  <https://github.com/agentstation/starmap/actions/runs/31280546982>.
- Verification Gate job `93161007919` passed in 21 minutes 45 seconds.
- Security & Reliability job `93161007941` passed in 2 minutes 7 seconds.
- Action Pin Provenance job `93161007971` passed.
- The pre-pull-request autoreview used `gpt-5.6-sol` at high reasoning effort.
  It found no actionable issue and rated the patch correct with 0.87
  probability.

## P12.10 hosted generation evidence

Hosted catalog-generation run
<https://github.com/agentstation/starmap/actions/runs/31281436199> ran from the
exact protected-main merge and passed in 2 minutes 49 seconds. It downloaded
3,612,744 bytes from models.dev and accepted 6,228 records from 181 providers.
It rejected four source records, quarantined exactly 115 unresolved Azure
OpenAI and Mistral offerings, applied ten valid updates, and added or removed
no offering. Strict validation passed for 14 providers, 104 authors, 611 model
records, and all cross-references.

The hosted generation has these identities:

- Generation ID: `c0360240-840b-4d6f-8475-eb83c0e26340`.
- Semantic checksum:
  `sha256:2d81c90d8e17152401d25f90e78667166436d7857583176ae36b968eadb3d837`.
- Payload checksum:
  `sha256:3d37200dee62ae5774dec11955b2718666328b832cb14c3bab6b4bad63d540ab`.
- Uncompressed payload size: 8,267,345 bytes.
- Compressed payload size in the generation budget: 306,978 bytes.

The run published the immutable prerelease
[`catalog-semantic-2d81c90d8e17152401d25f90e78667166436d7857583176ae36b968eadb3d837`](https://github.com/agentstation/starmap/releases/tag/catalog-semantic-2d81c90d8e17152401d25f90e78667166436d7857583176ae36b968eadb3d837).
GitHub API readback reports `immutable: true`. The downloaded archive passed
its published SHA-256 check. `starmap-catalog-release --verify-dir` reproduced
the generation, semantic, payload, and archive identities. GitHub attestation
verification passed with the catalog-generation workflow as the required
signer and self-hosted runners denied.

The workflow had no earlier `catalog-semantic-*` release to read back. The
latest earlier immutable release uses schema 1 and the historical
`catalog-payload-*` identity. Independent verification proved that release at
its declared schema contract. The public archive matched its published hash,
and the extracted payload matched its manifest. The GitHub API reported the
release as immutable, and its GitHub attestation passed.

The current schema-3 verifier correctly rejects the schema-1 `auth_required`
field. We added no legacy compatibility path.

## Closeout

P12.9, P12.10, F-107, and F-108 are `DONE`. At closeout, all 130 tasks and all
108 findings were terminal and no open implementation item remained. This
change affects catalog generation only. It does not require an application
release or a Starport dependency update. This proof replaces the completed
durable plan as the historical record.
