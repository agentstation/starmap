# xAI direct source

Status: P13.14 implementation evidence

## Source contract

Starmap uses xAI's authenticated `GET /v1/language-models` endpoint. This is
the current first-party full-information inventory for chat and image-understanding
models; unlike the minimal `/v1/models` projection, it returns modalities,
fingerprint, version, aliases, token pricing, and long-context pricing fields.

Primary evidence:

- [xAI model REST API](https://docs.x.ai/developers/rest-api-reference/inference/models)
- [xAI pricing](https://docs.x.ai/developers/pricing)
- [Grok 4.5 model card](https://docs.x.ai/developers/grok-4-5)
- [May 15, 2026 model retirement](https://docs.x.ai/developers/migration/may-15-retirement)
- [xAI API security and retention FAQ](https://docs.x.ai/developers/faq/security)

## Canonical mapping

- Inventory records are active xAI offerings and `owned_by: xai` maps to the
  distinct xAI author identity.
- Text input, cached input, and text output prices are converted from the API's
  integer USD-cents-per-100-million-token unit to canonical USD per million.
- A non-zero `long_context_threshold` becomes an exact context pricing tier;
  zero long-context fields use the documented standard-price fallback.
- Fingerprint, version, aliases, and the source-native image-token price remain
  controlled xAI source evidence rather than being promoted into unrelated
  canonical fields.
- The inventory is public provider metadata. It contains no customer identity,
  API key, team, project, or deployment material.

Retired aliases documented outside the authenticated inventory are lifecycle
evidence only. Starmap does not create synthetic retired offerings from that
guide; an ID must be returned by the current inventory to be published.

## Failure and live policy

The `models` array is required and fails closed when missing or null. Fetches use
the shared bounded provider retry policy: authentication/authorization failures
are terminal, while rate limits and transient server failures are bounded and
cancel-aware. A credential-backed isolated live attempt on 2026-07-12 reached
the endpoint but returned terminal HTTP 403 and zero models. That result is
recorded as authorization failure rather than live success; fixtures retain the
deterministic no-secret contract coverage.
