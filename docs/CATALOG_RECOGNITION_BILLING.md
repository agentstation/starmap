# Document recognition billing

Catalog schema 10 adds `Model.Billing` and `ProviderOffering.Billing`.
Billing units belong to the provider offering. They do not belong to the author’s model definition.
The record is independent of capability, endpoint selection, and current prices.
A price refresh selects one complete pricing record and retains independently sourced billing facts.

A recognition record declares one basis:

- `pages`: settle processed pages against `pricing.operations.page_input`.
- `tokens`: settle measured provider input and output tokens against the selected token rates, including applicable modality rates.

An absent billing record means unknown billing units. A missing price or usage value is unknown, not a free operation.
A consumer must check currency and the price validity interval before settlement.
A cache hit must not charge the original recognition work again.

```yaml
billing:
  recognition:
    basis: tokens
```

An optional `input_page_estimate` describes an input token assumption:

```yaml
billing:
  recognition:
    basis: tokens
    input_page_estimate:
      tokens: 258
      source: https://ai.google.dev/gemini-api/docs/document-processing
      assumptions: One page at the documented resolution; excludes output.
```

The estimate is not a fixed charge. A consumer must label its derived price as an estimate and show the assumptions and selected source rate.
It excludes output tokens and cannot replace measured usage during settlement.
Do not apply one token count across all Gemini models and resolutions.
The embedded Gemini recognition offerings declare token billing without a page estimate.

Catalog protocol facts determine chat eligibility independently of pricing.
An offering with only WebSocket delivery does not gain an HTTP chat route when its audio prices change.
An offering that declares both HTTP and WebSocket delivery can retain HTTP chat support.
Google’s observed generation actions supply these delivery protocols.

Schema 10 payloads require a compatible consumer. Readers retain the schema and identity of validated older payloads.
A payload that declares an older schema cannot carry the new billing record.
Starport’s accounting integration and paired qualification remain part of CSP6.2.
