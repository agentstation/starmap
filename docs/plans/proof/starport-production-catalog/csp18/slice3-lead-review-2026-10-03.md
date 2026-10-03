# CSP18 slice 3 lead review

Date: 2026-10-03. Reviewer: Fable 5.1, plan lead. Implementer: Opus 5.5 high.

## Scope

Branch `csp18-site-a11y` in `/private/tmp/starport-csp18-a11y-20261003`. The base is `feb5934a`, the slice 2 head. The slice delivers two decisions: 18-1 (prepare only) and 18-8, both in DECISIONS.md. It meets the accessibility targets of specification section 10.3 and proves the structural facts in jsdom tests. It adds the versioned docs Pages assembler and the manual Pages workflow, which stays disabled.

## Commits

- `d947cd19` console: meet the docs accessibility targets and test them
- `25052bc3` scripts: assemble the versioned docs Pages site from the release assets
- `c18b73fc` ci: prepare the manual docs Pages deployment
- `94d6f6f6` docs: describe the docs Pages publication and rollback
- `8354d91d` console: keep the docs header in a 320 px view and fragment targets below it (findings 1 and 2)
- `c9b81f9a` console: keep the sticky docs columns below the header at the page end (finding 3)

## Independent checks at `94d6f6f6`

- `pnpm -C console check` exit 0: 83 test files, 536 tests.
- `make build` exit 0.
- `verify-action-pins`, `test-docs-pages-assembler`, `verify-release-workflow`, `verify-doc-links`, `verify-console-polish`, `verify-v1-architecture`, `verify-package-layout`, and `verify-starmap-ownership`: exit 0.
- `make lint` 0 issues. `go test ./internal/console/... ./internal/server/...` exit 0.
- The worktree stayed clean after every check.

## Browser method

The Chrome extension was not connected. The lead drove a real Chromium 151.0.7922.34 through Playwright 1.62.1 in headless mode. The lead built the gateway from the reviewed head and ran two instances. The open one listens on port 18731 with `--no-auth`. The guarded one listens on port 18732 with authentication on.

The review script records 23 observations with measured evidence and full-page captures. The script and the captures are in `browser-review-2026-10-03/`.

## Findings at `94d6f6f6`

1. At a 320 px viewport, `form.site-search` kept its content width of 310 px. The document scrolled 6 px sideways, and 17 px with the text-spacing overrides. The flex item lacked `min-width: 0`. Fixed in `8354d91d`.
2. At 64rem and wider, the header wraps to two rows. It measures 109 px at 1024 and 1280 wide. The `scroll-padding-top` value stayed at the one-row value of 72 px. A fragment load and a table of contents link put the heading under the header. `#steps` landed at 88 px. Fixed in `8354d91d` with a measured offset. See the design note below.
3. At the end of a long page at 64rem and wider, the sticky navigation column moves under the header. Its containing block ends at the footer. The measurements are in the list below. Tab can focus a link under the header, and the browser does not scroll it into view. WCAG 2.2 AA 2.4.11 applies. Sent back and fixed in `c9b81f9a`, see the result below.

### Finding 2 design

A resize observer writes the sticky header height to `--docs-header-offset`. The scroll padding and the sticky columns read it. The stylesheet value stays as the fallback without the script. The table of contents read line follows the same height.

The lead accepts the design. A single-row header cannot hold the build facts, the selector, and the toggle at 1024 px. That layout needs `nowrap` or `overflow: hidden`, which the stylesheet test forbids.

### Finding 3 measurements

Measured at the end of the recovery topic in a 900 px viewport:

- 1024: nav top 13, header bottom 109.
- 1280: nav top -29, header bottom 67.
- 1440: nav top -39, header bottom 57.

## Checks at `8354d91d`

- `pnpm -C console check` exit 0: 83 test files, 538 tests. Two new a11y tests cover the shrinkable search form and the measured header offset.
- The 8 verify scripts above: exit 0. `make lint` 0 issues. `go test` on console and server: exit 0. `make build` exit 0.
- Browser review: 22 of 23 observations pass. Reflow at 320 shows `scrollWidth` 320 on all three pages in both themes, with and without text spacing. Zoom 200 and 400 pass. Fresh loads and reloads of `#steps` and `#coordinated-recovery` land 32 px below the header at 1024, 1280, and 1440.
- The one failure was the review script's own sequence, not the fix. See the limit below.

## Measured limit: reload after a window resize

The review script navigated to a fragment, resized the viewport, opened another fragment in the same document, and reloaded. In 2 of 10 trials, Chromium's history scroll restoration landed after `restoreFragment` and left the page at the clamped old position (`#steps` at -172 px, `scrollY` 2939). A fresh load of the fragment address and a reload in place without a resize were stable in every trial. The review script now measures the fresh-load flow and the reload in place. The record states this limit. No product change: a `history.scrollRestoration` change would also change what a reload after a scroll does, which is a separate product decision.

## Checks at `c9b81f9a`

- `pnpm -C console check` exit 0: 83 test files, 539 tests. One new a11y test proves the column fit and the measured footer offset.
- The 8 verify scripts: exit 0. `make lint` 0 issues. `go test` on console and server: exit 0. `make build` exit 0.
- Logs: `csp18-a11y-lead3/` in the session scratch directory. The worktree stayed clean.
- Browser review against a gateway built from this head: 24 of 24 observations pass. The script gained `sticky_columns_page_end`.

### Finding 3 result

The client measures the footer and writes its height to `--docs-footer-offset`. Both sticky columns subtract it and the body padding from their maximum height. The stylesheet value of 5rem is the fallback without the script. The lead accepts the design. The columns reserve the footer height at every scroll position, so the navigation shows up to 80 px less before its own scroll. The record lists that trade-off as a limit.

Measured at the end of the recovery topic in a 900 px viewport, both columns and the first link in focus:

- 1024: nav top 125, header bottom 109, first link 125 to 169.
- 1280: nav top 83, header bottom 67, table of contents top 83, first link 83 to 127.
- 1440: nav top 73, header bottom 57, table of contents top 73, first link 73 to 117.

## Result

Accepted at `c9b81f9a`. The slice rebased to `69c3dd76` after the #410 merge and to `68ed84df` after the #407 merge. Neither rebase changed a reviewed input. Starport PR #411 merged on 2026-10-03 as `1f18b216` at that exact head with 42 of 42 checks green. One job needed a rerun after a GitHub HTTP 503. The merge record is `starport411-merge-2026-10-03/`.

## Deviations accepted as documented limits

- `h3` renders at 18 px, below the 20 px heading target. The 20 px target applies to `h2` section headings. `h3` keeps weight 600 and a clear step below `h2` and above `h4`.
- On an archived version page, the version selector lists only that build.
- The `include-hidden-files: true` upload keeps the `.nojekyll` marker in the Pages artifact.
- The first manual Pages run sets the Pages source. The owner enables Pages and runs the workflow under separate authorization (decision 18-1).
