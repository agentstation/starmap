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

## Rebase onto slice 1

The lead rebased the branch onto `csp18-docs` at `f99695f8` with no conflict. The duplicate `docs_fs.go`, `docs_fs_test.go`, and `.gitignore` lines dropped out. The lead changed the precedence page reference to a site link and kept the committed generated pages in the site test fixture. The result is `703d2988` with 6 commits.

Checks at `703d2988`:

- `go build ./...`, `go vet ./...`, and `make lint` with 0 issues.
- `go test` on `internal/config`, `internal/cli`, `internal/console`, and `internal/server`: exit 0.
- `pnpm -C console check` exit 0: 82 test files, 529 tests, 47 pages, 426 search sections.
- `goreleaser release --snapshot --clean --skip=notarize` exit 0 in 1 m 55 s.
- `verify-release-binaries.sh dist` and `verify-release-archives.sh dist`: PASS, 5 archives, 5 SBOMs, the docs archive, and the checksum manifest.
- `verify-release-workflow`, `verify-doc-links`, `verify-console-polish`, `verify-v1-architecture`, `verify-package-layout`, and `verify-starmap-ownership`: exit 0.

## Rebase onto main

After the slice 1 squash merge at `e7a58541`, the lead moved the branch onto `main` with `git rebase --onto`. The result is `b5e24637` with 6 commits and no content change against `703d2988`. Draft PR #410 opened at `b5e24637`.

Checks at `b5e24637`:

- The pre-PR roster passed with 35 checks. The check `goago` has its usual exit 2 and the other 34 have exit 0. The full `go test ./...` took 991 s. The head stayed unchanged and clean.
- Structured review (Sol 6.1 high): no findings, patch correct 0.97.
- CI run 37136565909: the `Security Scan` job failed. The other jobs stayed green while the run continued.

## Finding 6: gosec file modes

The `Security Scan` job runs gosec 2.29.0. It reported G301 and G306 in `scripts/docsgen/main.go` for the 0755 directory and the 0644 files. The generator writes committed documentation source, so the modes are correct. The lead added two `#nosec` comments with the reason, in the style of `internal/cli/docs.go`. The fix is `feb5934a`. The diff holds comments only, so the roster at `b5e24637` still covers the behavior. The lead reran `gofmt`, `go build ./...`, gosec on the touched packages, `make lint`, and the generator at `feb5934a`, all with exit 0 and no stale page.

The structured review at `feb5934a` reported no findings, patch correct 0.98. CI run 37138162238 started at `feb5934a`.

## Result

Accepted at `feb5934a`. PR #410 merges at that head when CI run 37138162238 completes green. The merge record follows in `starport410-merge-2026-10-03/`.
