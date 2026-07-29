# P5 Provider/Model Identity Review

Date: 2026-07-28  
Baseline: protected main `76dd317810815604b6c796814bce5b8887aaadd0`

## Review question

Could the author/model structure removed by PR #53 be restored without
reintroducing stale denormalized copies, and could that structure be used to
generate complete provider endpoint data?

## Finding

Yes, but only when provider identity and author identity remain independent.
The provider corpus disproves a provider-to-author rule:

- Alibaba serves models authored by Moonshot AI, Zhipu AI, DeepSeek, and Qwen;
- Fireworks and DeepInfra serve models from many independent labs;
- exact provider model IDs are opaque and may contain namespaces, slashes,
  dates, case, or provider-specific aliases;
- one canonical authored model may have several exact provider offerings with
  different price, limits, lifecycle, and endpoint behavior.

The pre-#53 `Author.Models` implementation could not express this safely. It
copied provider facts into authors, discarded provider identity, skipped some
hierarchical IDs on save, and therefore could neither stay fresh nor produce a
complete endpoint join.

## Reviewed replacement

The workspace now retains two human-readable record roles:

| Record | Identity | Owned authority |
| --- | --- | --- |
| `authors/{author}/models/{slug}.yaml` | canonical `{author}/{slug}` | intrinsic authored identity, description, release metadata, lineage, weights, and capabilities |
| `providers/{provider}/models/**.yaml` | exact opaque provider model ID plus `model: author/slug` | provider price, limits, availability, lifecycle, modes, endpoint/request behavior, and provider evidence |

Provider source records may still contain overlapping upstream observations
such as a display name or authors list. Those values remain source evidence,
but they do not participate in canonical definition construction when an
explicit authored record exists. This gives the two roles non-overlapping
executable authority without pretending upstream payloads are syntactically
disjoint.

`Author` remains metadata-only in Go. The authored collection is a dedicated
Builder construction store rather than a nested mutable `Author.Models` map.
Publication copies it into the immutable catalog, derives canonical definitions
from authored records, derives provider offerings from linked provider records,
and precomputes identity and join indexes.

## Corpus disposition

The exact reviewed provider identity map is
[`P5_PROVIDER_MODEL_IDENTITY_MAP_2026-07-28.yaml`](P5_PROVIDER_MODEL_IDENTITY_MAP_2026-07-28.yaml),
SHA-256
`6d6fce188901b55bcad12df3fdb5624cda4747ff2802e3cdc3cb4a487e4f136c`.

| Measurement | Before restoration | Reviewed result |
| --- | ---: | ---: |
| Provider model YAML records | 611 | 610 |
| Provider records with explicit canonical link | 549 | 610 |
| Provider records without a resolved link | 62 | 0 |
| Authored model YAML records | 322 restored records | 649 total canonical records |
| Canonical definitions with at least one offering | not available | 531 |
| Generated endpoint rows | 0 | 610 |
| Authored-only definitions | not available | 118 |

Every retained provider record now has a reviewed explicit link derived from
inline authorship, publisher namespace, exact model match, or named source
evidence. The full map, rather than an inference rule, is the executable review
boundary. Additional authored records were created only where a serving record
had no retained canonical target.

One record,
`providers/alibaba/models/pre-zhongyun-test-chat.yaml`, was removed. It was an
opaque metadata-free test identifier with no author, public model identity, or
reviewable source evidence. Retaining it would require inventing a canonical
author/model and would violate the zero-unresolved-link release gate.

Specific reviewed identities include:

- Alibaba `kimi-k2.5` → `moonshot-ai/kimi-k2.5`;
- Alibaba `glm-5.1` → `zhipu-ai/glm-5.1`;
- Alibaba `pre-qwen-mt-lite` → `qwen/qwen-mt-lite`;
- Alibaba and DeepInfra Qwen offerings → the independent `qwen` / Qwen Team
  author namespace, not the Alibaba Cloud serving-provider namespace;
- Groq `openai/gpt-oss-safeguard-20b` → OpenAI;
- Groq `canopylabs/orpheus-*` → Canopy Labs;
- Vertex `bart-large-cnn` → Meta;
- Fireworks Kimi and GLM aliases → Moonshot AI and Zhipu AI respectively;
- Vertex deployment suffixes such as `@001` and `-maas` remain exact opaque
  provider model IDs while their links resolve to provider-independent author
  slugs;
- `FastVideo/LTX-2.3-Distilled-Diffusers` →
  `fastvideo/ltx-2.3-distilled-diffusers`, with Lightricks retained as a
  secondary author rather than inferred as the serving provider.

## Generated endpoint projection

`internal/embedded/catalog/endpoints.yaml` is generated from the validated
immutable join. It contains:

- projection schema version;
- committed generation ID and catalog payload digest;
- canonical `author/slug`;
- exact provider ID and exact opaque provider model ID;
- exact provider pricing, limits, availability, lifecycle, modes, and endpoint
behavior when known.

The first structured outcome review found and the branch corrected four
classes of generation defect before acceptance: serving provider mistaken for
model author, Vertex deployment suffixes promoted into canonical author
identity, Groq per-token literals mislabeled as per-million prices, and JSON
request overrides encoded as YAML byte arrays. Provider float noise is snapped
only when it is within a tight tolerance of a human decimal price; meaningful
small prices are unchanged.

It deliberately contains no invented latency, throughput, uptime, or other
runtime telemetry. It is built in the off-side workspace candidate, verified
for deterministic byte stability, and promoted with the YAML workspace after
the durable generation-store commit. Direct edits are detected as drift and
are not silently overwritten.

The removed generic `catalogs.Endpoint` collection was unrelated: it carried
only ID, name, and description, had no production endpoint generator, and
could not represent a provider offering. It was deleted instead of being
repurposed into a second authority.

## Immutable consumer result

The normal public path remains:

```go
sm, err := starmap.New()
if err != nil {
    return err
}

catalog := sm.Catalog()
model, err := catalog.FindModel("gpt-4o")
if err != nil {
    return err
}
offerings, err := catalog.DefinitionOfferings(model.ID)
```

`FindModel` resolves canonical `author/slug`, unambiguous bare slugs, and
unambiguous exact provider IDs. Ambiguity is a typed conflict. Exact provider
facts remain available through `Offering(provider, providerModelID)` and
`DefinitionOfferings`.

## Verification contract

- every authored YAML file becomes exactly one definition;
- every provider YAML file becomes exactly one offering and one generated
  endpoint row;
- every provider record links to an existing authored record;
- every canonical lineage root/parent resolves;
- Alibaba-served Kimi and GLM definitions retain Moonshot/Zhipu authorship;
- provider prices remain independent and exact across shared definitions;
- authored and provider records round-trip independently through YAML and
  catalog schema version 3;
- `endpoints.yaml` reproduces byte-for-byte from the embedded generation;
- malformed unrelated siblings quarantine while unresolved public identity
  fails publication;
- builder mutation cannot change a published catalog;
- ordinary, race, documentation, lint, security, and hosted phase gates pass
  on the exact final head.

## Decision

Restore and retain the author/model hierarchy, but do not restore
`Author.Models` or the old copy semantics. Canonical authored records and
explicitly linked provider-serving records are the minimal complete input
model. The immutable definitions, offerings, indexes, and `endpoints.yaml` are
derived products.
