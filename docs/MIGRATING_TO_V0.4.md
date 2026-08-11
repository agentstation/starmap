# Migrate to Starmap v0.4.0

Starmap v0.4.0 is a direct pre-v1 compatibility break. It changes provider
YAML, catalog and manifest schemas, provider acquisition interfaces, and the
remote subscriber constructor. It does not include a runtime compatibility
path for v0.3 data.

## Prepare

1. Stop every process that can write the Starmap workspace or generation
   store.
2. Back up the provider YAML workspace and durable generation store.
3. Update code and custom provider YAML as described below.
4. Start v0.4.0 against a new or migrated schema-v5 workspace and a new
   manifest-v2 generation store.
5. Keep the backup until provider validation, catalog reads, and remote
   activation pass.

The default CLI paths are `~/.starmap/catalog` for provider YAML and
`~/.starmap/state/catalog` for durable generations. A deployment can use
different paths. Confirm its configuration before you move data.

The `starmap migrate catalog` command migrates the older directory layout. It
does not convert a schema-v4 catalog to schema v5.

## Migrate provider YAML

Replace `api_key`, `env_vars`, and `catalog.auth` with one provider-level
`credentials` contract. This example defines one API-key profile for both
catalog acquisition and inference:

```yaml
credentials:
  fields:
    - id: api-key
      kind: secret
      required: true
      environment:
        - OPENAI_API_KEY
  profiles:
    - id: api-key
      primitive: api-key
      fields:
        - api-key
      placements:
        - field: api-key
          kind: header
          name: Authorization
          scheme: bearer
  catalog_acquisition:
    required: true
    alternatives:
      - api-key
  inference:
    required: true
    alternatives:
      - api-key
```

Use these rules for each custom provider:

- Put conventional environment names in `credentials.fields[].environment`.
  Starmap checks them before the derived
  `STARMAP_<PROVIDER_ID>_<FIELD_ID>` name.
- Put request authentication in a named profile and select that profile in
  `catalog_acquisition`. Inference metadata is separate and contains no value.
- Replace endpoint `base_url_env_var` and `path` with a parameter field, one
  `endpoint_bindings` entry, and a complete URL template such as
  `{base_url}/v1/models`.
- Delete `catalog.authors`. Use `author_mapping` for a documented provider
  response field, and link each reviewed offering to an explicit
  `author/model` definition.
- Delete `feature_rules`. Use `capability_mappings` only when an exact typed
  response predicate and its provider contract prove each target capability.
  Model names and free text cannot create capability facts.
- Keep provider model IDs exact. Do not convert them into aliases or infer an
  author from the provider name.

Run `starmap validate` after the conversion. Run the intended provider update
only after validation succeeds.

## Replace incompatible durable data

v0.4.0 rejects pre-v5 payloads and manifest-v1 generations.
After you back up the old data, start v0.4.0 with an empty generation store. A
read-only client uses the verified embedded schema-v5 catalog until the first
explicit update commits a durable generation.

A remote deployment can seed an empty store with a verified v0.4.0
`PinnedBootstrap`. A valid durable current generation always takes precedence
over the pin.

Do not point a v0.3 process at the new workspace or store after migration.

## Update publication candidates

`starmap.NewCandidate` now accepts one evidence value:

```go
candidate, err := starmap.NewCandidate(catalog, starmap.CandidateEvidence{
    SourceObservations: observations,
    ReviewCandidates:   reviewCandidates,
})
```

Use an empty `starmap.CandidateEvidence{}` for a custom update that has no
external observations. Each review candidate must identify the exact source
observation that supplied its evidence.

## Update provider acquisition

Provider clients and raw fetchers now receive request-scoped credential
material:

```go
type ProviderClient interface {
    ListModels(context.Context, sources.ProviderCredentialMaterial) ([]catalogs.Model, error)
}
```

Configure a `sources.ProviderCredentialResolver` with
`sources.WithProviderCredentialResolver`, or use
`acquisition.NewProviderFetcher` for Starmap's built-in resolver. Use
`acquisition.WithCredentialResolver` to inject a deployment-owned resolver
into an acquisition syncer.

Remove calls to `sources.WithoutCredentialLoading` and
`sources.WithAllowMissingAPIKey`. A selected credential profile now returns a
typed resolution error when its required material is absent or invalid.

Remove use of the old provider API-key, environment-value, and provider
validation report types. Read `Provider.Credentials` for the secret-free
contract. Use `starmap providers` or `starmap providers --test` for operator
readiness checks.

## Update remote subscribers

Every remote subscriber now requires a caller-owned catalog store:

```go
subscriber, err := remote.NewContext(ctx, remote.Config{
    BaseURL:      baseURL,
    CatalogStore: store,
    PinnedBootstrap: pinned,
})
if err != nil {
    return err
}
```

Remove `PinnedBootstrap` when the deployment does not ship an offline
generation. `remote.New` remains a background-context convenience wrapper for
store I/O. Neither constructor starts a goroutine or sends a remote request.

Use `subscriber.State()` when catalog bytes and generation identity must stay
consistent. The returned state includes the catalog, generation ID, payload
checksum, generation timestamp, and process-local sequence.

## Verify the migration

Run these checks before production traffic:

```bash
starmap validate
starmap models list
starmap providers
```

For a Go consumer, run its complete test and race suites against
`github.com/agentstation/starmap@v0.4.0`. For a remote deployment, also test a
restart while the publisher is unavailable and confirm that the subscriber
serves the verified durable or pinned generation.
