# Snowflake Cortex AI Source

Status: P13.23 implementation evidence

## Discovery and invocation

- [Cortex Inference API](https://docs.snowflake.com/en/developer-guide/snowflake-rest-api/reference/cortex-inference)
  defines authenticated `GET /api/v2/cortex/models` for models available to the
  current session and `POST /api/v2/cortex/inference:complete`.
- [Cortex REST API](https://docs.snowflake.com/en/user-guide/snowflake-cortex/cortex-rest-api)
  defines the account-scoped OpenAI-compatible chat endpoint.
- [Cortex AI Functions](https://docs.snowflake.com/en/user-guide/snowflake-cortex/llm-functions)
  documents SQL `AI_COMPLETE`/`COMPLETE`, regional availability, lifecycle, and
  cross-region inference.

Discovery is account/session scoped because eligibility depends on cloud,
region, cross-region settings, role, and model access. `SNOWFLAKE_ACCOUNT_URL`,
token, home region, and cross-region setting are runtime-only values. The
published offering retains only the eligible model, home region, declared
cross-region profile, lifecycle, and Snowflake invocation contracts; account
identity and token never serialize.

## Pricing

The authoritative [Snowflake Service Consumption Table](https://www.snowflake.com/legal-files/CreditConsumptionTable.pdf),
effective July 10, 2026, provides AI Credits per million input/output/cache
tokens in Table 6(b) and 6(c). Starmap retains those source-native rates as
evidence and emits two exact USD modes using Snowflake's documented routing
conversion:

- global routing: USD 2.00 per AI Credit;
- regional/specific-geography routing: USD 2.20 per AI Credit.

The active base price follows `SNOWFLAKE_CORTEX_CROSS_REGION`; both modes remain
available for review. Unknown current-session model IDs stay unpriced instead
of inheriting another model's rate. Promotional footnotes are retained through
the effective-date evidence and must be refreshed when the legal table changes.

## Failure and live policy

The parser accepts the documented session list as names or model objects inside
`models`/`data`, requires a non-null array and non-empty identity, and rejects
missing account configuration. Fixtures prove bearer auth, lifecycle, authors,
REST/SQL contracts, home/cross-region mapping, global/regional/cache price
conversion, response-shape handling, and deep-copy isolation. Live session
discovery is explicitly skipped when Snowflake account URL/token/region context
is absent.
