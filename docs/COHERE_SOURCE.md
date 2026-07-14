# Cohere direct source

Status: P13.15 implementation evidence

## Current inventory contract

The Cohere documentation is presented under the v2 API site, but the current
model inventory contract is explicitly `GET /v1/models`. Starmap follows that
native contract with bearer authentication, `page_size=1000`, opaque
`page_token` traversal, repeated-cursor detection, and finite page/record
ceilings.

Primary evidence:

- [List Models](https://docs.cohere.com/v2/reference/list-models)
- [Get a Model](https://docs.cohere.com/v2/reference/get-model)
- [Model overview](https://docs.cohere.com/v2/docs/models)
- [Deprecations](https://docs.cohere.com/v2/docs/deprecations)
- [Pricing behavior](https://docs.cohere.com/docs/how-does-cohere-pricing-work)
- [Current first-party pricing page](https://cohere.com/pricing)

## Canonical and isolation mapping

- `name`, `is_deprecated`, `context_length`, `tokenizer_url`, endpoints,
  features, defaults, and sampling evidence are mapped or retained exactly.
- Chat, Embed, and Rerank endpoints become distinct canonical invocation APIs;
  the retired Generate endpoint is not promoted into a current route.
- Cohere is both the direct provider and model author, represented by distinct
  provider and author identities.
- `finetuned: true` inventory is excluded from public publication because it is
  authenticated customer-scoped inventory. Starmap does not turn a private
  fine-tune name into a public definition or offering.
- Current inventory lifecycle controls publication. Historical shutdown IDs
  are not synthesized after they disappear from the inventory.

The current pricing page no longer publishes pay-as-you-go prices for every
active model. Starmap therefore prices only exact IDs for which that page gives
a first-party numeric value (retired Command variants, Command R+ 08-2024, and
Aya Expanse). All other IDs remain explicitly unpriced instead of inheriting a
family guess or cloud-marketplace price.

## Failure and live policy

Missing/null model arrays, invalid context lengths, cursor loops, excessive
pages/records, authentication failures, and malformed payloads fail closed.
The shared provider retry policy supplies bounded transient retry around the
complete list operation. Live discovery is attempted only when `COHERE_API_KEY`
is present; deterministic multi-page fixtures otherwise prove the request and
mapping contract without exposing a credential.
