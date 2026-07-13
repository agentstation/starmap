# Adding a Provider

This is the normative provider implementation contract for Starmap. Choose the
smallest role that matches the upstream protocol. A provider directory owns its
behavior tests and evidence, while shared transport remains provider-neutral.

## Ownership first

Starmap has four distinct authorities:

| Data | Owner | Location |
| --- | --- | --- |
| Stable acquisition and interpretation | Provider configuration | `internal/embedded/catalog/providers.yaml` |
| Normalized current or reviewed facts | Catalog schema v2 | provider model YAML or typed provider catalog data |
| Source, revision, observation time, and scope | Provenance/fixture metadata | adjacent evidence metadata; never a duplicate fact table |
| Irreducible protocol behavior | Go | the provider package |

Do not compile current prices, lifecycle lists, model capabilities, regions, or
provider-wide endpoint/deployment defaults into Go. Edit the catalog directly
for reviewed last-known-good facts. Use declarative field mappings and pricing
units when an OpenAI-compatible payload exposes bounded source fields. Add a
live pricing importer only when a first-party pricing API has behavior that
cannot be represented as catalog data.

## Choose one module role

```text
Standard OpenAI-compatible list endpoint?
├─ Yes, configuration expresses the response and defaults
│  └─ YAML-only: no production Go; client_test.go + testdata
├─ Yes, but bounded provider wire behavior remains
│  └─ Adapter: adapter.go + adapter_test.go + testdata
├─ No, one provider client can acquire the public inventory
│  └─ Native client: client.go + client_test.go
└─ No, discovery requires regional/account sweeps or multiple APIs
   └─ Source: source.go + source_test.go; client.go or sdk.go when useful
```

Additional files have narrow meanings:

- `pricing.go` is only a parser/importer for a live official pricing source. It
  requires `pricing_test.go`; a map of current prices belongs in catalog data.
- `source_shape_test.go` classifies supplemental response fields. It never
  replaces `client_test.go` or representative evidence.
- `catalog_test.go` may verify provider-owned catalog facts after mutable tables
  move out of Go.

Baseten, Hyperbolic, and Scaleway are examples of YAML-only shared-client
providers. Mistral, Novita, and xAI retain adapters only for provider-specific
record semantics. Cohere and Together use native clients. Bedrock, Microsoft
Foundry, and OCI are regional/account sources.

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

Configuration is data, not an executable expression language. Unknown paths,
units, duplicate targets, incompatible pricing units, malformed offering
contracts, and secret-bearing extension fields must fail validation before any
request.

Register a named factory branch only for an adapter or a native endpoint type.
A YAML-only provider must work through the shared OpenAI-compatible client with
no provider-name branch.

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

## Own representative evidence

The preferred provider fixture is:

```text
internal/providers/<id>/testdata/models_list.json
internal/providers/<id>/testdata/models_list.metadata.json
```

Refresh it explicitly:

```bash
make testdata PROVIDER=<id>
```

The refresh command fetches the raw response, validates it through the
registered provider client, rejects empty/oversized/unchanged/invalid or
secret-bearing payloads, and promotes payload plus integrity/freshness metadata
with atomic file replacement. It prints only provider ID, byte count, and
checksum. Ordinary tests never make network calls, and `go test -update` is not
a supported refresh path.

If a single raw payload would persist customer/account/region data or falsely
represent a multi-API/SDK source, record a concrete exception in
`internal/providers/fixture_policy.yaml`. The exception must name checked-in
wire/SDK test evidence. “Uses inline fixtures” by itself is not a reason.

## Required tests

At minimum, prove:

- authentication and effective endpoint selection;
- successful representative decoding and canonical definition/offering output;
- malformed/null/wrong-shape/duplicate/negative/overflow failure policy;
- unknown-field or schema-drift classification;
- provider/author separation and public/customer isolation;
- exact pricing units, zero versus absence, and modes when present;
- deep-copy isolation for retained canonical state;
- fixture integrity/freshness or the tested exception.

Run the narrow gate first, always with race detection:

```bash
go test -race ./internal/providers/<id>
go test -race ./internal/providers ./internal/providerfixture ./internal/providers/testhelper
go vet ./internal/providers/<id> ./internal/providerfixture
make provider-contract-check
make catalog-generation-check
make embedded-catalog-budget-check
git diff --check
```

Before closeout, run the repository-wide short race, vet, pinned lint,
generated-doc, catalog-generation, artifact/hash, diff, and complete verifier
gates required by the active control plane. Live calls and hosted checks are
reported separately from deterministic fixture proof.
