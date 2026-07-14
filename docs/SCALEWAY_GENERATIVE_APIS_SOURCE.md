# Scaleway Generative APIs source contract

Status: P13.35 implementation evidence

## Authoritative public surfaces

- The current [Generative APIs API](https://www.scaleway.com/en/developers/api/generative-apis) defines bearer authentication with `SCW_SECRET_KEY`, Paris region `fr-par`, OpenAI-compatible `/v1/models`, Responses, Chat Completions, audio transcription, embeddings, rerank, and batches.
- The [Models API guide](https://www.scaleway.com/en/docs/generative-apis/api-cli/using-models-api/) identifies `GET https://api.scaleway.ai/v1/models` as the live available-model inventory. Its intentionally small OpenAI shape carries exact model identity and creation/owner facts, not pricing or lifecycle.
- The current [serverless pricing table](https://www.scaleway.com/en/pricing/model-as-a-service/) is the pricing authority. Prices are EUR per million input/output tokens or EUR per audio minute; batch requests are exactly 50% discounted.
- The [supported-model catalog](https://www.scaleway.com/en/docs/generative-apis/reference-content/supported-models/) supplies model authors, modalities, limits, and per-model feature facts that are absent from `/v1/models`.
- The [lifecycle policy](https://www.scaleway.com/en/docs/generative-apis/reference-content/model-lifecycle/) defines Preview, Active, Deprecated, and EOL. The July 2026 first-party changelog marks Devstral 2, Voxtral Small, Gemma 3, Pixtral, and Qwen 3 Coder deprecated beginning July 1, 2026.

## Public and customer boundaries

The public source publishes only Scaleway's shared Generative APIs Serverless inventory. Each offering is `serverless/pay-per-use`, uses the stable `https://api.scaleway.ai/v1` base, and is resident in Paris, France (`fr-par`). Upstream authors remain canonical authors; Scaleway is the provider.

Generative APIs Dedicated Deployment is a separate customer-scoped product. Project IDs, deployment IDs, custom or private model names, chosen quantization/GPU, private-network configuration, endpoint URLs, and hourly infrastructure prices are never inferred from the public model list and never enter the public generation. A future private inventory reader must require explicit customer configuration and canonical-definition mapping.

## Pricing and routing policy

Only exact model IDs present in the current first-party pricing table receive prices. Token prices retain EUR without currency conversion. Audio-minute pricing remains operation pricing. The `batch` mode is a deterministic half-price copy of token rates. An unrecognized live model remains discoverable without invented pricing until the pricing authority is updated.

Invocation contracts are assigned only from first-party endpoint/model-type evidence. Embedding and audio models do not become chat routes. `gpt-oss-120b` retains both Chat Completions and the documented Responses contract. Holo2 remains vision-capable but is not marked function-calling because the current model page explicitly says function calling is unsupported.

## Verification

The focused fixture proves bearer auth, exact author separation, Paris/France residency, active and deprecated lifecycle, EUR standard and batch pricing, embedding-only routing, Holo2 vision/non-tool behavior, and additive-field drift evidence. Shared OpenAI-client transport tests cover terminal errors, bounded retry, cancellation, malformed envelopes, duplicate IDs, and validation. Live discovery is an explicit skip when `SCW_SECRET_KEY` is absent.
