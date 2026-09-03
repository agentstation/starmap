# CAT9 proof: Starmap operator documentation

CAT9 owns the maintained Starmap text of the catalog-publisher campaign. It
documents the connected catalog runtime, every canonical setting, and every
published timestamp. It also documents the egress of each source kind, the
readiness fields, the catalog release tags, the Sigstore trusted root, and the
migration contract.

CAT9.1 owns the Starport text. CAT9.2 owns the runbook, the README link, and
the Kubernetes pair. `docs/proof/catalog-publisher/cat9.2.md` records CAT9.2.

## Fail before

The base commit is `ee65923c`. The maintained documentation named no catalog
setting and no readiness field of the connected runtime:

```console
$ git grep -c 'STARMAP_CATALOG_SOURCE' ee65923c -- 'docs/*.md' 'README.md'
$ git grep -c 'channel_freshness' ee65923c -- 'docs/*.md'
$ git grep -c 'source_check_freshness' ee65923c -- 'docs/*.md'
```

Each command printed nothing and exited 1. No maintained document described
the four layers, the startup policies, the hop budget, or the trusted root.

`docs/REMOTE_CATALOG_PROTOCOL.md` still called the connected path an opt-in
mode, and `docs/RELEASES.md` still called the canonical catalog tag a
payload-digest prerelease tag.

The stale-name scan of the base commit printed only historical records:

```console
$ git grep -n -E 'REFRESH_ON_START|REFRESH_INTERVAL|REMOTE_URL|REMOTE_API_KEY|REMOTE_ACTIVATION_INTERVAL' -- '*.md'
docs/proof/catalog-publisher/cat2-audit.md:136:...
docs/proof/catalog-publisher/cat2-review.md:65:...
```

## What CAT9 wrote

| Document | Content |
| --- | --- |
| `docs/ARCHITECTURE.md` | A connected-runtime section with the four layers, the source kinds and their safe identities, the startup policies, the canonical settings table, the timestamps, the freshness thresholds, the hop budget, the egress table, and the refresh lease |
| `docs/CATALOG_DISTRIBUTION_TRUST.md` | The Sigstore trusted-root section: how the verifier reads its root, how an operator refreshes it, and what a stale root does |
| `docs/REMOTE_CATALOG_PROTOCOL.md` | The fifth route, the source-chain document, the four cycle rules, and the connected constructor as the normal consumer path |
| `docs/REST_API.md` | The asynchronous update routes, the source-chain route, and every runtime readiness field |
| `docs/RELEASES.md` | The catalog tag namespaces, the channel document, and the historical readback rule |
| `docs/CLI.md` | The catalog source flags, the flag and environment precedence, and the credential separation |
| `docs/MIGRATING_TO_CATALOG_SOURCE.md` | The migration contract, the replacement of each removed setting, the policy choice, and the offline recipe |
| `docs/README.md` | The index entry and the quick link |
| `GLOSSARY.md` | Eighteen approved terms for the new vocabulary |

The migration contract states four facts. The old opt-in wording is gone. No
removed name is a runtime alias. A process that still reads a removed name
fails at startup. The canonical names live in `internal/catalog/settings`, and
Starport reads the same suffixes with the same defaults.

## The trusted-root finding

The code holds no runtime refresh path for the Sigstore trusted root. The
verifier reads the compiled document at
`internal/attestation/roots/sigstore-public-good-trusted-root.json` through
`attestation.DefaultTrustedRootJSON`. No environment name and no command flag
replaces that document. The only injection point is the Go option
`github.WithTrustedRoot`, which no non-test caller uses.

`docs/CATALOG_DISTRIBUTION_TRUST.md` states that plainly and gives the three
operator workarounds the code supports. An operator upgrades the binary. An
operator embeds the library and passes refreshed bytes to the option. An operator
verifies the bundle outside Starmap and loads the artifact through the `file`
source.

## The stale-name scan

The maintained text holds no removed name:

```console
$ git grep -n -E 'REFRESH_ON_START|REFRESH_INTERVAL|REMOTE_URL|REMOTE_API_KEY|REMOTE_ACTIVATION_INTERVAL' -- '*.md' ':(exclude)docs/proof' ':(exclude)docs/reviews'
```

The command prints nothing and exits 1.

The unrestricted scan of the CAT9 commit printed two lines. Both name a
historical record that the technical-writing policy excludes:

```console
docs/proof/catalog-publisher/cat2-audit.md:136:...
docs/proof/catalog-publisher/cat2-review.md:65:...
```

This record and `cat9.2.md` quote the same regular expression, so a later scan
also names these two proof files. The policy excludes proof records.

## Tests

CAT9 changes prose only, so it adds no Go test. The maintained-text gates below
carry the CAT9 evidence.

| Check | Scope | Result |
| --- | --- | --- |
| `technical-writing glossary check` | `GLOSSARY.md` | PASS, 77 terms, 0 errors |
| `technical-writing glossary update --check` | the repository | PASS, no missing candidate |
| `technical-writing lint --mode strict` | 758 files | PASS, 0 diagnostics |
| `make docs-check` | generated documentation | PASS, all documentation current |
| `verify-catalog-package-ownership.sh` | approved paths in current documentation | 13 passed, 0 failed |

## Commands

| Command | Result |
| --- | --- |
| `GOTOOLCHAIN=go1.26.6 make lint` | 0 issues, ago clean, technical writing PASS |
| `GOTOOLCHAIN=go1.26.6 make test` | every package passed |
| `GOTOOLCHAIN=go1.26.6 go tool ago -stale-ignores -format json ./...` | no finding and no stale ignore |
| `GOTOOLCHAIN=go1.26.6 make technical-writing-check` | PASS, 758 files, 0 diagnostics |
| `GOTOOLCHAIN=go1.26.6 make godoc` | no generated file changed |
| `GOTOOLCHAIN=go1.26.6 make docs-check` | all documentation is up to date |
| `bash scripts/verify-catalog-package-ownership.sh` | 13 passed, 0 failed |
| `shellcheck scripts/*.sh` | no diagnostic |
| `GOTOOLCHAIN=go1.26.6 bash scripts/verify-catalog-distribution.sh` | `Summary: 49 passed, 0 failed, 19 unverified.` |

The 19 unverified conditions need the Starport tree or the console toolchain,
which this worktree does not hold.
