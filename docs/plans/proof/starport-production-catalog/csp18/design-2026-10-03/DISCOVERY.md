# CSP18 discovery: versioned embedded and public documentation

Explorer record, 2026-10-03. Read-only. Starport main at `c99f0f09` (matches origin).
Verifier tree `/private/tmp/starmap-acceptance-20261003` at `344eb64cf`.

Path abbreviations:

- `PLAN` = `starmap-catalog-requirements/docs/plans/starport-production-catalog-plan.html`
- `MAP` = `starmap-catalog-requirements/docs/plans/proof/starport-production-catalog/acceptance-map.json`
- `SPEC` = `starmap-catalog-requirements/docs/design/catalog-lifecycle/ENGINEERING_SPEC.md`
- `VER` = `/private/tmp/starmap-acceptance-20261003/scripts/catalog_product_verify.py`
- `REG` = `/private/tmp/starmap-acceptance-20261003/scripts/catalog-product-checks.json`
- `SP` = `/Users/jack/src/github.com/agentstation/starport`

## Answer first

- CSP18 has 10 verifier subcases. 2 pass now: `A30.instruction_contrast` and `A24.generated_file_inventory`. Both came with CSP17. The other 8 have no registry entry, so they report UNVERIFIED. Predicted `--task CSP18` result: gate FAIL, exit 1. This is not a live run. See "Fail-before".
- The plan's fail-before text is partly stale. CSP0.1 (Starport #366) already made `/docs` public on the client and added h2 anchors. The real gaps are different. Docs need a running gateway. No offline export exists. No content search exists. No version or content manifest exists. No generated settings or file inventory pages exist. The E01 browser proof is stale.
- No GitHub Pages site exists for `agentstation/starport` (`gh api repos/agentstation/starport/pages` returns 404, `has_pages` false, no environments). Publication is an owner decision and belongs to CSP23. CSP18 can only prepare an unpublished artifact and workflow.
- 8 owner decisions block design. The largest are the docs build tool, the version path and retention scheme, the static recovery entry form, and the A30 browser proof method.

## Task statement

From `PLAN:779-789` (task-CSP18):

> Problem: "Short embedded TSX docs drift from detailed repository instructions and require console access."
>
> Owning concept and paths: SP · docs content tree, docs build, console docs route, static recovery entry.
>
> Depends on: CSP1, CSP8, CSP16, CSP17.
>
> Steps:
> 1. Build one versioned content tree, generated settings and file inventories, and offline search.
> 2. Prepare the proposed Pages artifact, protected deployment workflow, version paths, and rollback.
> 3. Keep static docs public and dynamic deployment data authenticated.
> 4. Verify the CSP18 discovery audit criteria before task completion.
>
> Acceptance: "A30 and local A29 subcases pass. The release owner verifies hosting configuration. CSP23 supplies public-URL evidence for the complete A29 case."
>
> Fail-before: "Record auth-gated docs, absent heading anchors, and no article-content search."
>
> Verification: `pnpm --dir console run check` (SP), then `bash scripts/verify-catalog-product.sh --starport-root "$CSP_STARPORT_WORKTREE" --task CSP18` (SM).

Ledger: `PLAN:258` shows CSP18 as `todo`, "UNVERIFIED. Planned proof: csp18.md". `PLAN:15` says "CSP18 follows" CSP16.2.

Extra scope that other records assign to CSP18:

- Discovery audit, `discovery-readiness-2026-09-07/contracts.md:34`: "Explain permitted discovery, structural support, caller readiness, and cache freshness in embedded and public documentation. Name the released dependency that supplies canonical reconciliation." The MAP mapping DR07 lists tasks CSP17, CSP18, CSP20 and cases A18, A34.
- Unrecognized-journal guidance: `PLAN:225` (CSP2 row) and `csp2.md:7` say "CSP18 owns unrecognized-journal guidance." `SPEC:3287`: "A replaced lock, unknown journal, changed entry, or unrecorded stage remains preserved."
- Static recovery docs: `SPEC:1821` "Diagnostics and static recovery docs remain available." `SPEC:3549` "Diagnostics stay authenticated. Static recovery instructions remain available separately." `REPOSITORY_FINDINGS.md:396` (G18).
- Downstream: CSP19 depends on CSP18 (`PLAN:793`). CSP20 depends on CSP18 (`PLAN:805`). CSP23 must pass A06 and A29 against the public docs URL (`PLAN:846`).

Spec targets (`SPEC` §10.2, about lines 3562-3630; §10.3, about 3631-3667):

- One authored content tree in Starport builds both the public site and the embedded docs.
- Generated configuration pages use the pinned Starmap shared settings contract and Starport descriptors. A per-platform file inventory comes from §4.2 and the path descriptors.
- Nine areas: Start, Choose an architecture, Configure, Catalog lifecycle, Operate Starmap, Operate Starport, Storage, API compatibility, Troubleshoot.
- The embedded build holds prose, diagrams, fonts, styles, and a search index. It makes no external requests.
- Dynamic deployment information stays behind authorization. Installation and recovery docs need an offline export path for when the gateway cannot start. Public content holds no deployment secrets.
- Published references link to the applicable version, not an unpinned `main`.
- Hosting: proposed GitHub Pages. Stable `/<release>/` paths. Each deployment keeps prior release content. The maintainer configures a protected Pages environment before CSP23. Keep the exact static artifact as a release asset for offline export and rollback. A failed site release keeps the last version and a retryable record.
- Each build shows the Starport release, the Starmap module version, and the content revision. A visible version selector exists.
- Topics and headings have stable URLs. Reload, back, forward, and copied links keep state. The TOC marks the current section.
- Search indexes headings, prose, and setting names. "A link to the docs landing page alone does not provide documentation search."
- Code examples have copy controls and language labels.
- Typography (§10.3): body 16px/26px; measure max 68ch, cap 720px; title 28-32px; headings 20px with anchors; code 14px/21px; touch targets 44px; no horizontal scroll at 320px; WCAG 2.2 AA 4.5:1 in both themes; 200% zoom and text spacing; keyboard, skip links, screen reader.
- Case definitions: A29 (`SPEC:3755`) "Embedded docs load and search without internet, credentials, or catalog readiness. Public and embedded content revisions agree." A30 (`SPEC:3756`) "Storage screens match effective path reports. Topic links, narrow layouts, zoom, keyboard, text spacing, and contrast pass both themes."

## Subcases

`MAP` `task_checks.CSP18` lists exactly 10 subcases. A29 also requires `A29.public_url_content_manifest` and `A29.hosting_rollback`. CSP23 owns those two, and they are not in CSP18's gate. A29 is in `requires_published_assets`. `VER:903-909` requires qualification for any `--case A29` run. CSP18 is not in the list of tasks that need qualification. CSP18 is `primary_task` for A30. CSP8 owns A24.

The registry resolves each subcase through `VER:96-102`. A null entry returns UNVERIFIED with "No behavior check is registered." (`VER:276-278`).

| id | Verifier check (file:line) | Current Starport state (file:line) | Gap |
| --- | --- | --- | --- |
| A30.stable_links_history | None. `REG` entry is null, so `VER:276-278` gives UNVERIFIED. MAP contract text is null. | h2 anchors `doc-<slug>` with `#id` links (`SP/console/src/routes/docs.tsx:104-115`). `audience` search param (`docs.tsx:8-15`). Test "documentation audience and section survive a copied URL and history" (`SP/console/src/routes/docs.test.tsx`, 9 tests at :38-132). Only h2 has anchors. No TOC. No current-section marker. No topic-per-URL routing. | Topic routes, anchors on all headings, TOC with current section, version in the path. Register a vitest check. |
| A30.desktop_both_themes | None (null). | Docs theme toggle and token themes from CSP0.1 (`csp0.1.md`). E01 proof covers this only for old inputs, and it is stale. | New content renders in both themes. Needs a browser proof (reviewed_ui or automated). Register. |
| A30.narrow_320 | None (null). | CSP0.1 checked 320px reflow for the old page. `SP/console/src/styles/docs.css` (55 lines) sets `min(68ch, 720px)`. | Re-prove on the new tree (tables, code, TOC, version selector). Register. |
| A30.zoom_text_spacing | None (null). | Not proven. `csp0.1.md` limits exclude 200% zoom. CSP17 verification lists zoom as unchecked. | New proof at 200% zoom and WCAG text-spacing overrides. Register. |
| A30.keyboard_screen_reader | None (null). | `focus-visible` 2px outline (`docs.css`). Headings use `tabIndex -1` (`docs.tsx:104-115`). No skip link found in docs shell (`SP/console/src/routes/__root.tsx:35-37`). Not proven with a screen reader. | Skip link, landmark structure, search keyboard flow, proof. Register. |
| A30.instruction_contrast | `REG`: `all` of vitest `src/styles/contrast.test.tsx`, 3 tests: "instruction contrast settings instructions meet WCAG AA in both themes", "instruction contrast catalog panel instructions meet WCAG AA in both themes", "instruction contrast settings prose is at least 16 px". Run by `VER:390-415`. | PASS at CSP17 acceptance (`csp17/acceptance-2026-10-03/verification.json`, 16/16). Measured in jsdom, not a browser (`csp17/starport408-merge-2026-10-03/verification.json`). | None for the gate. Keep passing. New docs styles must not break these tests. |
| A29.embedded_offline_search | None (null). | No docs content search. `SP/console/src/components/palette/paletteIndex.ts` indexes 6 kinds (page, action, model, provider, author, key). `PaletteDialog.tsx:68` has only a `/docs` page entry. `SP/console/src/lib/search.ts` holds search-param readers only. | Build a local index of headings, prose, and setting names. Embed it. Prove no network request. Register. |
| A29.recovery_without_auth | None (null). | `/docs` skips the credential check on the client (`__root.tsx:21-27`) and is mounted outside auth groups (`SP/internal/server/routes.go:388-391`). Test "static recovery docs load without a session or deployment requests" exists in `docs.test.tsx`. But all docs need a running gateway. No offline export, no static recovery entry, no `starport docs` command. | Static recovery entry that works when the gateway cannot start (CLI export, release asset, or archive file). Register. |
| A29.no_dynamic_data_disclosure | None (null). | Docs content uses `location.origin` but no deployment data. Test "a public docs reader still needs authorization for deployment pages" exists. `GET /config/schema` is dynamic and admin-only (`routes.go:357`). | Prove generated pages hold no deployment values and the public tree makes no authenticated request. Register. |
| A24.generated_file_inventory | `REG`: `all` of go_test in `./internal/config`: `TestFileManifestIncludesWorkspaceRecoveryArtifacts` (`SP/internal/config/file_manifest_test.go:26`), `TestFileManifestSourceCachesFollowCanonicalSelection` (:147), `TestFileManifestIncludesSetupArtifactsAtSelectedLeaves` (:179); `./internal/cli` `TestConfigFileManifestReportsSelectedStorageWithoutOpeningIt` (`SP/internal/cli/file_manifest_test.go:15`). MAP contract: "Generated reference files agree with the runtime path and storage descriptor inventory." | PASS at CSP17 acceptance. The tests prove the runtime manifest (`SP/internal/config/file_manifest.go:66`, `starport config paths --files`, `SP/internal/cli/inspection_commands.go:36-80`). No generated reference doc exists. | Gate passes now, but the contract names generated reference files. CSP18 should add a generated inventory page and a test that compares it with `FileManifest`. Consider adding that test to the registry entry. |

Count: 2 PASS, 8 UNVERIFIED (predicted). All 8 need Starport behavior and a Starmap registry PR, as CSP17 did with Starmap #215.

## Current implementation

Console docs:

- `SP/console/src/routes/docs.tsx` (530 lines) holds hand-written TSX docs. Route `/docs` with search param `audience` = `build | account | operate` (:8-15). Three personas (:20-38). Tabs (:63-83). `CodeBlock` with copy button and language label (:90-102). `DocSection` with h2 anchors (:104-115). Scope table, 12 scopes (:129-144). `BuildDocs` :177-316, `AccountDocs` :320-375, `OperateDocs` :379-530. "Catalog and routing" is brief (:459-468). No version display, TOC, search, or content revision.
- `SP/console/src/routes/__root.tsx`: `beforeLoad` skips auth for `AUTH_PATH` and `/docs` (:21-27). `/docs` renders in a documentation shell without the app `Shell` (:35-37). Other pages redirect to auth (:43-44).
- `SP/console/src/styles/docs.css` (55 lines): measure `min(68ch, 720px)`, body 1rem / 1.625, h1 1.875rem (30px), h2 1.375rem (22px, spec says 20px), code 0.875rem, buttons min 2.75rem, `focus-visible` 2px outline.
- Docs entry points: `Shell.tsx:101` nav "Docs"; `FirstContact.tsx:64` links `/docs?audience=operate`.
- Unpinned link: `SP/console/src/components/members/IdentityRequired.tsx:6` links to `github.com/agentstation/starport/blob/main/docs/OPERATOR-GUIDE.md#identity`. The spec forbids unpinned `main` references.
- Markdown renderer already present: `streamdown` plus `@streamdown/code`, `math`, `mermaid` in `SP/console/package.json`. Used in `components/chat/Messages.tsx` and `components/overview/QuickstartCard.tsx`. `cmdk` drives the palette. No Markdown docs pipeline. No search library.
- Build: `console/vite.config.ts` outputs to `../internal/console/dist`. `check` = lint, build, typecheck, test.

Embedding and serving:

- `SP/internal/console/spa.go` (138 lines): `go:embed all:dist` (:28). CSP header is same-origin (:33-36). Index `no-cache` (:76-94). Assets immutable (:98-114). `spaPagePaths` includes `/docs` (:123-128). `Register` (:133-138). `TestSPAPagePathsCoverClientRoutes` ties paths to client routes.
- `SP/internal/server/routes.go:388-391` mounts the console outside the auth groups, after the session and identity routes (:369-386).
- `SP/internal/app/app.go:899-908` opens the console when `Console.Enabled`.

Settings and file inventory sources:

- `SP/internal/config/configuration_schema.go:23-53`: `ConfigurationSchema()` projects only Starmap `catalogconfig.Descriptors()` plus `config_management`.
- Starport's own settings use env struct tags (`SP/internal/config/config.go:22-38`, prefixes `SERVER_`, `STORAGE_`, `CATALOG_`, and others). About 106 distinct `STARPORT_` literals appear in `internal/config/*.go`. Exact settings count: UNVERIFIED.
- `SP/internal/config/file_manifest.go:66`: `func (c *Config) FileManifest(version string) (productpaths.FileManifest, error)`. Exposed by `starport config paths --files | --inspect | --legacy`.

Repository Markdown:

- `SP/docs/`: OPERATOR-GUIDE.md 2558 lines, ARCHITECTURE.md 782, README.md 456, DEPLOYMENT-TOPOLOGIES.md 325, DEVELOPMENT.md 306, PERFORMANCE.md 293, RECOVERY.md 274, ARCHITECTURE_CONTROL_PLANE.md 237, SECURITY-POSTURE.md 213, others. 6608 lines total. None is in the console.
- `SP/docs/TASKS.md:10-16`: the canonical plan owns documentation.

Tooling and release:

- `SP/scripts/verify-doc-links.sh` checks 15 Markdown files through `scripts/doclinks/main.go`.
- `Makefile` has no docs target.
- Workflows: `ci.yml`, `native-release.yaml`, `release.yaml` (tag-triggered goreleaser, immutable releases), `update-copyright-year.yml`. None mentions Pages or docs.
- `.goreleaser.yaml` has `files:` at :56 and `release:` at :158. A docs archive could attach here.
- GitHub (read-only `gh`): Pages 404, `has_pages` false, repo public, no environments.

Gates that touch docs:

- `verify-console-polish.sh:282`, CPL-V44: requires `/health/live` and `/health/ready` in `console/src/routes/docs.tsx`. CPL-V45 requires `docs.test.tsx`. Terminal at 48 conditions.
- `verify-package-layout.sh` scans all `.md`, `.go`, `.sh`, `.yml` for forbidden path strings (for example `internal/httpapi`).
- `verify-starmap-ownership.sh:96` scans `docs/ARCHITECTURE.md`.
- `verify-v1-release.sh:38` needs a phrase in `ARCHITECTURE.md`.
- `verify-developer-experience.sh:124` forbids "Coming Soon" in `docs/README.md`.

Prior proof:

- E01 (early check, CSP0.1) in `REG`: vitest `src/routes/docs.test.tsx` (9 tests) plus reviewed_ui proof `csp0.1/browser-review.json` with 14 required inputs and 10 observations (desktop, reflow_320, keyboard_focus, keyboard_tabs, keyboard_code_scroll, direct_fragment_reload, light_dark_contrast, static_docs_without_session, protected_routes_remain_guarded, current_commands_reviewed). `VER:418-445` requires every input sha256 to match.
- Measured now with read-only Python: 89 of 220 E01 input hashes mismatch at `c99f0f09`. 9 of the 14 required inputs mismatch: `docs.tsx`, `docs.css`, `tokens.css`, `FirstContact.tsx`, `Deployment.tsx`, `Section.tsx`, `api.ts`, `console/package.json`, `pnpm-lock.yaml`. E01 is stale before CSP18 starts.

Verifier mechanics relevant to registration:

- `run_vitest` (`VER:390-415`) runs `pnpm --dir console test <files> --reporter=json`. Files must start with `src/` and end with `.test.ts` or `.test.tsx`. PASS only if every named `fullName` passed.
- `reviewed_artifacts` (`VER:418-445`) needs `schema_version` 1, verdict PASS, all required observations true, `required_inputs` a subset of inputs, and matching sha256 for every input and capture. It does not repeat the capture.
- `aggregate` (`VER:858-875`). Exit 0 only on gate PASS (`VER:923`).
- Registry kind counts: go_test 1078, all 237, native_ci 21, vitest 4, reviewed_ui 1, others 1 each.

Exemplars (local, not cloned):

- `~/src/github.com/tauri-apps/tauri-docs`: Astro Starlight 0.42, Pagefind offline search, `starlight-links-validator`, `starlight-llms-txt`.
- `~/src/github.com/FerretDB/FerretDB`: Go repo with `website/` Docusaurus versioned docs (`versions.json`, `versioned_docs`, config :88-95) and `.github/workflows/docs.yml` (`deploy-pages@v4`, `github-pages` environment, concurrency).
- `~/src/github.com/apple/container/.github/workflows/docs-release.yml`: manual `workflow_dispatch`, branch validation, `github-pages` environment, `deploy-pages@v5`, permissions `pages: write` and `id-token: write`.
- Also: `addmax-ai/docs` (Docusaurus), `plabayo/rama` `mdbook.yml`, `wardgate` `deploy-docs.yml`, `agentstation/modelwiki` `hugo.yaml`.
- Note: `deploy-pages` replaces the whole site. Keeping prior `/<release>/` paths means the workflow must reassemble earlier versions, for example from release assets.

## Fail-before evidence expected

Record these on Starport `c99f0f09` (or the CSP18 base) before any change.

1. Verifier gate. From the Starmap verifier tree:

   ```bash
   bash scripts/verify-catalog-product.sh --starport-root "$CSP_STARPORT_WORKTREE" --task CSP18 --json
   ```

   Expected (UNVERIFIED, not run here): 2 PASS (`A30.instruction_contrast`, `A24.generated_file_inventory`), 8 UNVERIFIED with "No behavior check is registered.", gate FAIL, exit 1. Basis: CSP17 acceptance used the same verifier head, registry sha `641e04e5…`, and Starport head.

2. Console baseline:

   ```bash
   pnpm --dir console run check
   ```

   Expected: PASS, 79 files, 494 tests (CSP17 merge record). Live count UNVERIFIED.

3. No content search:

   ```bash
   grep -n "kind" console/src/components/palette/paletteIndex.ts
   grep -n "/docs" console/src/components/palette/PaletteDialog.tsx
   ```

   Expected: six kinds, no docs article or heading kind. One `/docs` page entry at `PaletteDialog.tsx:68`.

4. Docs need a running gateway:

   ```bash
   grep -n "embed\|/docs" internal/console/spa.go
   ./starport docs --help   # expected: unknown command
   ```

   Expected: docs exist only inside the embedded SPA. No CLI or release-asset docs export.

5. Only h2 anchors, no TOC, no version:

   ```bash
   grep -n "id=\|<h3\|toc\|version" console/src/routes/docs.tsx
   ```

6. Unpinned link:

   ```bash
   grep -rn "blob/main" console/src
   ```

   Expected: `IdentityRequired.tsx:6`.

7. No hosting:

   ```bash
   gh api repos/agentstation/starport/pages        # expected 404
   gh api repos/agentstation/starport/environments # expected total_count 0
   ```

8. Stale browser proof: hash the 14 E01 required inputs against `csp0.1/browser-review.json`. Expected: 9 mismatches.

Correct the plan's fail-before text. "Auth-gated docs" and "absent heading anchors" are no longer true as stated. Record instead: docs need a running gateway; anchors exist on h2 only; no TOC; no content search; no version or content manifest.

## Owner decisions needed

1. **Hosting and deployment workflow.** Options:
   - (a) CSP18 adds a Pages workflow that builds the artifact but does not deploy (manual `workflow_dispatch`, `github-pages` environment, required reviewers). The owner enables Pages and the protected environment before CSP23. Consequence: no public URL until CSP23. A29 stays incomplete, as planned.
   - (b) Enable Pages now. Consequence: an outward-facing publication that the task does not authorize. Content becomes public and cached before review.
   - Recommendation: (a). The repo has no Pages site and no environments.
2. **Version path scheme and retention.** Options:
   - (a) `/<release>/` per tag (`/v1.3.0/`) plus a `/latest/` alias. Consequence: many paths. Each deploy must rebuild all prior versions from release assets, because `deploy-pages` replaces the whole site.
   - (b) `/vX.Y/` per minor. Consequence: fewer paths, but patch-level content revisions overwrite each other, which weakens the A29 content-manifest comparison.
   - Rollback either way: redeploy a previous release's docs asset. A failed deploy keeps the live site because Pages deploys atomically.
3. **Docs build tool.** Options:
   - (a) Reuse Vite and React. Markdown content tree, build-time compile, local index (MiniSearch or Pagefind). Consequence: one stack, one theme, the existing tokens and contrast tests apply. More custom work for versioning, TOC, and the static export.
   - (b) Starlight or Docusaurus as a separate site that the console embeds as static files. Consequence: versioning, TOC, and search come built in. A second toolchain, a second theme that must match DESIGN.md and both contrast themes, larger embed, and a separate CSP review. CPL and shadcn ownership rules do not cover it.
   - Recommendation: (a) for the embedded console. It keeps one set of tokens and the existing jsdom tests. UNVERIFIED that Pagefind runs under the same-origin CSP without changes.
4. **Content tree location and the persona docs.** Options: `SP/docs/site/` (Markdown next to the repository docs) or `SP/console/src/docs/`. Moving or replacing `docs.tsx` breaks CPL-V44 (`verify-console-polish.sh:282`), which requires the health paths in that file. Consequence: either keep a thin `docs.tsx` that contains those strings, or change a terminal gate, which needs its own justification. Also decide whether `OPERATOR-GUIDE.md` (2558 lines) moves into the tree or stays as a repository file.
5. **Public versus authenticated boundary.** Static docs are public. Generated settings and file inventory pages must come from descriptors only, with no deployment values. `GET /config/schema` and effective config stay admin-only (`routes.go:357`). Decide whether the embedded docs may call any endpoint at all. Recommendation: none, so A29 holds without a session.
6. **Static recovery entry and offline export.** Options:
   - (a) A `starport docs export <dir>` command that writes the embedded tree. Consequence: works only if the binary runs, which is usually true when the gateway fails at config or store startup.
   - (b) A docs archive as a release asset via `.goreleaser.yaml`. Consequence: works with no binary. Also serves Pages rollback. Adds a release artifact that `verify-release-archives.sh` may need to know.
   - (c) Ship the static HTML inside the release archive. Consequence: larger archive. Changes archive layout gates.
   - Recommendation: (b) plus (a). Both use the same build output.
7. **A30 browser proof method.** Options:
   - (a) `reviewed_ui` manual proof, as E01. Consequence: cheap to add, but hash-bound to inputs. It goes stale on every docs edit, as E01 did (9 of 14 inputs drifted).
   - (b) Automated Playwright checks registered as a new check kind or through vitest browser mode. Consequence: durable, but the verifier has no Playwright kind today. Needs a Starmap verifier change and a browser in CI.
   - Also decide: renew E01 or retire it in favor of the new A30 checks.
8. **Settings inventory scope.** Options:
   - (a) Starmap descriptors only (what `ConfigurationSchema()` projects now). Consequence: Starport's own `STARPORT_*` settings stay undocumented in generated pages.
   - (b) All settings. Consequence: needs a Starport descriptor source for env-tag settings in `internal/config`. About 106 literals, count UNVERIFIED. Larger change, but matches the spec's "Starport descriptors".

## Risks and second-order effects

- **Terminal gates.** CPL-V44 and CPL-V45 bind to `docs.tsx` and `docs.test.tsx`. `verify-console-polish.sh` is terminal at 48 conditions. Moving content without a plan breaks it.
- **Path scans.** `verify-package-layout.sh` scans every new `.md` file for forbidden path strings. `verify-starmap-ownership.sh:96` scans `ARCHITECTURE.md`. Generated docs that quote internal paths may trip these gates.
- **Link checks.** `verify-doc-links.sh` covers 15 named files. A new tree needs coverage, or broken links ship. The unpinned link at `IdentityRequired.tsx:6` must move to a versioned path.
- **CSP and network.** `spa.go:33-36` is same-origin. The embedded build must not load external fonts, scripts, or search assets. Pagefind loads WASM and fragment files at runtime. UNVERIFIED whether it needs `wasm-unsafe-eval`.
- **Routing.** New docs routes must join `spaPagePaths` (`spa.go:123-128`), or `TestSPAPagePathsCoverClientRoutes` fails and deep links 404 on reload.
- **Embed size.** Prose, a search index, and generated pages grow `internal/console/dist` and the binary. No size gate was found. Size impact UNVERIFIED.
- **Ownership boundaries.** Generators that read Starmap descriptors must import through `internal/catalog` or `internal/config`. Composition stays in `internal/app` and wiring in `internal/server` (`SP/CLAUDE.md:34-36, 70`). A generator under `cmd/` or `scripts/` may break `verify-dependency-direction.sh`.
- **Contrast tests.** `contrast.test.tsx` measures settings and catalog instructions. New docs styles that change tokens can break A30.instruction_contrast, which passes now. CSP17 decision 17-4 already moved light `text-3` to `#66666e`.
- **Spec drift in styles.** `docs.css` h2 is 22px. The spec says 20px headings. Changing it moves the E01 inputs again.
- **Registry copies.** The verifier's acceptance-map copy (sha `c6acdcc0…`) and the plan-repo copy (sha `599464e5…`) differ in A14, CSP16.2, A26, and A27. CSP18, A29, and A30 entries are identical. The Starmap registry PR must base on current Starmap main.
- **Test counts.** Console check counts (79 files, 494 tests) change. Proof records that cite counts must update.
- **Downstream tasks.** CSP19 and CSP20 build on this tree. A tool choice that blocks generated pages or recipe validation blocks them. CSP23 needs the artifact and the content manifest that CSP18 defines.
- **Required content.** The tree must cover the discovery audit topics (permitted discovery, structural support, caller readiness, cache freshness, the released dependency for canonical reconciliation) and unrecognized-journal guidance. CSP17 decision 17-7 reports discovery readiness as a fixed unknown. The docs must say that, not promise readiness data.
- **Release workflow.** Adding a docs asset to `.goreleaser.yaml` changes release outputs. `verify-release-workflow.sh` and the Release Snapshot job gates may need updates. Releases are immutable, so a wrong asset cannot be replaced after the tag.
- **Untracked files.** `SP/docs/proof/catalog-native/` has untracked files from other work. Implementers must not touch them.

## Suggested delivery slices

Dependency order. Each Starport slice pairs with a Starmap registry PR (CSP17 pattern, Starmap #215), or one registry PR lands at the end.

1. **Content tree, build, and offline search (Starport).** One Markdown content tree with the nine spec areas. Build-time compile into the console. Topic routes, anchors on all headings, TOC with current section, version and content-revision display, local search index of headings, prose, and setting names, palette integration. Keep CPL-V44 strings. Add new routes to `spaPagePaths`. Turns to PASS: `A29.embedded_offline_search`, `A30.stable_links_history`. Keeps `A30.instruction_contrast` passing.
2. **Generated references, recovery entry, and the boundary (Starport).** Generated settings pages from descriptors (scope per decision 8). Generated per-platform file inventory with a test that compares it with `FileManifest`. Static recovery entry and offline export (decision 6). Tests that the public tree makes no authenticated request and holds no deployment values. Discovery and unrecognized-journal content. Replace the unpinned link. Turns to PASS: `A29.recovery_without_auth`, `A29.no_dynamic_data_disclosure`. Strengthens `A24.generated_file_inventory` (already PASS).
3. **Accessibility and browser proof (Starport, plus Starmap if Playwright).** Skip link, landmarks, 20px headings, 320px reflow of tables and code, 200% zoom and text spacing, both themes. Browser proof per decision 7. Renew or retire E01. Turns to PASS: `A30.desktop_both_themes`, `A30.narrow_320`, `A30.zoom_text_spacing`, `A30.keyboard_screen_reader`.
4. **Pages artifact and prepared workflow (Starport), and registry (Starmap).** Static site artifact with content manifest. Versioned paths and retention from release assets. Manual, protected, not-yet-enabled deploy workflow. Rollback by redeploy of a prior asset. Starmap registry PR registers all 8 subcases (if not done per slice). Turns `--task CSP18` to PASS (10/10). Prepares `A29.public_url_content_manifest` and `A29.hosting_rollback` for CSP23. Does not publish.

## Not checked and uncertain

- I did not run the verifier, vitest, `go test`, or any gate. They write caches in the repositories. All gate results above are predictions from code and the CSP17 records.
- Exact count of Starport settings: UNVERIFIED (about 106 `STARPORT_` literals).
- Real-browser behavior (keyboard, zoom, screen reader, 320px) of the current docs: UNVERIFIED.
- Pagefind or MiniSearch behavior under the current CSP: UNVERIFIED.
- Binary size impact of embedding the docs: UNVERIFIED.
- I did not read `verify-release-archives.sh` or `verify-release-workflow.sh` in full, so their reaction to a new docs asset is UNVERIFIED.
- I did not clone exemplars. I read only local copies.
