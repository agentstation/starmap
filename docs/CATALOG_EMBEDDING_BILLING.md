# Embedding billing

Catalog schema 12 adds `billing.embeddings` to provider offerings.
The declaration covers synchronous text and token-ID embeddings at the ordinary input token rate.
It excludes media inputs and provider-side batch discounts.
Vector dimensions do not change the declared rate.

```yaml
billing:
  embeddings:
    basis: input_tokens
    request_charge: false
```

`input_tokens` charges the complete provider input count through `pricing.tokens.input`.
The explicit `request_charge` boolean states whether `pricing.operations.request` also applies once per call.
Missing declarations or prices remain unknown.
An explicit zero price means free.
A billing declaration grants no capability or permission.

The OpenAI `text-embedding-3-small` offering carries this declaration.
Its [model reference](https://developers.openai.com/api/docs/models/text-embedding-3-small) supplies the input price.
The [embedding guide](https://developers.openai.com/api/docs/guides/embeddings) states input-token billing and the 8,192-token input limit.
The serving record uses that per-item limit.
The gateway can reserve the full limit for every request item without a local token estimate.
Other offerings need their own verified declarations.

Settlement requires complete provider-reported input usage.
A missing count, a local estimate, or an inconsistent total cannot release reserved capacity.
Consumers retain the selected generation and rates throughout the attempt.
Retries and semantic-cache child calls each need separate admission.

Schema 11 remains readable but cannot carry embedding declarations.
JSON, YAML, model copies, and immutable offerings must retain the declaration and its explicit boolean.
