# P7 OpenRouter Adapter Outcome

Date: 2026-07-29

Status: `DONE`

## Scope

This outcome closes P7.12 and F-087 against
[`P7_OPENROUTER_ADAPTER_ARCHITECTURE_REVIEW_2026-07-29.html`](P7_OPENROUTER_ADAPTER_ARCHITECTURE_REVIEW_2026-07-29.html).
It adds OpenRouter-compatible catalog discovery at the public server seam
without changing the restored author-model/provider-offering data model,
persisting another representation, or making generated `endpoints.yaml`
authoritative.

The implemented routes are the current documented OpenRouter contracts:

- [`GET /api/v1/model/{author}/{slug}`](https://openrouter.ai/docs/api/api-reference/models/get-a-model-by-its-slug)
- [`GET /api/v1/models/{author}/{slug}/endpoints`](https://openrouter.ai/docs/api/api-reference/endpoints/list-all-endpoints-for-a-model)

Both routes use `{"data": ...}` success envelopes and numeric
`{"error":{"code":N,"message":"..."}}` error envelopes.

## Architecture disposition

| Review recommendation | Implemented disposition | Evidence |
| --- | --- | --- |
| Retain the restored identity join | Kept authored definitions and exact provider offerings unchanged | Embedded authored/provider YAML and generated `endpoints.yaml` have zero diff from protected main |
| Keep OpenRouter shape at the server seam | Added one concrete `internal/server/openrouter` module and exact handlers/routes | No OpenRouter transport type was added to `pkg/catalogs`; no new public interface or persisted schema exists |
| Use existing immutable indexes | Resolution uses canonical author/slug, author aliases, model aliases, and `DefinitionOfferings` | Canonical, alias, ambiguous, unknown, and variant tests pass |
| Preserve author versus serving-provider identity | Model identity comes from the authored definition; every endpoint retains its provider and exact opaque provider model ID | Multi-provider/lab tests prove Alibaba and DeepInfra serving Moonshot models without becoming their author |
| Keep pricing provider-authoritative | Each endpoint projects its exact valid USD provider price and limits; the model summary selects a deterministic comparable eligible offering | Price-string and multi-provider golden tests pass |
| Do not invent runtime facts | Telemetry is optional in the transport DTO and omitted without a real producer; cache price does not imply implicit caching | JSON absence/presence tests pass |
| Avoid a hypothetical abstraction | The adapter is concrete; no telemetry interface or second adapter seam was introduced | One module owns resolution, projection, DTOs, and HTTP envelopes |

The adapter reads the immutable catalog directly. Generated `endpoints.yaml`
remains a digest-bound human-inspectable projection of the same definition and
offering join; the server does not read it.

## Resolution and projection contract

- Canonical author aliases are resolved before model lookup.
- Exact canonical author/slug wins before known full-ID or bare-slug aliases.
- An alias must still belong to the requested canonical author.
- Ambiguous aliases remain typed conflicts and map deterministically to 404.
- A `:variant` suffix is accepted only when at least one eligible offering
  explicitly defines that mode; the mode's price replaces only the response
  copy.
- Unavailable and retired offerings are excluded. Restricted and unknown
  offerings remain visible.
- Exact provider IDs, provider model IDs, prices, limits, and stable catalog
  order are preserved.
- Non-USD prices are omitted because the compatibility schema has no currency
  field and Starmap does not invent exchange rates.
- Status `0` means only that an included offering is catalog-eligible; it is
  not provider runtime health.
- Latency, throughput, and uptime are absent because catalog freshness,
  publisher health, and SSE liveness are not provider-performance samples.
- Detail links use the configured server path prefix.
- Compatibility-route authentication failures use the numeric OpenRouter 401
  envelope; native routes retain Starmap's existing envelope.
- Invalid encoded segments cannot smuggle separators into author or slug
  identity, and non-compatibility `/models` paths retain native handling.

## Structured review disposition

The outcome-oriented autoreview command was:

```bash
/Users/jack/src/github.com/nimbus/agent-skills/skills/autoreview/scripts/autoreview \
  --mode local \
  --engine claude \
  --model claude-fable-5 \
  --thinking max \
  --stream-engine-output
```

Every reported finding was verified against the real code and accepted as an
in-scope adapter or router issue:

| Finding | Disposition |
| --- | --- |
| Detail links hardcoded `/api/v1` | Fixed by threading the validated configured prefix into projection; real HTTP regression added |
| Compatibility auth returned 403 rather than OpenRouter 401 | Fixed with the exact numeric 401 envelope; native auth behavior remains unchanged |
| Auth envelope classification used decoded paths while routing used escaped paths | Fixed by classifying the escaped path; encoded-separator regression added |
| Missing provider during endpoint projection could map an internal integrity error to 404 | Fixed by returning a typed validation failure that maps to 500 |
| Invalid three-segment native `/models` paths could receive an OpenRouter envelope | Fixed by reserving compatibility handling for the exact `endpoints` suffix and falling through otherwise |
| External consumer pinned `openai/gpt-4o` availability | Fixed by deriving a stable eligible canonical definition/offering from the immutable catalog |

Because remediation changed public wire and failure behavior, the structured
review was repeated as required. The final run exited successfully with:

```text
autoreview clean: no accepted/actionable findings reported
```

The final reviewer classified the patch as correct and independently
reconfirmed the singular model route, endpoint route, numeric envelopes,
encoded-segment rejection, native-route preservation, deterministic resolution,
fixture consistency, and OpenAPI alignment.

## Verification

The implementation includes:

- unit tests for canonical identity, aliases, ambiguity, variants, eligibility,
  pricing, parameters, reasoning, modalities, telemetry omission/presence, and
  envelopes;
- golden real-HTTP fixtures for model and endpoint responses;
- exact authentication and configured-prefix tests;
- a complete embedded-catalog projection/marshal sweep;
- a real external `GOWORK=off` server consumer that derives an eligible model
  from the current catalog and calls both compatibility routes;
- generated OpenAPI and GoDoc; and
- an enforced 85% statement-coverage floor for the adapter.

Final product-tree `./scripts/verify.sh` passed ordinary tests, all four external
consumer modules, the repository-wide short race suite, vet, the catalog
performance budget, pinned golangci-lint v2.12.2, every enforced coverage
floor, generated documentation/OpenAPI, diff checks, build, all 610 catalog
records, and CLI smoke tests. Selected ordinary/race timings were:

| Package | Ordinary | Race |
| --- | ---: | ---: |
| root | 49.119s | 264.090s |
| `internal/server/openrouter` | 4.322s | 10.258s |
| `internal/server/handlers` | 12.818s | 46.556s |
| `internal/server` | 25.162s | 111.846s |
| public `server` | 7.944s | 28.873s |

The separately pinned CI-equivalent command
`go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...` found zero reachable
vulnerabilities and zero vulnerabilities in imported packages.

The focused final race gate also passed:

```text
internal/server/openrouter  6.676s
internal/server/handlers   33.311s
internal/server/middleware  1.810s
internal/server            92.589s
server                     19.970s
```

Adapter statement coverage is 88.5%. All four external consumer modules pass
with dependency closures of read-only 159/160, server 247/260, and remote
231/240 packages; the store-only publication consumer also passes. Catalog
access remains 8.567–8.793 ns/op with zero bytes and zero
allocations. Embedded author/provider inputs and generated `endpoints.yaml`
remain byte-identical to protected main.
