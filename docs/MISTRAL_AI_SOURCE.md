# Mistral AI Direct Source

Observed 2026-07-12 from Mistral's primary API and product documentation.

## Discovery contract

- `GET https://api.mistral.ai/v1/models` lists every model available to the
  authenticated API key using bearer authentication.
- A model card carries exact `id`, `owned_by`, Unix `created`,
  `max_context_length`, aliases, fine-tuned/base type, archived state, and
  booleans for chat completion, FIM, function calling, fine tuning, vision, and
  classification.
- The list contract has no cursor or page token. Starmap therefore performs one
  bounded response read rather than inventing pagination. The shared provider
  fetcher still bounds attempts and retries only retryable HTTP failures.
- The provider model ID remains the exact API `id`. Returned aliases are source
  evidence only and do not create duplicate offerings or route aliases.

Primary source: <https://docs.mistral.ai/api/endpoint/models>

## Definition and offering mapping

Mistral AI is both the direct inference provider (`mistral`) and the author
(`mistral`) but those values occupy separate typed roles. `owned_by=mistralai`
normalizes to the author ID; it never replaces the offering's provider ID.

The current model-card capability object maps to intrinsic definition features.
The direct provider offering owns the exact model ID, server-to-server access,
OpenAI-compatible chat endpoint, context limit, lifecycle, and first-party
price. `archived=true` becomes deprecated/unavailable lifecycle evidence rather
than silently dropping the record.

The model overview and model cards are the lifecycle/category authority. They
name active/open/premier/labs families and identify deprecated or retired model
versions with replacement dates where applicable.

Primary sources:

- <https://docs.mistral.ai/models/overview>
- <https://docs.mistral.ai/models/model-cards/mistral-small-3-2-25-06>

## Pricing evidence

The checked mapping uses USD per million tokens from Mistral's first-party API
pricing page. Current examples include Mistral Medium 3.5 at 1.5 input / 7.5
output, Mistral Small 4 at 0.15 / 0.6, Mistral Large 3 at 0.5 / 1.5,
Ministral 3 at 0.1-0.2 symmetric by size, Codestral 25.08 at 0.3 / 0.9,
Mistral Embed at 0.1 input, and Codestral Embed at 0.15 input. Batch discounts
are a conditional service mode and are not applied to the base price.

Unmatched model IDs remain unpriced. Starmap never guesses a price from a model
family or alias.

Primary source: <https://mistral.ai/pricing/api/>

## Live and fault policy

`MISTRAL_API_KEY` enables live discovery. Missing credentials are an explicit
optional-source degradation. HTTP 401/403/404/409 are terminal; 408/425/429 and
5xx are retried within the shared three-attempt provider policy and the caller's
context deadline. Malformed records are quarantined by the provider observation
boundary, while source-wide schema corruption preserves the last-known-good
generation.
