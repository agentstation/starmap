# Starmap Catalog Control Plane Proof

This directory is the durable proof root for the active control plane in
`docs/STARPORT_CATALOG_CONTROL_PLANE.md`.

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
authority and baseline resolution. The quarantine must be typed, deterministic,
and visible by provider/model subject. It must not infer authorship from the
serving provider.

The first invalid live subject was `azure-openai/claude-fable-5`. Provider
metadata supplied the raw offering, while the provider-specific model result
was empty after source filtering. The old composition path retained that raw
map because it replaced provider models only for a nonempty result.

## P12.9 local recovery evidence

Reconciliation now applies a final record-local resolution pass after all
providers are composed. It preserves an explicit current authored-model
reference, carries an exact valid baseline reference, or quarantines only the
unresolved provider/model record and its provenance. It emits a sorted typed
`unresolved_model_reference` issue with the exact provider and opaque model ID.
Malformed nonempty references still fail validation. No path infers an author
from the serving provider.

The first fixed live run exposed a second fail-closed boundary after commit:
human-workspace YAML did not reproduce immutable payload bytes for quoted model
descriptions and typed provenance values. Regressions now cover quoted scalar
decoding, JSON/YAML price spelling, capability presence, nil rejection slices,
and scientific numeric values. Existing bootstrap JSON stays byte-stable, and
source identity survives a save and reload.

The final isolated live run used a detached temporary worktree, empty
credentials, and private home, state, generation-store, and report paths. It
completed with this evidence:

- 181 providers, 6,228 accepted source models, and four rejected source
  records;
- exactly 115 quarantined unresolved offerings from Azure OpenAI and Mistral;
- 611 valid published offerings retained, ten valid updates, and no unresolved
  additions;
- generation `efc4403d-5867-46e6-ad7e-75e62ca660b0`;
- payload checksum
  `sha256:18458cff139ba774beda565d8bbc0e2a0776dc59bdd1d2e992b80a700a95f5ba`;
- semantic checksum
  `sha256:e58d574ff2cf72410872523d693bb4d49dd6c534d73bff4be5722128698750a6`;
- regenerated bootstrap manifest and successful strict provider, author, model,
  and cross-reference validation.

Focused tests and the changed-package race gate pass. Full local, exact-head,
protected-merge, and hosted-generation evidence remains open.
