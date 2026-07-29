# P8 Go Naming and Package-Family Audit

Date: 2026-07-29

Scope: P8.4 and the naming portion of F-022

## Standard

The audit applies normal Go naming rules with the repository's depth and
deletion tests:

- package names provide context, so exported identifiers should not repeat them
  without adding meaning;
- file names identify the concept implemented inside the package;
- `util`, `utils`, `helper`, `common`, `manager`, and `service` directories
  require a concrete invariant and are deleted when they only collect
  one-liners;
- a family of similarly prefixed packages is retained only when each member has
  a distinct dependency boundary and production composition; and
- because Starmap has not launched, corrections are clean breaks. No aliases,
  deprecated wrappers, or compatibility shims preserve the old names.

## Corrected identifier and file stutter

| Before | After | Reason |
| --- | --- | --- |
| `provenance.ProvenanceFile` | `provenance.File` | The package already supplies the missing noun; the old name required a linter suppression |
| `provenance.Provenance` | `provenance.Entry` | One value is a field-history entry; the package supplies its provenance context |
| `pkg/provenance/provenance.go` | `pkg/provenance/tracking.go` | The file implements tracking, conflict reporting, and persisted entry history rather than the package in the abstract |
| `pkg/provenance/provenance_fuzz_test.go` | `pkg/provenance/file_fuzz_test.go` | The fuzz target exercises the persisted file envelope |
| `format.Format` | `format.Kind` | The value identifies which output representation to render |
| `format.Formatter` / `FormatterFunc` | `format.Renderer` / `RendererFunc` | The package supplies “format”; the interface writes a rendered value |
| `format.NewFormatter` | `format.New` | The package and return type supply constructor context |
| `internal/cli/format/formatter.go` | `internal/cli/format/renderer.go` | The file owns the renderer interface and implementations |
| `catalogartifact.Artifact` | `catalogartifact.Bundle` | The product contains both the reproducible archive and detached statement, so “bundle” is more accurate and avoids repetition |
| `pkg/catalogartifact/artifact.go` | `pkg/catalogartifact/bundle.go` | The file implements the bundle and deterministic codec |
| `pkg/catalogartifact/artifact_test.go` | `pkg/catalogartifact/bundle_test.go` | Tests now name the product under test |
| `pkg/catalogremote/remote.go` | `pkg/catalogremote/client.go` | The file owns the verified protocol client; “remote” merely repeated its package |
| `pkg/catalogremote/remote_test.go` | `pkg/catalogremote/client_test.go` | Tests now name the client behavior under test |

`catalogs.Catalog` remains deliberately unchanged. It is the concrete immutable
product required by the canonical public DX, and the package contains many
other catalog-domain types rather than being a package named `catalog`.
`catalogstore.Store` also remains unchanged: it is the small implementation-
neutral contract, and the independent review explicitly found this name clear.
`catalogs.CatalogPayload` and `NewCatalog` retain “catalog” because they
distinguish the complete publication representation and immutable final
product from the same package's observation catalog and mutable builder.

## Generic helper result

The original `internal/utils/ptr` target was a three-call collection of pointer
one-liners. P6.5 deleted the directory and its callers instead of renaming it.
The current tree contains no `util`, `utils`, `helper`, `helpers`, `common`,
`manager`, or `service` directory. Test-only `testhelper` is retained because
it owns provider protocol fixtures and is named for its non-production role.

Exact package-name filenames that remain are either:

- standard `main.go` command entrypoints;
- the primary concrete adapter or server entrypoint, such as `server.go`,
  `app.go`, or `local.go`; or
- a deep domain implementation, such as `authority.go`, `differ.go`, or
  `reconciler.go`.

Renaming those files would only replace a precise central concept with another
generic word. P8.3 separately reviewed the size and locality of `differ.go`.
P8.5 and P8.6 still own complexity and deletion decisions; this naming audit
does not grant unused behavior permanence.

## The original seven `catalog*` packages

At reviewed baseline `9508ee78`, the public prefix family was:

1. `pkg/catalogartifact`
2. `pkg/catalogdistribution`
3. `pkg/catalogmeta`
4. `pkg/catalogremote`
5. `pkg/catalogs`
6. `pkg/catalogscheduler`
7. `pkg/catalogstore`

P6.5 deleted `catalogdistribution` and `catalogscheduler` after production-call
and composition review. The current family has five members:

| Package | Direct production importers | Disposition and boundary |
| --- | ---: | --- |
| `catalogartifact` | 2 | Retain for the deterministic archive/attestation bundle, release CLI, embedded-budget verification, and P9 release-import path |
| `catalogmeta` | 9 | Retain as the small cycle-breaking home for source identity, observation status, and projection result value types shared below `catalogs` |
| `catalogremote` | 3 | Retain as the versioned manifest/payload/SSE wire client used by server handlers, SSE, and the public reactive subscriber |
| `catalogs` | 42 | Retain as the canonical immutable catalog, builder, schema, indexes, and read views |
| `catalogstore` | 10 | Retain as the generation/CAS contract plus memory, filesystem, generic object, and optional S3-backed implementations |

These names repeat the domain prefix intentionally because they are independent
public import paths and commonly appear beside unrelated `artifact`, `meta`,
`remote`, or `store` packages in embedding applications. Collapsing them would
reverse proven dependency isolation: read-only consumers must not acquire
remote transport, object storage, release packaging, or server dependencies.

`internal/catalog` is a namespace directory, not a Go package. Its `pipeline`,
`query`, and `workspace` children have separate acquisition/reconciliation,
read-query, and transactional human-workspace invariants. Likewise,
`internal/embedded/catalog` is embedded data, not a Go package.

## Verification

The following checks pass on the naming candidate:

```text
go test -race ./pkg/provenance ./pkg/catalogs ./pkg/reconciler \
  ./pkg/catalogartifact ./pkg/catalogremote -count=1
catalogremote 1.273s
catalogartifact 1.677s
reconciler 1.905s
catalogs 21.070s
provenance passed in the preceding focused race run at 1.182s

go test -race ./internal/catalog/... ./internal/cli/... \
  ./cmd/starmap/cmd/... -count=1
all packages passed; pipeline 6.631s, query 2.292s, workspace 6.588s

make generate
make docs-check
git diff --check
```

Exact source scans find no `provenance.ProvenanceFile`,
`provenance.Provenance`, `format.Formatter`, `format.Format`,
`format.NewFormatter`, `catalogartifact.Artifact`, or old same-name file path.
Generated GoDoc now links the corrected types and filenames. No compatibility
alias or duplicate implementation remains.
