# Migrate to the catalog source runtime

The catalog source runtime replaces the earlier remote-consumption settings. It
is a direct pre-v1 break in configuration and in startup behavior. It changes no
catalog schema, no generation manifest, no artifact bytes, and no wire protocol.
Existing stored generations and published catalog releases stay valid.

## What changed

Remote catalog consumption was an opt-in mode. A process made no catalog request
at startup, and an operator turned a separate remote mode on.

A connected runtime now reads a catalog source at startup under a startup
policy. `starmap.Open` returns that runtime. The default source is the public
`agentstation/starmap` channel on GitHub, and the default startup policy is
`prefer_source`.

`starmap.New` still opens no connection and makes no request. Importing the root
package still enables no network work. The connected behavior belongs to the
explicit `Open` constructor only.

## The migration contract

1. The old opt-in wording is gone. A connected process reads its source at
   startup.
2. No removed name is a runtime alias. No removed name selects behavior.
3. A process that still reads a removed name fails at startup. The error names
   the removed setting, so no silent default replaces an operator choice.
4. The canonical names live in `internal/catalog/settings`. Starmap reads them
   under the `STARMAP_` prefix and Starport reads the same suffixes under the
   `STARPORT_` prefix, with the same defaults.

## Replace each removed setting

| Removed setting | Canonical replacement |
| --- | --- |
| The switch that made a startup catalog read opt-in | none. Use `CATALOG_SOURCE_STARTUP_POLICY`, which defaults to `prefer_source`. |
| The refresh period of the old remote mode | `CATALOG_SOURCE_POLL_INTERVAL`, default `1h` |
| The endpoint of the old remote mode | `CATALOG_SOURCE_URL`, with `CATALOG_SOURCE` set to `starmap` |
| The credential of the old remote mode | `CATALOG_SOURCE_API_KEY` |
| The activation period of the old remote mode | none. The runtime activates each verified generation when it arrives. |

The Starport configuration reference lists the exact removed spellings and the
startup error text for each one. Read it before an upgrade.

## Choose a startup policy

| Policy | Choose it when |
| --- | --- |
| `prefer_source` | The process must start fast and may serve the embedded baseline for a short time. |
| `require_source` | The process must never serve the embedded baseline. |
| `prefer_local` | The process must keep its retained generation until an operator refreshes it. |

## Keep a process offline

Set `CATALOG_SOURCE` to `embedded` and set `CATALOG_ACQUISITION_ENABLED` to
`false`. The process then makes no catalog request and no provider request. It
serves the verified catalog that its binary compiles in.

`file` is the other offline source. It reads one verified catalog artifact from
a local path, which suits an air-gapped host that receives artifacts through a
review process.

## Read the new operator surfaces

- [ARCHITECTURE.md](ARCHITECTURE.md#connected-catalog-runtime) lists every
  setting, every default, every layer, every timestamp, and the egress of each
  source kind.
- [REST_API.md](REST_API.md#runtime-readiness-fields) lists the runtime
  readiness fields, including `channel_freshness`, `source_check_freshness`,
  `instance_identity`, and `chain_hops`.
- [ENTERPRISE_CATALOG_SERVER.md](ENTERPRISE_CATALOG_SERVER.md) is the runbook
  for a central catalog server.
- [CATALOG_DISTRIBUTION_TRUST.md](CATALOG_DISTRIBUTION_TRUST.md) holds the
  Sigstore trusted-root procedure.
