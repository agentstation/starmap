# Hugging Face Inference Providers Source

This source catalogs Hugging Face's public Inference Providers router. It is an
aggregator channel: Hugging Face is the routed provider and billing boundary,
while every provider inside a model's `providers` array remains a distinct
upstream offering.

## Primary contracts

- [Hub API](https://huggingface.co/docs/inference-providers/hub-api) documents
  public `GET https://router.huggingface.co/v1/models`, its OpenAI list
  envelope, the provider array, USD-per-million-token prices, context, status,
  authorship, and latest validation-probe latency/throughput signals.
- [Inference Providers](https://huggingface.co/docs/inference-providers/index)
  documents the OpenAI-compatible chat endpoint, explicit `model:provider`
  selection, and the `:fastest`, `:cheapest`, and `:preferred` routing policies.
- [Pricing and billing](https://huggingface.co/docs/inference-providers/pricing)
  documents centralized Hugging Face billing, provider-rate pass-through, the
  separate `hf-inference` provider, custom provider keys, and enterprise
  `X-HF-Bill-To` organization/resource-group attribution.

## Catalog mapping

The inventory request itself is public. Invocation uses a fine-grained
`HF_TOKEN`; no token or organization billing identity is serialized.

Each returned model is one canonical definition. Each element of its provider
array becomes an offering whose exact Hugging Face route ID is
`<model>:<provider>`. The offering provider is `huggingface`; the provider-array
identity is preserved in `aggregator_upstream`. This represents the actual
router and billing path without misrepresenting the upstream as a direct
credentialed route.

The source maps provider-specific context and exact input/output pricing into
the offering, and retains status, free-tier indication, tool/structured-output
support, author-provider relation, latency, throughput, and the response Date
as controlled evidence. `error` upstreams are unavailable and non-routable.
Negative/non-finite price or metric values and unknown statuses fail closed.

`auto`, `fastest`, `cheapest`, and `preferred` are policies, not provider or
model identities. They are rejected if returned as provider records and are
never materialized as definitions. Customer billing targets, tokens, custom
provider keys, preferences, and dedicated Inference Endpoints remain outside
the public catalog.

## Verification

Fixtures prove the strict list envelope, public request, provider expansion,
one-definition/many-offerings projection, exact route IDs, upstream identity,
pricing/context, unavailable fault behavior, probe timestamps, policy
rejection, negative-value failure, factory routing, and deep-copy behavior.
The shared provider transport tests cover bounded retry, terminal
authentication failures, rate limits, and cancellation.

An isolated live update on 2026-07-12 read the public router and produced 280
model/provider route offerings in a temporary export. It did not replace the
embedded model files and required no credential. A separately created
fine-grained token has only the `Make calls to Inference Providers` permission;
a one-token chat smoke returned HTTP 200 through the router. The token remains
only in ignored mode-0600 `.env` and is not catalog data.
