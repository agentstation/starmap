# P8 Residual Deletion and Public-Surface Audit

Date: 2026-07-29

Reviewed base: `2269b556`

Status: `ACCEPTED`

## Outcome

The P8.6 deletion test removes public and importable implementation surfaces
that had no supported consumer contract. The candidate reduces 21
consumer-facing/importable library packages at the reviewed base to 16 while
preserving every named production composition:

- direct immutable-catalog library use;
- opt-in source acquisition;
- caller-owned generation storage;
- embeddable server and reactive remote subscriber;
- optional S3-compatible object storage; and
- deterministic catalog artifact creation and verification.

The change is prelaunch and intentionally provides no compatibility aliases.
It moves implementation behind `internal`, deletes dead behavior, and updates
the one affected wire resource from `snapshot` to the accurate `payload`
vocabulary.

## Internalized Implementation Packages

| Previous path | Current path | Named production owner | Disposition |
| --- | --- | --- | --- |
| `pkg/authority` | `internal/catalog/authority` | Catalog reconciliation and immutable read-view construction | Internal policy, not a consumer extension contract |
| `pkg/reconciler` | `internal/catalog/reconciler` | `internal/catalog/pipeline` | One concrete catalog compiler; no external customization case |
| `pkg/convert` | `internal/cli/convert` | CLI model export | Presentation adapter used only by CLI commands |
| `pkg/sourcepayload` | `internal/sourcepayload` | Provider/source decoders | Low-level quarantine machinery; plugin authors retain `sources.ValidateJSONPayload` and `sources.MaxJSONNestingDepth` |
| `pkg/constants` | `internal/constants` | Cross-package implementation defaults and resource bounds | Implementation policy, not a supported consumer API |
| `cmd/starmap/app` | `internal/cli/app` | `cmd/starmap` | CLI composition root |
| `cmd/starmap/cmd/*` | `internal/cli/commands/*` | `internal/cli/app` | Twelve CLI command implementations |

The constants bag was also reduced from 71 declarations to the 23 values with
real repository callers. The empty example data, documentation-only examples,
and 48 unused speculative timeout, retry, cache, logging, network, path,
format, default-value, and error-message constants were deleted.

## Deleted Behavior

- `differ.ApplyStrategy`, its four values, and
  `Changeset.Filter` were unused. The reconciler's `ApplyStrategy` method and
  inert `ApplyAdditive` configuration path existed only to feed that dead
  filter, so they were deleted together.
- The reconciler's generic `Strategy` interface and source-order adapter were
  also deleted. Source order was a test-only alternate policy that contradicted
  the one canonical authority table. Reconciliation now accepts the narrow
  authority reader and constructs one concrete authority strategy.
- `catalogs` no longer imports `testing` in production or exports nineteen
  fixture/assertion helpers. The only two callers now own small local fixtures.
- Logging test helpers moved to `internal/testlogging`; three unused helpers
  were deleted. Unused global convenience/configuration APIs were also
  removed. The retained package is the explicit zerolog/context diagnostics
  seam used by the root client, acquisition pipeline, providers, and
  transport.
- `catalogremote.SnapshotPath` and the `/snapshot` route became
  `PayloadPath` and `/payload`. The resource is an immutable canonical catalog
  payload bound to a generation manifest; it is not the public catalog type or
  a second snapshot abstraction.

## Retained Public Packages

| Package | Concrete composition that justifies it |
| --- | --- |
| `starmap` | Canonical immutable in-process catalog client and atomic publisher |
| `acquisition` | Explicit opt-in provider/models.dev synchronization |
| `catalogs` | Concrete immutable catalog, read views, and controlled builder |
| `catalogstore` | Small external store contract plus memory/filesystem/object stores |
| `catalogstore/s3` | Optional production S3-compatible conditional object backend |
| `server` | Embeddable HTTP/SSE server |
| `remote` | Reactive verified remote catalog consumer |
| `catalogremote` | Shared verified manifest/payload/SSE wire protocol |
| `catalogartifact` | Deterministic release/import bundle used by tooling and P9 |
| `catalogmeta` | Shared source, resource, observation, and projection vocabulary |
| `sources` | Public source/plugin observation and provider-client boundary |
| `sync` | Public acquisition options and observable result |
| `differ` | Public changesets carried by synchronization results |
| `provenance` | Public field evidence exposed by catalog readers/results |
| `errors` | Typed public failure contract |
| `logging` | Explicit zerolog and context diagnostics integration |

`server`, `remote`, and `catalogstore/s3` have small or zero in-repository
importer counts because their named consumers are the isolated external
composition modules. They are product modules, not hypothetical seams.

## Verification

Focused race verification passed:

```text
CGO_ENABLED=1 go test -race \
  ./internal/catalog/authority ./internal/catalog/reconciler \
  ./internal/sourcepayload ./pkg/differ ./pkg/logging ./pkg/sources ./pkg/sync \
  ./internal/cli/app ./internal/cli/convert ./internal/cli/commands/... -count=1

catalog authority 1.204s; reconciler 1.460s; sourcepayload 1.501s;
differ 1.187s; logging 1.982s; sources 1.699s; sync 1.853s;
CLI app 69.401s; every command package green.
```

The manifest/payload route and reactive transport passed under race:

```text
CGO_ENABLED=1 go test -race \
  ./pkg/catalogremote ./remote ./internal/server/... -count=1

catalogremote 1.262s; remote 21.444s; internal server 95.311s;
all server subpackages green.
```

After the final alternate-strategy deletion, ten race-enabled repetitions of
the reconciler and catalog pipeline passed (`1.988s` and `38.736s`).

The isolated external consumer matrix passed after all internalization:

```text
make test-consumer-deps

read-only: 31/32 non-standard packages, 153 total, forbidden families absent
store-only: caller-owned adapter publication passed; database/application implementations absent
server-embed: 241/260 packages, acquisition families absent
remote-subscriber: 225/240 packages, forbidden families absent
server-storage: 333/340 packages, filesystem/S3 reactive restart matrix passed
```

Structural scans find no current Go, workflow, script, or Makefile reference to
the five removed public paths, old importable CLI paths, dead differ APIs, or
the old snapshot route. Generated GoDoc, the CLI, README, architecture
documentation, and repository verification use the new paths and vocabulary.

The final generated-document-inclusive candidate then passed uninterrupted
`./scripts/verify.sh`:

- ordinary root `58.112s`, acquisition `22.224s`, internal server `34.400s`,
  catalogs `28.180s`, and models.dev `19.325s`;
- cgo-off library, store, S3, server, remote, and CLI compositions;
- cgo-enabled root race `251.308s`, internal server `105.122s`, catalogs
  `43.461s`, and models.dev `72.297s`; unchanged remote and public-server
  results were reused from the immediately preceding exact invocation;
- `BenchmarkClientCatalog` at `8.724–8.922 ns/op`, `0 B/op`, and
  `0 allocs/op`;
- `go vet`, pinned golangci-lint with zero issues, every coverage floor,
  generated documentation, diff checking, file-size policy, CLI build/smokes,
  and all 610 embedded catalog records.

## Residual Risk

The retained logging package still owns a process-wide default logger. Its
concurrency and configuration behavior belongs to the canonical-Go P8.8
review; P8.6 does not hide that concern behind another abstraction.

The breaking package and route changes are intentional before launch. There
is no migration wrapper to maintain, and the external composition matrix is
the compatibility boundary that matters.
