# HZR1 Starmap provider proof

Work commit: `28ce574d` (`feat: add Hetzner inference provider`).

## Contract

- Provider ID: `hetzner`.
- Catalog endpoint: `https://inference.hetzner.com/api/v1/models`.
- Inference base URL: `https://inference.hetzner.com/api/v1`.
- Chat path: `/chat/completions`.
- Credential environment name: `HETZNER_API_KEY`.
- The credential uses bearer placement for catalog acquisition and inference.
- `max_model_len` maps to the provider offering context window.
- Published offerings:
  - `Qwen/Qwen3.6-35B-A3B-FP8` links to
    `qwen/qwen3.6-35b-a3b`.
  - `Qwen3.8-27B` links to `qwen/qwen3.8-27b`.
- Each offering has a 262,144-token context window and supports text and image
  input.
- Pricing stays unknown because Hetzner does not publish a durable tariff for
  the experimental service.
- Unreported capability flags use explicit `null` values. The catalog keeps
  unknown capabilities unknown.

## Fail-before and tests

- Before the change, `TestEmbeddedHetznerProviderContract` failed with
  `Hetzner provider is missing`.
- `make testdata PROVIDER=hetzner` passed and captured two live records.
- The focused embedded and OpenAI-compatible race tests passed.
- The reviewed identity map covers 613 linked provider records and no
  unlinked records.
- Both exact model IDs returned HTTP 200 from minimal authenticated chat
  completion requests.
- A provider-only dry run fetched two models and found no additions or
  removals.

## Repository gates

- `make verify`: passed. It covers unit tests, the race suite, vet, lint,
  performance, coverage, and generated documentation. It also covers catalog
  validation, consumer tests, and the build check.
- `make lint`: passed with zero issues.
- `make build`: passed.
- `make docs-check`: passed after the OpenAPI and Go API documentation update.
- `bash scripts/verify-live-providers.sh`: passed. The Hetzner check succeeded.
- `bash scripts/verify-provider-fixture-drift.sh`: the Hetzner subtest passed.
  The full manual matrix failed because Groq and Moonshot returned unrelated
  new wire fields. This change does not modify those provider fixtures.
- `git diff --check`: passed.
- The technical-writing lint passed for the new provider prose. Existing
  diagnostics elsewhere in the full files are outside this change.

The ignored Starmap `.env` contains the local token. Git does not track that
file. Starport has no copy of the token.
