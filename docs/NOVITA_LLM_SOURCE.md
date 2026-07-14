# Novita AI LLM API source contract

Status: P13.37 implementation evidence

## Authoritative public surface

- The current [List models API](https://novita.ai/docs/api-reference/model-apis-llm-list-models) defines bearer-authenticated `GET https://api.novita.ai/openai/v1/models` and requires exact owner-qualified ID, creation time, object kind, fixed-point input/output prices, title, description, and context size.
- Novita's [LLM API overview](https://novita.ai/docs/api-reference/model-apis-introduction) exposes separate OpenAI-compatible Chat Completions, text Completions, embeddings, and rerank APIs. The LLM model-list contract is kept separate from the media API families.
- The current [pricing table](https://novita.ai/pricing) and model pages express prices as USD per million tokens and document the introductory 50% batch discount. The integer list fields use thousandths of USD per million: the documented `3900` example represents `$3.900/M`, and current first-party `$0.135/M` and `$0.400/M` rows correspond to `135` and `400`. Starmap retains both raw integers and normalized USD values so this fixed-point interpretation is auditable.
- The [billing policy](https://novita.ai/docs/changelog/27-03-26) separately charges input and output tokens and confirms that rejected pre-inference requests are not billed.

## Public/private boundary

Only the `/openai/v1/models` LLM inventory enters this public source. The separate `/v3/model` API spans public and private image checkpoints, LoRAs, training outputs, uploaded weights, download URLs, hashes, and cursors; it is not an enterprise LLM catalog and is deliberately excluded. Dedicated endpoints, aliases, endpoint URLs, GPU selections, and uploaded/custom weights require an explicit future credential-scoped source and canonical mapping.

Novita is the provider; exact model-ID prefixes retain Alibaba/Qwen, BAAI, DeepSeek, Google, Meta, Microsoft, Mistral, Moonshot, OpenAI, Xiaomi, and Zhipu authors. Public models use the stable `https://api.novita.ai/openai/v1` base and `serverless/pay-per-token` deployment.

## Validation and routing

The source fails closed when the data array is null, IDs repeat, object kind is not `model`, title/description is absent, context is non-positive, either price is absent, or a price is negative. Zero/zero prices remain raw evidence but omit canonical pricing because the current pricing validator intentionally rejects empty zero-only price objects. Chat and text Completions remain separately typed invocation contracts. Unknown additive fields become source-drift evidence.

## Verification

Fixtures prove bearer auth, upstream author identity, title/description/context preservation, exact fixed-point conversion, raw price evidence, 50% batch prices, Chat/Completions routing, drift evidence, and fail-closed negative/missing/duplicate inputs. Shared transport tests cover terminal/transient failures, retry, and cancellation. Live discovery is explicitly skipped when `NOVITA_API_KEY` is absent.
