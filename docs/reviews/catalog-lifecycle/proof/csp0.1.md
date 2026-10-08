# CSP0.1 evidence

CSP0.1 passes after renewed browser review of the integrated Starport branch.
The current E01 behavior tests and browser input hashes pass.
All 50 primary release cases remain UNVERIFIED.
Starport PR #366 merged the changes. The evidence below records the accepted review and merge.

## Changes

Static documentation at `/docs` no longer mounts the authenticated console shell.
The console access page links to operator documentation without requiring a session.
Protected navigation checks credential presence before the shell can start deployment queries.

Audience selection and section anchors survive copied links and browser history.
Documentation has its own theme control, readable typography, and keyboard focus indicators.
Code examples scroll inside the page. Settings instructions use larger text and stronger contrast.

The SDK instructions now require a gateway API key instead of the local console token.
The text separates exact cache hits from potentially paid semantic-cache lookups.
It names shared credentials and the remote console's gateway API key option correctly.

## Evidence

Starport source base: `042eb97851496ecdd1ef20bfa44fe3d84b86b2d8`.
Starmap plan base: `4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225`.
The [browser review](../../../plans/proof/starport-production-catalog/csp0.1/browser-review.json) binds the review to 220 source hashes and 17 retained evidence hashes.

| Check | Result | Record |
| --- | --- | --- |
| `pnpm --dir console run check` | Build and type check passed. 444 tests passed across 70 files. | [Raw result](../../../plans/proof/starport-production-catalog/csp0.1/console-check-contrast.json) |
| `verify-catalog-product.sh --task CSP0.1` | E01 passed. Nine named UI tests ran again. Current browser evidence matched its source hashes. | [Task result](../../../plans/proof/starport-production-catalog/csp0.1/task-verification-current.json) |
| `python3 scripts/test_catalog_product_verify.py` | 16 verifier tests passed. Missing, skipped, failed, and stale evidence cannot pass. | [Verifier tests](../../../plans/proof/starport-production-catalog/csp0.1/verifier-tests.json) |
| Browser layout and contrast | 12 views passed: three audiences, two widths, and two themes. Minimum text contrast: 4.63:1 across 764 samples. | [Measurements](../../../plans/proof/starport-production-catalog/csp0.1/browser-measurement-check.json) |
| Console and authentication guards | 111 structural conditions and documentation links passed. SPA and four named server tests passed with race detection. | [Repository checks](../../../plans/proof/starport-production-catalog/csp0.1/repository-checks.json) |
| CLI command review | Isolated build and four documented CLI help commands passed. | [Commands](../../../plans/proof/starport-production-catalog/csp0.1/command-checks.json) |

Body text measures 16 px with 26 px line spacing.
Headings measure 30 px and 22 px. Code and table text measure 14 px.
The reading column is at most 720 px. No view exceeds its viewport width.
Every documentation button measures at least 44 px in both dimensions.

Keyboard Enter selected an audience. ArrowRight scrolled a focused Python example.
The focus outline measured 2 px with a 3 px offset.
A direct operator fragment survived reload and remained within the visible page.

## Failures and corrections

The first test attempt lacked the generated route tree.
The normal console build generated it before behavior checks ran.
Four new behavior tests then failed against the original documentation route.
Those failures remain in [the baseline record](../../../plans/proof/starport-production-catalog/csp0.1/behavior-fail-before.json).

A protected navigation initially started two deployment queries before its redirect.
The render guard now checks credential presence before mounting the shell.
Existing small-screen shell tests now open a protected page because documentation has its own layout.
The tests retain their navigation and breakpoint assertions.

The light-theme table header initially measured 4.40:1 contrast.
Its stronger text role now exceeds the required 4.5:1 threshold.

## Limits

The verifier reruns UI behavior tests and validates recorded browser evidence hashes.
It explicitly reports that it does not repeat the manual browser interaction.
The review used the production UI build through local Vite preview.
Go tests provide separate HTTP authentication and SPA evidence.

This task does not qualify paid inference, native installers, public documentation, or 200 percent browser zoom.
Later plan tasks own those checks. The complete pre-PR roster and autoreview have not run.
The console build retains existing route-test discovery and large-chunk warnings.
The Starmap prose check still reports three diagnostics in unchanged historical storage proof logs.


## Visual revalidation after the main integration

The [current E01 result](../../../plans/proof/starport-production-catalog/csp0.1/publication-review/task-verification.json) passes all nine behavior tests and the recorded browser review.
The review covers all three audiences at 320 and 1280 CSS pixels, in both themes.
All 12 screenshots received visual inspection. No view overflows the document width.
Body text measures 16 px with 26 px line spacing. The reading column never exceeds 720 px.

The [measurements](../../../plans/proof/starport-production-catalog/csp0.1/publication-review/measurements.json) cover 764 rendered text samples, with a minimum contrast of 4.63:1.
Every control has at least 44 px height. Every code block supports keyboard focus and internal horizontal scrolling.
The [keyboard evidence](../../../plans/proof/starport-production-catalog/csp0.1/publication-review/keyboard-and-navigation.json) records tab selection, visible focus, code scrolling, fragment reload, and protected navigation.
Public documentation made no deployment-data requests in any measured view.

The previous review remains in `csp0.1/browser-review-pre-main-integration.json`.
The current review binds 220 source inputs and the renewed captures.
This local browser review does not qualify public hosting, released installers, paid inference, or 200 percent browser zoom.

## Implementation merge

Starport PR #366 merged at `cffa9300160d90eecb641b756a3ab2dd05d6ef47` on 2026-09-08.
All 16 checks passed at head `2973fd08122bc8264c499b99e60b44c890f3c6c7`.
The [merge record](https://github.com/agentstation/starmap/blob/af32c6ce2468bcb2260345881516e99a049ac892/docs/plans/proof/starport-production-catalog/pr-audit-2026-09-08/approved-merges/starport-366-merged.json) confirms the merge and exact source.
All 220 inputs from the accepted browser review match the merged source.
E01 retains its nine passing behavior tests and 12 inspected browser views.
CSP0.1 now meets the owner’s implementation-merge completion rule.

The earlier local-only and pending-review statements describe their original checkpoints.
Later README, Compose, and Starmap integration commits remain outside this merge.
