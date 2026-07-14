# Hyperbolic Serverless Inference source contract

Status: P13.36 implementation evidence

## Authoritative surfaces

- The current [Text Generation APIs](https://www.hyperbolic.ai/docs/inference/text-apis) define bearer-authenticated OpenAI-compatible Chat Completions, the separate text Completions contract for base models, exact owner-qualified model IDs, per-million-token prices, tool support, and sunset markers.
- The same first-party table is the pricing and lifecycle authority for serverless text models. A single price applies to total processed tokens, so Starmap records the same USD-per-million rate on input and output; summing the two token classes reproduces total-token billing.
- `GET https://api.hyperbolic.xyz/v1/models` is the authenticated live OpenAI inventory endpoint. A read-only unauthenticated probe on 2026-07-12 returned the expected `401` Bearer challenge and no catalog data; it is endpoint evidence, not live catalog proof.
- The [inference product](https://www.hyperbolic.ai/inference) distinguishes shared pay-as-you-go serverless inference from dedicated single-tenant hosting with private endpoints, customer weights, and hourly GPU pricing.

## Identity and routing

Hyperbolic remains the provider while exact `deepseek-ai/`, `meta-llama/`, `moonshotai/`, `openai/`, and `Qwen/` prefixes identify model authors. Shared offerings use `https://api.hyperbolic.xyz/v1` and `serverless/pay-per-token` deployment. Instruct models route through Chat Completions. The sunset Llama 3.1 405B base model routes only through the separately typed Completions contract; it is never mislabeled as chat.

Only model IDs in the current first-party table receive pricing, tool, or sunset enrichment. Additive live fields remain source-drift evidence. Unknown live IDs keep their observed identity and route but receive no invented pricing or tool claim.

Dedicated hosting is contextual inventory. Private endpoint URLs, caller weights/model aliases, cluster/GPU identity, capacity, and hourly contracts must never enter public generation and require an explicit future credential-scoped source plus canonical mapping.

## Verification

The focused fixture proves bearer auth, upstream author preservation, exact total-token pricing representation, tool flags, active versus sunset lifecycle, Chat versus base Completions routing, serverless deployment, and additive-field evidence. Shared transport and OpenAI envelope tests cover malformed/null responses, terminal and transient failures, bounded retry, cancellation, and duplicate validation. Live catalog discovery is explicitly skipped when `HYPERBOLIC_API_KEY` is absent.
