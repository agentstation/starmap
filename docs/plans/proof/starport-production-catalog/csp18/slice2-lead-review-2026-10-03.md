# CSP18 slice 2 lead review

Date: 2026-10-03. Reviewer: Fable 5.1, plan lead. Implementer: Opus 5.5 high.

## Scope

Branch `csp18-generated` in `/private/tmp/starport-csp18-generated-20261003`, base `c99f0f09`, head `0dd750b0`. The slice delivers decision 18-7 and decision 18-9: the generated settings and file reference, the `starport docs export` command, and the documentation release archive.

## Commits

- `9a1249fb` config: generate the settings and file reference from the loader
- `5f16a4ff` docs: commit the generated settings and file reference pages
- `d28e1625` cli: add starport docs export for the embedded documentation site
- `6bee09c3` release: publish the documentation site archive with its checksum
- `c6305391` console: link the identity setup guide to the embedded docs site
- `0dd750b0` config: mark the webhook signing secret as a secret (lead fix)

The squash merge hides the stale-file test failure at `9a1249fb` alone.

## Independent checks

- `go build ./...` exit 0. `go vet ./...` exit 0.
- `go test ./internal/config/... ./internal/cli/... ./internal/console/...` exit 0.
- `make lint` 0 issues before and after the lead fix.
- Technical-writing lint PASS on `settings.md` and `files.md`.
- The generator reads no environment variable and no configuration file. `WithEnvironment(nil)` is an empty-map lookuper.
- Each new standard-library use stays inside the `go 1.27.1` floor.

## Findings

1. `STARPORT_EVENTS_WEBHOOK_SECRET` had no secret tag. `starport config show` printed it, and the reference page said `Secret: No`. The lead added `secret:"true"` and regenerated the pages. An inventory must be truthful about secrets, so the fix is in scope.
2. `docs export --force` replaces a file at the same path and keeps other files. Accepted as a documented limit.
3. The docs archive has no SBOM, because it holds no software package. Accepted.
4. The loader reads the bootstrap variables before it decodes the env tags, so the generator cannot list them. The Configure content lists them by hand. Documented limit of the generator.
5. The manifest field `starport_release` matches the contract. No change.

## Merge coordination

- The docs branch must merge first or together. The Release Snapshot job needs `internal/console/dist/docs`.
- Both branches need `!docs/site/` in `.gitignore`.
- Both branches define `console.DocsFS()`. The docs branch now carries the slice 2 copy, so the rebase drops the duplicate.
- At the rebase, change the plain-text settings reference in `docs/site/configure/precedence.md` to a link to `../generated/settings.md`.
- The implementer ran `TestReferencePlatformMatchesUserDefaults` on macOS only. The CI matrix covers Linux and Windows.

## Result

Accepted at `0dd750b0`. The pull request waits for slice 1.
