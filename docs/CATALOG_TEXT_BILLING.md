# Text chat billing

Catalog schema 11 adds `billing.text_chat` to provider offerings.
The record declares every charge for online text chat at the default service level.
It covers streaming, ordinary text messages, and client function declarations.
It excludes media, hosted tools, provider extensions, batch discounts, and other service levels.
Missing billing remains unknown, even when some prices exist.

```yaml
billing:
  text_chat:
    input: [input, cache_read]
    output: [output]
    request_charge: false
```

The input classes partition the complete input token count.
The output classes partition the complete output token count.
The `input` and `output` classes are mandatory.
Each class names the corresponding field under `pricing.tokens`.
A class absent from a declaration has no separate charge.

| Class | Meaning |
| --- | --- |
| `input` | Input tokens outside declared cache classes. |
| `cache_read` | Input tokens that the provider reads from its cache. |
| `cache_write` | Input tokens that the provider writes to its cache. |
| `output` | Output tokens outside a declared reasoning class. |
| `reasoning` | Output reasoning tokens with a separate rate. |

If the output list omits `reasoning`, the output rate includes reasoning tokens.
A consumer must normalize provider counters into these disjoint classes before settlement.
It must not count one token in two classes.

`request_charge` requires an explicit boolean.
When true, the `pricing.operations.request` charge also applies once per provider call.
Every declared class requires its own known rate for monetary admission.
An explicit zero rate means free. A missing rate does not.

Prices remain a separate catalog record.
A price update must preserve the billing declaration unless the source explicitly replaces it.
Consumers retain the selected generation, declaration, and exact rates for each admitted attempt.
A declaration grants no model capability or inference permission.

## Provider evidence and current scope

The OpenAI `gpt-4o-mini` serving record declares input, cached input, and output charges.
Its [model reference](https://developers.openai.com/api/docs/models/gpt-4o-mini) documents these rates and the model limits.
This declaration covers the text subset of its chat operation.
Other offerings require their own verified declarations. Consumers must not copy this contract based on protocol compatibility.

The declaration does not supply an input token counter.
A consumer can use declared provider limits for conservative reservations.
A narrower reservation needs verified request bounds.
Approximate token counts cannot establish a strict spending bound.

## Validation

JSON, YAML, catalog copies, and immutable offerings retain the complete declaration.
Validation rejects missing request decisions, missing ordinary classes, repeated classes, and classes in the wrong group.
Schema 10 retains recognition billing support.
Payloads below schema 11 cannot carry text-chat declarations.

Run the focused contract tests:

```sh
go test -race -count=1 ./pkg/catalogs -run 'Test(TextChat|RecognitionBilling)'
```
