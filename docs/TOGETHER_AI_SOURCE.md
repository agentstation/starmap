# Together AI source

Status: P13.18 implementation evidence

## Inventory and deployment contracts

Starmap reads Together's bearer-authenticated native `GET /v1/models` response
twice: the default serverless catalog and `dedicated=true` catalog. The current
API returns exact model ID, type, organization, context, license/link evidence,
and input/output/cached-input pricing.

Primary evidence:

- [List all models](https://docs.together.ai/reference/models)
- [Serverless model catalog](https://docs.together.ai/docs/serverless/models)
- [Dedicated model catalog](https://docs.together.ai/docs/dedicated-endpoints/models)
- [Inference pricing semantics](https://docs.together.ai/docs/inference/pricing)
- [Current first-party price table](https://www.together.ai/pricing)
- [OpenAI-compatible invocation](https://docs.together.ai/docs/inference/openai-compatibility)

## Canonical mapping and isolation

- Together is the provider; the returned organization/model namespace remains
  the underlying model author. Known author identities are mapped explicitly.
- Chat/language/code, embedding, image, and rerank inventory types become exact
  invocation APIs. Unsupported types remain discoverable rather than receiving
  an invented chat route.
- Serverless and dedicated availability are distinct deployment modes on one
  provider offering. Token pricing remains USD per million; source-native
  hourly/base/fine-tune figures stay controlled evidence because the canonical
  schema has no matching unit.
- Records whose organization cannot be resolved to the public author registry
  are excluded. This fails closed against authenticated customer uploads rather
  than publishing account-scoped model names as public catalog identities.

The shared provider retry policy supplies bounded transient/terminal behavior.
Deterministic two-inventory fixtures verify authentication, deployment merge,
pricing, author preservation, and customer isolation. A credential-backed,
isolated live update on 2026-07-12 fetched 179 public Together models and wrote
them only to a temporary export; no embedded model file was replaced.
