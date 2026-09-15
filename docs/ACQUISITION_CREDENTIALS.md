# Catalog acquisition credentials

Starmap resolves credentials only for explicit catalog acquisition and credential checks.
An explicit source reference takes precedence over environment discovery.
Ambient discovery tries `STARMAP_<PROVIDER>_<FIELD>` before the conventional names declared by the catalog.
An empty selected value causes an error. It does not expose a lower-priority credential.

Supported references use environment variables, files, AWS Secrets Manager, GCP Secret Manager, Azure Key Vault, Vault, or OpenBao.
Applicable cloud identity chains remain available. One complete profile uses one response version for each secret resource.
Resolution uses a thirty-second deadline or the caller's shorter deadline.
Known expired or denied material cannot pass resolution. A failed refresh returns an error without returning stale material.

## Upgrade selection

Persistent installations retain a selection policy for each migrated provider.
Before a legacy provider adopts the new precedence, Starmap compares both complete selections from the same source responses.
Different material causes a provider conflict. The error names the competing variables and excludes their values.
An explicit reference or removal of the conflicting variable resolves the choice.
Identical material permits migration without a conflict.

Fresh installations use the new precedence immediately.
Once migration succeeds, later configuration changes follow the accepted policy.
Policy records contain ownership, schema, provider identity, and policy version. They contain no credential values or value hashes.

## Policy storage

The standalone application stores policy under `<state>/credentials/<deployment-id>/<instance-id>/`.
`STARMAP_STATE_ROOT` selects the state root. `STARMAP_HOME` supplies `<home>/state` when no specific state root exists.
`starmap config paths` reports the selected location and access policy.

| Platform | Default state root |
| --- | --- |
| Linux | `$XDG_STATE_HOME/starmap`, otherwise `~/.local/state/starmap` |
| macOS | `~/Library/Application Support/starmap/state` |
| Windows | `%LOCALAPPDATA%\starmap\state` |

`policy.json` retains the installation default.
`provider-<provider-id-sha256>.json` records each accepted provider migration from that default.
The filename hash identifies the provider. It does not represent credential material.
Each record has a 4,096-byte limit and requires canonical encoding and matching ownership.
Publication uses the private-file journal under `.record-publications/`.

These files require owner-only access. Preserve the directory with deployment backups and filesystem migrations.
Do not edit or delete policy records to resolve a credential conflict.
Use an explicit source reference or remove the conflicting environment variable.
An existing catalog pointer, instance seed, or baseline directory without a policy record selects the legacy migration path.

## Go composition

`acquisition.OpenCredentialResolver` exposes the same acquisition secret sources to embedding applications.
Pass `CredentialResolverConfig.References` to select provider fields explicitly.
`CredentialResolverConfig.State` selects private policy storage and binds it to a product, deployment, and instance.
The host must set `LegacyInstallation` from its existing deployment state when no policy record exists.
Existing records take precedence over this initialization hint.

Omitting `State` creates an ephemeral resolver with the current Starmap precedence.
Construction reads no credential source. Only explicitly selected policy storage causes filesystem access during construction.
Pass the resolver to `acquisition.WithCredentialResolver` or the provider fetcher's resolver option.
The API does not inspect inference credentials or account repositories.
Starport's ambient namespace and configuration authority require separate host integration.
