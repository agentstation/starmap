# Cloudflare Workers AI Source

Status: P13.28 implementation evidence

## Catalog and routing

- Cloudflare's [Model Search API](https://developers.cloudflare.com/api/resources/ai/subresources/models/methods/list/)
  defines account-authenticated `GET /accounts/{account_id}/ai/models/search`,
  page/per-page controls, experimental/deprecation filters, and the
  `format=openrouter` marketplace response.
- [Workers AI](https://developers.cloudflare.com/workers-ai/) documents the
  serverless GPU service and REST invocation contract.
- [OpenAI-compatible endpoints](https://developers.cloudflare.com/workers-ai/configuration/open-ai-compatible-endpoints/)
  define the account-scoped `/ai/v1` route.

Starmap requests the normalized marketplace format, includes the three-month
deprecated grace window, hides experimental records, and traverses at most 32
pages of 50 records. Stable `@cf/<author>/<model>` identity preserves the model
author separately from Cloudflare as provider. Architecture modalities decide
chat, embeddings, image, or audio routing; unknown modalities remain
discoverable-only. Context, supported parameters, descriptions, and unknown
upstream fields remain evidence.

The account ID and API token are runtime-only. Public offerings retain an
`{account_id}` path template rather than a concrete customer account, so no
account identifier or test origin can enter the embedded catalog.

## Pricing

The current [Workers AI pricing table](https://developers.cloudflare.com/workers-ai/platform/pricing/),
last updated July 8, 2026, defines per-model token and non-token units and the
underlying neuron conversion. For OpenRouter-format LLM/embedding records,
Starmap converts the exact per-token decimal strings returned by model search
to canonical USD per-million token values. Missing prices remain absent;
invalid, negative, or non-finite values fail closed. Image/audio/specialized
units are not coerced into token or request prices.

## Failure and live policy

Fixtures prove bearer auth, account-scoped URL construction, filters, two-page
termination, exact price conversion, author/provider separation, chat/embed
routing, unknown-field evidence, unknown-modality non-routability, null-envelope
failure, invalid pricing, account/token isolation, and factory routing. Shared
transport retry fixtures cover terminal and transient failures. Live discovery
is explicitly skipped when `CLOUDFLARE_API_TOKEN` and
`CLOUDFLARE_ACCOUNT_ID` are absent.
