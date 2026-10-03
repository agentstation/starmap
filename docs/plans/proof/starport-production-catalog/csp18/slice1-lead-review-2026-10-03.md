# CSP18 slice 1 lead review

Date: 2026-10-03. Reviewer: Fable 5.1, plan lead. Implementer: Opus 5.5 high, with two Opus 5.5 content writers.

## Scope

Branch `csp18-docs` in `/private/tmp/starport-csp18-docs-20261003`, base `c99f0f09`. The slice delivers decisions 18-2 through 18-6, 18-8 (site inputs), and 18-10: the documentation site content, the prerender build, the `/docs/` route, and the CPL re-point.

The lead reviewed head `01483956` (4 commits). The implementer then rewrote the unpublished branch into 6 commits at `f99695f8` to add the slice 2 reconciliation. The rewrite changed 6 files: `.gitignore`, `configure/precedence.md`, `docs_fs.go`, `docs_fs_test.go`, `spa.go`, and `spa_test.go`.

## Commits at `f99695f8`

- `7ef07e44` docs: track the documentation site source under docs/site
- `0744ad28` docs: add the operator documentation site content
- `38296495` console: prerender the documentation site into dist/docs
- `15085f2e` server: serve the documentation site at /docs/ without a session
- `f19346ab` console: open the static documentation site from the console
- `f99695f8` ci: point the CPL-V44 and CPL-V45 conditions at the static site

## Independent checks at `01483956`

- `go build ./...` exit 0. `go vet ./...` exit 0.
- `go test ./internal/console/... ./internal/server/... ./internal/cli/...` exit 0.
- `make lint` 0 issues.
- `pnpm -C console run check` exit 0: lint, build, typecheck, 82 test files, 529 tests.
- `scripts/verify-doc-links.sh` exit 0 on all 44 pages.
- `scripts/verify-console-polish.sh` exit 0, 48 passed. The diff changes only CPL-V44 and CPL-V45.
- Structured review (Sol 6.1 high, two passes): no findings, patch correct 0.98.
- Structured review at `f99695f8` (Sol 6.1 high, two passes): no findings, patch correct 0.98.
- Integration build with the slice 2 generated pages copied in: exit 0, 47 pages, 425 search sections. The configure area renders `settings/` and `files/`. The manifest holds the five contract fields and 57 files. The lead removed the copies after the build.

## Implementer checks at `f99695f8`

- `make lint` 0 issues. `go build ./...` exit 0.
- `pnpm -C console check` exit 0: 82 files, 529 tests.
- `go test ./internal/console/... ./internal/server/...` exit 0: 689 pass, 4 skip.
- `verify-doc-links.sh` exit 0 on the branch and on a trial merge with `csp18-generated`. The trial merge has no conflict.
- `verify-console-polish.sh` 48 of 48, `verify-v1-architecture.sh` 11 of 11, `verify-starmap-ownership.sh` 12 of 12, `verify-package-layout.sh` exit 0.
- Technical-writing lint PASS on all 44 pages. All 113 setting names and 109 commands in the pages exist in the code.

## Review points

- The docs handler needs no session. It guards the path with `fs.ValidPath`, sets the console CSP on HTML, and serves a 503 notice without a build.
- The renderer rejects `blob/main` and `tree/main` links. The only remaining `blob/main` strings are the rejection test fixture and the identity screen that slice 2 re-points.
- The build makes no network call. A no-session Go test covers the route.
- The release label comes from `STARPORT_DOCS_RELEASE`, then a tag ref, then `dev`. The Dockerfile passes the version and the commit.
- `configure/precedence.md` lists by hand the bootstrap variables that the loader reads before the env tags. The list matches `loader.go` and `paths.go`.

## Findings

1. Both slices defined `console.DocsFS()`. The rewrite moves it to `docs_fs.go` with the slice 2 shape and test. Resolved.
2. Slice 1 narrowed `.gitignore` to `/site/`. Slice 2 needs `!docs/site/`. The rewrite carries both lines. Resolved.
3. 26 links point at repository Markdown such as `OPERATOR-GUIDE.md` and `RECOVERY.md`. They render as pinned GitHub links and do not open offline. Accepted as a documented limit.
4. The release recovery job downloads the `release-dist` artifact of the source run and rebuilds nothing. A recovered release keeps the docs label of its source run. No workflow change.
5. `scripts/verify-starmap-ownership.sh` needs `STARMAP_OWNERSHIP_STARMAP_ROOT` in a `/private/tmp` worktree. No script change.

## Follow-ups outside CSP18

The writers compared the code with the repository Markdown and reported conflicts. The site states the code behavior. The old Markdown keeps the stale facts until a later task rewrites it.

- Writer A: 12 facts that the writer could not verify and 11 conflicts. Examples: no `GET /api/v1/providers/status` route, a `PUT` credentials route documented as `POST`, and a stale `internal/config/README.md`. The file `.env.example` states a request limit of 10,485,760, and the code holds 33,554,432.
- Writer B: 16 conflicts. Examples: `route_validation` values, `backup adopt` against `FLEET_INITIALIZATION.md`, `SOURCE_REFRESH_MODE`, the generation pin, and the `catalog/v2` default.
- Catalog channel: Starport defaults to `catalog/v1` (`internal/config/catalog.go:49`). The pinned Starmap names `catalog/v2` as its publication channel. The site states the Starport default. The acceptance cases pass against the released pair, so the default works. The owner of the acquisition contract decides the channel name.
- Possible defect, not verified: `loader.go` passes the raw `STARPORT_CONFIG_ACCESS` value to `productpaths.ReadConfiguration` instead of the checked value.
- `gofmt -l` lists `internal/recovery/catalog_expiring_preparation_test.go` on `main`. The branch does not touch it.

## Merge coordination

- This branch merges first. The slice 2 branch rebases onto it and drops its own copies of `docs_fs.go`, `docs_fs_test.go`, and the `.gitignore` lines.
- At the rebase, slice 2 changes the plain-text settings reference in `configure/precedence.md` to a link to `../generated/settings.md`.
- The Release Snapshot job builds the site from this branch. The slice 2 docs archive needs it.

## Result

Draft PR #409 is open at `f99695f8` with CI run 37130769293 pending. The lead roster at `f99695f8` passed with 35 checks. The check `goago` has its usual exit 2 and the other 34 have exit 0. The full `go test ./...` took 935 s. The head stayed unchanged and clean.

CI run 37130769293 completed with 42 checks green. PR #409 merged at `e7a58541` on 2026-10-03 with the reviewed tree equal to the merge tree. The proof is in `starport409-merge-2026-10-03/`.
