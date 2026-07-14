# Enterprise Provider Expansion Inventory

Inventory commit: `9508ee7866e4683e001e7ad153319d348433045d`

Retrieved: 2026-07-12

This inventory is the Wave 0 baseline for P13. It is derived from protected
`origin/main`, not from an older worktree or historical provider count.

## Reproduce the baseline

```bash
git rev-parse HEAD
rg -c '^- id:' internal/embedded/catalog/providers.yaml
rg -c '^- id:' internal/embedded/catalog/authors.yaml
rg -n '^- id:|^      type:|^      url:|^    name: .*API|^    name: .*TOKEN' \
  internal/embedded/catalog/providers.yaml
find internal/embedded/catalog/providers -mindepth 1 -maxdepth 1 -type d
sed -n '1,80p' internal/embedded/catalog/generation.json
```

The configured public catalog has 11 inference providers and 83 model authors.
The embedded generation is
`catalog-20260711T235312Z-bc35069f054f`, payload
`sha256:bc35069f054f385724b31f13828061d1d4f78a3c2b568b37b4ae028708d42d1e`,
with 2,241,095 canonical bytes. The source tree has 611 provider model records
and 322 author model records; canonical migration reports 583 definitions.
These are different measures and must not be presented as one model count.

## Configured provider inventory

| Provider | Client family | Discovery scope | Authentication | Embedded provider records | Current classification |
| --- | --- | --- | --- | ---: | --- |
| Alibaba Cloud Model Studio | OpenAI-compatible | global public endpoint | bearer API key | 86 | Routable server-to-server |
| Anthropic | custom Anthropic | global public endpoint | `x-api-key` | 10 | Routable server-to-server |
| Cerebras | OpenAI-compatible | global public endpoint | bearer API key | 3 | Routable server-to-server |
| DeepInfra | OpenAI-compatible | global public endpoint | public list; bearer inference token | 168 | Routable server-to-server |
| DeepSeek | OpenAI-compatible | global public endpoint | bearer API key | 2 | Routable server-to-server |
| Fireworks AI | OpenAI-compatible | global public endpoint | bearer API key | 7 | Routable server-to-server |
| Google AI Studio | custom Google | global public endpoint | API key | 54 | Routable server-to-server |
| Google Vertex AI | custom Google/SDK | credential- and region-scoped | ADC plus project/location | 136 | Routable server-to-server; current source is customer/cloud scoped |
| Groq | OpenAI-compatible | global public endpoint | bearer API key | 17 | Routable server-to-server |
| Moonshot AI | OpenAI-compatible | global public endpoint | bearer API key | 11 | Routable server-to-server |
| OpenAI | OpenAI-compatible | global public endpoint | bearer API key | 117 | Routable server-to-server |

Eight of the eleven configured providers use the shared OpenAI-compatible
client: Alibaba, Cerebras, DeepInfra, DeepSeek, Fireworks, Groq, Moonshot, and
OpenAI. Anthropic and the two Google channels use custom clients.

## Source and control-plane inventory

| Area | Repository reality | Wave 0 consequence |
| --- | --- | --- |
| Source adapters | provider fan-out, models.dev HTTP, explicit pinned models.dev Git verification, local catalog, embedded bootstrap | New enterprise sources compose the direct-observation contract; Git and HTTP remain alternatives |
| Authentication | API-key/header/query support plus Google ADC | Add an SDK-neutral credential-chain seam before AWS/Azure clients; never serialize resolved credentials |
| Pagination | Google clients paginate; OpenAI-compatible and Anthropic clients are single-page implementations | Add one bounded cursor contract reusable by every new client |
| Retry | Deployment scheduler has typed bounded retry; provider HTTP calls do not share a `Retry-After`-aware policy | Add provider-call retry separately from scheduler retry |
| Evidence | normalized replay plus encrypted, owner-only, bounded raw evidence | Extend source descriptors with primary URL, retrieval time, API version, and schema assumptions |
| Resilience | bounded payloads, unknown-field fingerprints, record quarantine, last-known-good generation | Reuse these policies for every provider; source-wide corruption remains fail-closed |
| Authority | valid provider offering price wins through production `CanonicalPolicies`; models.dev remains lower-authority enrichment | Keep one production authority module and reject cross-context merges |
| Scheduling | daily 03:17 UTC serialized public generation; deployment scheduler supplies lease/jitter/retry/freshness | Credential-scoped sources remain opt-in and make their run ineligible for public generation |
| Observability | source age/status/degradation and last publication are available | Carry rejected counts, provider coverage, and pricing observation age into the operator projection |
| Publication | immutable exact schema-v2 generation with validation, checksum, source links, and rollback | Any credential-scoped observation rejects the complete public write before bytes; there is no parallel catalog product |

## Wave source classifications

| Source type | Default cadence | Freshness budget | Publication rule |
| --- | --- | --- | --- |
| Direct provider inventory | daily | degraded after 30h; unready after 72h when required | Public when the endpoint is a provider-wide catalog |
| Provider pricing | daily | degraded after 30h; unready after 72h when required | Last-known-good price retained; lower-authority evidence cannot replace a current provider price |
| Regional cloud sweep | daily, serialized by provider | degraded after 30h; unready after 72h | Public only for provider-documented regional offerings |
| Credential-scoped provider source | operator configured; disabled when credentials or bindings are absent | operator configured | Reconciles into the caller's catalog; never embedded or included in public generation |
| Curated application-only evidence | daily evidence check or scheduled review PR | degraded after 7d; unready after 30d | Discoverable-only and never route eligible |

## Primary research registry

Each provider task must add its exact API version and limitations. Wave 0 uses
these primary sources to establish the framework:

- AWS SDK for Go v2 configuration and credential chain:
  <https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html>
- AWS SDK for Go v2 retries/timeouts:
  <https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-retries-timeouts.html>
- Amazon Bedrock Go examples:
  <https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/go_bedrock_code_examples.html>
- Azure SDK for Go control-plane guidance:
  <https://learn.microsoft.com/en-us/azure/developer/go/control-plane>
- Azure Identity credential chains:
  <https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/credential-chains>
- Microsoft Foundry account-management Models List API:
  <https://learn.microsoft.com/en-us/rest/api/microsoftfoundry/accountmanagement/models/list>

The provider-specific primary URLs and required evidence are tracked task by
task in P13 of `docs/STARPORT_CATALOG_CONTROL_PLANE.md`.
