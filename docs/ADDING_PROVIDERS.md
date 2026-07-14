# Adding a Provider

This is the normative provider implementation contract for Starmap. Choose the
smallest role that matches the upstream contract. A connector is a deliberate,
reusable protocol module (currently OpenAI, Anthropic, or Google), not a synonym
for every provider client. A provider directory owns only provider-specific
acquisition, configuration tests, irreducible adapters, regional orchestration,
pricing importers, and deterministic deltas. Governed observations are keyed by
provider and logical source under the shared fixture module.

## Ownership first

Starmap has five distinct authorities:

| Data | Owner | Location |
| --- | --- | --- |
| Stable acquisition and interpretation | Provider configuration | `internal/embedded/catalog/providers.yaml` |
| Normalized current or reviewed facts | Catalog schema v2 | provider model YAML or typed provider catalog data |
| Source, revision, observation time, and scope | Provenance/observation metadata | governed observation metadata; never a duplicate fact table |
| Reusable outbound protocol behavior | Go | `internal/connectors/<protocol>` |
| Provider-specific acquisition/deviations/orchestration | Go | `internal/providers/<id>` |

Do not compile current prices, lifecycle lists, model capabilities, regions, or
provider-wide endpoint/deployment defaults into Go. Edit the catalog directly
for reviewed last-known-good facts. Use declarative field mappings and pricing
units when an OpenAI-compatible payload exposes bounded source fields. Add a
live pricing importer only when a first-party pricing API has behavior that
cannot be represented as catalog data.

## Choose one module role

```text
Existing connector can acquire this inventory?
├─ Yes, and configuration expresses provider semantics
│  └─ YAML-only provider: table-driven configuration test; no production Go
├─ Yes, but bounded provider record semantics remain
│  └─ Provider adapter: adapter.go + adapter_test.go + testdata
├─ No, the behavior is specific to one provider
│  └─ Provider client: internal/providers/<id>/client.go + client_test.go
├─ No, a stable protocol is deliberately reused across providers
│  └─ Protocol connector: internal/connectors/<protocol>/client.go + client_test.go
└─ No, discovery requires regional/account sweeps or multiple APIs
   └─ Provider source: source.go + source_test.go; connector/SDK seam when reusable
```

Additional files have narrow meanings:

- `pricing.go` is only a parser/importer for a live official pricing source. It
  requires `pricing_test.go`; a map of current prices belongs in catalog data.
- `response_schema_test.go` lives beside a connector and classifies supplemental
  response fields. It never replaces `client_test.go` or provider evidence.
- `catalog_test.go` may verify provider-owned catalog facts after mutable tables
  move out of Go.
- `provider_test.go` is reserved for a provider-owned configuration delta or
  composition that is not an exact connector contract; exact connector users
  belong in the shared table-driven configuration test.

Alibaba, Baseten, Cerebras, DeepInfra, DeepSeek, Fireworks, Groq, Hyperbolic,
Mistral, Moonshot, SambaNova, and Scaleway are YAML-only users of the OpenAI
connector. Additive formats such as Cerebras and SambaNova are declarative
extensions, not new connectors. Novita and xAI retain narrow provider adapters
only for evidenced validation or conversion behavior that YAML cannot express.
Cohere and Together own provider-local clients because their acquisition
contracts are not reused by another provider. Bedrock, Microsoft Foundry, and
OCI remain regional/account provider sources.

## Configure the provider

Add identity, authentication, and the catalog endpoint to
`internal/embedded/catalog/providers.yaml`. For OpenAI-compatible providers,
prefer these declarative fields:

- `response_collection` for a safe dotted object path to the model array;
- `field_mappings` for limits, lifecycle, capabilities, extensions, and pricing;
- allow-listed pricing `unit`, uppercase `currency`, optional `tier`, and `mode`;
- `offering` defaults for access APIs, endpoint type, deployment, and regions;
- `base_url_env_var` plus the inventory `path` so acquisition and published
  offering endpoints derive from one effective base URL.

Configuration is data, not an executable expression language. Safe dotted
source paths may select scalar or scalar-list values retained from the raw
model record; canonical destinations and conversions remain allow-listed.
Unsafe or secret-bearing paths, unsupported values, unknown units, duplicate
targets, incompatible pricing units, malformed offering
contracts, and secret-bearing extension fields must fail validation before any
request.

Register connector and provider-local acquisition implementations in
`internal/providers/registry`; configured logical sources are constructed by
`internal/sources/providers`.
Dispatch by endpoint type for a reusable protocol connector and by provider ID
only for provider-local clients or adapters. A YAML-only provider must work
through its connector with no provider-name branch. Production protocol
connector code must never import provider packages; the provider registry is
the composition module allowed to import both. Connector tests use
connector-local deterministic protocol fixtures and must not read provider
testdata or governed observations.

The configured catalog endpoint is the sole static authority for public
provider hosts, inventory paths, and provider-wide offering endpoint/deployment
defaults. A client must fail with a typed configuration error before transport
when that endpoint is missing. Full provider URLs are acceptable in tests;
runtime-computed account, realm, workspace, and regional endpoints are acceptable
only when the implementation records why they are operational inputs rather
than duplicate static provider configuration.

## Configure authentication and logical sources

Each source has one `auth` value. The exact forms are:

- `none`: credentials are prohibited for this source;
- `optional`: try the conventional `api_key`, then the provider-inferred
  `cloud_chain`, and execute once without authentication only when neither is
  available;
- `api_key`, `cloud_chain`, or another named method: that method is required;
- `[api_key, cloud_chain]`: ordered required alternatives; select the first
  available method and fail when none is available.

`api_key` is conventional but not an implicit decoder default: declare its
environment name once under `credentials.api_key.env`. A scalar is normal; an
ordered list is permitted only for evidenced aliases. `cloud_chain` never names
AWS, Azure, Google, or OCI in YAML. The provider ID selects exactly one
official-SDK adapter registered in Go. Compound methods declare each required
secret input under one named credential. A present but invalid or conflicting
credential always fails; it never falls through to a weaker method.

Source `optional: true` is independent of auth: it controls whether the whole
logical source may be skipped when prerequisites are unavailable. One logical
source is one bounded execution, even when its topology is paginated, regional,
or grouped. Pagination, retries, and regional iteration stay inside that one
execution and must have explicit bounds.

Before:

```yaml
api_key:
  env_var: EXAMPLE_API_KEY
catalog:
  endpoint: https://api.example.com/v1/models
  auth_required: false
```

After:

```yaml
credentials:
  api_key:
    env: EXAMPLE_API_KEY
catalog:
  sources:
    - id: models
      observation_scope:
        anonymous: global_public
        authenticated: credential_scoped
      auth: optional
      topology: single_endpoint
      endpoint:
        type: openai
        url: https://api.example.com/v1/models
```

If credentials exist, Starmap uses them and executes the source once. The
result is one ordinary schema-v2 catalog observation. It does not fetch a
second anonymous catalog or construct an overlay. `global_public` and
`regional_public` observations are eligible for public generation;
`credential_scoped` observations are usable in the current in-memory catalog
but reject every public store, generation, distribution, and remote-publication
boundary before bytes are written.

Bindings are non-secret operational inputs. Use typed `scopes` for account,
project, subscription, workspace, realm, and region identity; use `options` for
non-secret client behavior. SDK-owned ambient names are descriptive
`environment` advisories only and are never executed by Starmap. Cloudflare is
not a cloud-chain provider: its API token is a credential and its account ID is
a required scope. Databricks public documentation acquisition is explicitly
`none`; its workspace endpoint source requires the configured token and host,
preventing ambient credential attachment to the public request.

## Add catalog facts

Provider inventory proves current existence and availability; it need not carry
every fact. Reviewed model prices, lifecycle, invocation overrides, and
capabilities may be edited in `internal/embedded/catalog/providers/<id>/` and
are published as canonical schema-v2 definitions and offerings. The catalog is
the durable last-known-good fact store, so do not create a second versioned
evidence manifest containing the same values. Provenance records only where,
when, and under what revision/scope those values were observed.

Authority is provider-official, then the durable catalog baseline, then
models.dev. Inventory-only success preserves valid baseline facts. Partial,
malformed, or explicitly stale provider evidence cannot displace the baseline.
Only complete successful authoritative absence may delete an offering;
models.dev absence is never deletion evidence.

## Own testdata and observations separately

Stable connector protocol fixtures live under
`internal/connectors/<protocol>/testdata`. Stable provider-delta fixtures may
live under `internal/providers/<id>/testdata` only when configuration cannot
express the evidenced delta. These are minimized deterministic test inputs:
they carry no capture metadata and ordinary tests never refresh them. Exact
OpenAI-compatible providers use table-driven embedded-configuration and
registry-composition tests, not provider-local response-schema fixtures.

Retain a genuine raw response for replay or catalog import only as:

```text
internal/providers/fixtures/responses/<id>/<source>/models_list.json
internal/providers/fixtures/responses/<id>/<source>/models_list.metadata.json
```

Refresh a governed observation explicitly:

```bash
make testdata PROVIDER=<id> SOURCE=<source>
# or directly:
go run ./cmd/provider-fixtures refresh --provider <id> --source <source>
go run ./cmd/provider-fixtures replay --provider <id> --source <source> --fixture <path>
go run ./cmd/provider-fixtures import --provider <id> --source <source> --fixture <path> --output <catalog-root>
```

The refresh command writes only the governed observation store. It fetches the
raw response, validates it through the
registered connector, rejects empty/oversized/unchanged/invalid or
secret-bearing payloads, and promotes payload plus integrity/freshness metadata
with atomic file replacement. Output contains only safe provider/source
identity, counts, byte size, and checksum. Ordinary tests never make network
calls, and `go test -update` is not a supported refresh path.

Replay and import share the same verifier and decoder. They verify exact
provider/source path identity, checksum, revision, and freshness before reading
or decoding a governed payload. Fixture class is derived from source scope,
auth, bindings, topology, connector reuse, and any evidenced adapter delta:
connector fixtures cover shared protocols; table-driven tests cover exact
composition; provider fixtures cover only deltas; SDK fakes cover official-SDK
sources; credential-scoped raw captures are prohibited. There is no broad
fixture-policy file or duplicated exception registry.

## Required tests

At minimum, prove:

- authentication and effective endpoint selection;
- successful representative decoding and canonical definition/offering output;
- malformed/null/wrong-shape/duplicate/negative/overflow failure policy;
- unknown-field or schema-drift classification;
- provider/author separation and cross-context isolation;
- exact pricing units, zero versus absence, and modes when present;
- deep-copy isolation for retained canonical state;
- deterministic fixture ownership, governed observation integrity/freshness,
  or the tested exception.

Run the narrow gate first, always with race detection:

```bash
go test -race ./internal/connectors/<protocol>   # when adding connector behavior
go test -race ./internal/providers/<id>          # when adding provider behavior
go test -race ./internal/connectors/... ./internal/providers/...
go vet ./internal/connectors/... ./internal/providers/...
make provider-contract-check
make catalog-generation-check
make embedded-catalog-budget-check
git diff --check
```

Before closeout, run the repository-wide short race, vet, pinned lint,
generated-doc, catalog-generation, artifact/hash, diff, and complete verifier
gates required by the active control plane. Live calls and hosted checks are
reported separately from deterministic fixture proof.
