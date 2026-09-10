# Catalog lifecycle repository findings

The current repositories implement most catalog distribution infrastructure.
They do not yet satisfy every requested bootstrap, directory, credential, and
authority behavior. The [PRD](PRD.md) defines the product requirements.
The [engineering specification](ENGINEERING_SPEC.md) defines the proposed changes.

This report records source inspection and selected local tests on 2026-09-04.
It does not certify production availability or full API compatibility.
Dated entries retain their original qualification limits. The canonical plan owns current task status.

## Inspected revisions

| Repository | Reference used | Detail |
| --- | --- | --- |
| Starmap | `4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225` | GitHub default branch at inspection |
| Starport | `042eb97851496ecdd1ef20bfa44fe3d84b86b2d8` | GitHub default branch at inspection |
| Starport's Starmap dependency | `v0.16.5` | Published module pin in Starport's `go.mod` |

The original local Starmap checkout was at `fa7c26bb`. The original local
Starport checkout was at `117ad8f`. Both preceded the connected catalog work.
The findings below use isolated worktrees at the newer GitHub revisions.
The inspection did not modify either original checkout.

## Local implementation update: 2026-09-05

CSP1 completed the local Starmap settings contract after the inspected revisions below.
The public package owns 22 catalog settings and the generated [reference](../../CATALOG_SETTINGS.md).
The command now resolves explicit flags, environment, named dotenv files, and canonical YAML.
Source replacement clears old transport credentials and preserves independent node policy.

The [CSP1 proof](../../plans/proof/starport-production-catalog/csp1.md) records six passing task subcases and 217 race test events.
The affected packages also passed on Go 1.25.12.
These changes remain uncommitted. They do not establish Starport adoption or released-pair qualification.
The findings below retain their original revision scope.

## Verified current behavior

| ID | Finding | Evidence |
| --- | --- | --- |
| F01 | Starmap owns the embedded catalog, validates its manifest, and exposes it through the root Go client. | [Bootstrap loader](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/bootstrap/bootstrap.go), [root client](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/client.go) |
| F02 | The root constructor defaults to no workspace and no catalog store. Fresh passive CLI construction creates neither catalog directory. | [Options](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/options.go), [path tests](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/cli/app/catalog_paths_test.go) |
| F03 | The connected runtime starts with embedded or retained state and supports public, GitHub, internal Starmap, file, and embedded sources. | [Runtime](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/runtime/runtime.go), [source policy](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/runtime/policy.go) |
| F04 | The publication workflow runs every four hours at minute 17. It publishes immutable artifacts and an attested `catalog/v1` branch. | [Workflow](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/.github/workflows/catalog-generation.yaml) |
| F05 | The workflow modifies embedded data in its runner but pushes only the channel branch. It does not promote embedded inputs to the default branch. | [Generator](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/scripts/generate-embedded-catalog.sh), [publication step](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/.github/workflows/catalog-generation.yaml#L275) |
| F06 | The runtime replaces its baseline with the complete source payload. Provider layers use `MergeEnrichEmpty` and preserve prior layers after failures. | [Layer builder](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/runtime/layers.go), [refresh](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/runtime/refresh.go) |
| F07 | Starport composes Starmap's connected runtime and separates candidate and accepted heads in its KV catalog adapter. | [Runtime composition](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/catalog/runtime.go), [catalog adapter](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/catalog/generation_store.go) |
| F08 | Starmap acquisition resolves conventional environment names before `STARMAP` names. Starport inference resolves conventional names before `STARPORT` names. | [Starmap resolver](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/auth/resolver.go), [Starport resolver](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/credentials/resolver.go) |
| F09 | Starport acquisition uses `STARPORT` names before conventional names. It reads only the deployment lookup and has no `STARMAP` fallback. | [Acquisition resolver](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/catalog/acquisition_resolver.go) |
| F10 | Starmap acquisition and Starport inference have direct secret-manager adapters. Starport's connected acquisition resolver only reads environment values. | [Starmap secret sources](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/auth/secret_source.go), [Starport secret sources](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/credentials/secret_source.go), [acquisition composition](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/catalog/runtime.go) |
| F11 | Starport has Badger and Valkey adapters. SQLite, PostgreSQL, and MySQL use a separate relational contract. | [KV open](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/storage/open.go), [SQL open](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/sqlstore/open.go) |
| F12 | SQL migrations cover users, teams, memberships, grants, account templates, incident transitions, and audit records. | [SQLite migrations](https://github.com/agentstation/starport/tree/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/sqlstore/migrations/sqlite) |
| F13 | Starport has a catalog refresh lease, candidate acceptance checks, and compare-and-swap storage. | [Lease](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/catalog/lease.go), [acceptance](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/catalog/acceptance.go) |
| F14 | Starport exposes both API families and has a 17-condition OpenRouter parity guard. That guard largely checks source structure. | [Routes](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/routes.go), [parity guard](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/scripts/verify-openrouter-parity.sh) |

The supported acquisition source IDs include provider APIs, models.dev Git and
HTTP, local catalog, release artifact, and embedded catalog. The scheduled
generator refreshes models.dev before its catalog update. The connected
acquirer collects provider observations. These are distinct compositions.
[Source IDs](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/pkg/sources/source.go)
[Connected acquirer](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/acquisition/acquirer.go)

## Current directories

| Product and purpose | Current default | Override or limit |
| --- | --- | --- |
| Starmap configuration | `~/.starmap/config.yaml` | Explicit configuration file |
| Starmap human catalog workspace | `~/.starmap/catalog` | `STARMAP_CATALOG_WORKSPACE_PATH`, then existing catalog-path configuration |
| Starmap CLI catalog store | `~/.starmap/state/catalog` | Library callers can inject a store. The CLI helper selects this fixed default. |
| Starmap connected runtime | `~/.starmap/state/runtime` | `STARMAP_STATE_DIR` |
| Starport configuration | `os.UserConfigDir()/starport/config.env` | Absolute `STARPORT_CONFIG_DIR` |
| Starport local databases | Configuration root plus `data/badger` and `data/sqlite/starport.db` | Storage-specific settings |
| Starport connected catalog state | `$XDG_STATE_HOME/starport/catalog`, otherwise `~/.local/state/starport/catalog` | `STARPORT_CATALOG_STATE_DIR` |

Starmap's configuration and directory constants remain home-based across the
three operating systems. `STARMAP_STATE_DIR` relocates runtime state. It does
not relocate the CLI's separate catalog store.
[Starmap constants](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/constants/constants.go)
[CLI path selection](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/cli/app/catalog_paths.go)

Starport's configuration root follows the operating system. Go returns XDG
configuration on Linux, Application Support on macOS, and AppData on Windows.
[Go configuration paths](https://pkg.go.dev/os#UserConfigDir)
Starport's catalog state resolver separately uses the XDG pattern on every
platform. Its Linux-style fallback also applies on macOS and Windows.
[Starport paths](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/paths.go)
[Catalog state resolver](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/catalog.go)

The conclusion is partial platform support. Both products expose configuration
and storage controls. Neither currently has the complete, consistent path
contract in the proposed specification. Native Windows and Linux execution
was outside this inspection.

## Publication evidence

The scheduled run created at `2026-09-04T22:20:36Z` completed successfully at
`2026-09-04T22:24:57Z`.
[Observed workflow run](https://github.com/agentstation/starmap/actions/runs/33925098413)

The inspected channel reported sequence `3`, with a confirmation time of
`2026-09-04T22:24:34.746942242Z`. It selected generation
`d7eb2a18-d8a0-41f5-b1f1-3c5e7aeb1dea`.
[Channel document](https://github.com/agentstation/starmap/blob/catalog/v1/channel.json)

The inspected default branch still embedded generation
`catalog-20260830T083213Z-ad5d2a45e490`, generated on August 30. Its manifest
reported schema `6` and a payload size of `8,270,634` bytes.
[Embedded manifest](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/embedded/catalog/generation.json)

This difference confirms that public publication and embedded-source promotion
are separate today. The channel read verified reported metadata only. This
investigation did not independently download and verify the release attestation.

## Gaps and inconsistencies

| ID | Gap or concern | Required disposition |
| --- | --- | --- |
| G01 | Fresh passive startup intentionally leaves the catalog directories absent. Runtime initialization also retains an empty-layer baseline in memory. | Add explicit application baseline materialization while preserving the passive library contract. |
| G02 | Catalog publication does not promote the default branch's embedded input. | Add the checked promotion transaction from specification section 5. |
| G03 | Product roots and runtime roots follow different platform rules. A Starmap runtime override leaves the catalog store elsewhere. | Add one path report, explicit ownership, and a migration contract. |
| G04 | Provider enrichment fills empty fields rather than selecting newer authoritative values. | Review the field-authority rules before changing merge semantics. |
| G05 | `require_source` requires a fresh source read during open. It does not express retained internal-authority startup. | Add authority-aware first-boot and restart behavior. |
| G06 | Acquisition and inference use different environment precedence. Starport acquisition lacks Starmap-key fallback and direct secret references. | Implement the role-specific resolution contract and configuration diagnostics. |
| G07 | Starport can use Valkey while SQL remains local SQLite. | Validate fleet storage as a combined configuration. |
| G08 | Selecting another SQL backend does not migrate records between engines. | Supply migration and restore procedures with reference checks. |
| G09 | Starmap supports `prefer_local`, but Starport's configuration accepts only `prefer_source` and `require_source`. | Align the documented shared configuration surface or document an intentional restriction. |
| G10 | Starport documents acquisition interval zero as startup-only, but its option adapter forwards only positive intervals. | Prove zero behavior and preserve explicit zero during translation. |
| G11 | Deployment documentation describes restricted catalog egress as all gateway egress. | State inference egress separately. |
| G12 | The enterprise runbook describes uninterrupted rotation using one active server key. | Add key overlap or change the availability claim. |

G05 and G09 follow from the inspected runtime and configuration validators.
G10 follows from `Settings.starmapOptions` and its `AcquisitionInterval > 0`
condition. These are source findings, not new regression reproductions.
[Startup implementation](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/runtime/runtime.go#L193)
[Starport configuration](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/catalog.go)
[Option translation](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/catalog/settings.go)

G11 and G12 concern the meaning of the deployment instructions.
Disabling acquisition does not disable inference requests. A client that
switches to a new key before the server accepts it cannot authenticate.
[Deployment topologies](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/docs/DEPLOYMENT-TOPOLOGIES.md)
[Enterprise runbook](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/docs/ENTERPRISE_CATALOG_SERVER.md)

## Fleet concerns requiring implementation evidence

The following concerns need real backend and multi-process tests:

- Starport checks the lease epoch before a separate accepted-head commit.
  The required contract binds the lease and head in one atomic backend operation.
- Valkey's multi-key Lua compare-and-swap needs compatible cluster hash slots.
  The current catalog key names do not visibly define a common hash tag.
- Retained provider layers live in process-local files. Leader failover must
  preserve the evidence needed to build the next effective generation.
- SQL migration checks and applies files within one process. Concurrent
  migration ownership needs a separate test and contract.

These are review concerns, not reproduced production failures. Existing
single-process lease tests do not establish all distributed failure behavior.
[Acceptance](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/catalog/acceptance.go)
[Valkey compare-and-swap](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/storage/valkey.go)
[Layer persistence](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/runtime/layers.go)
[SQL migrations](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/sqlstore/migrate.go)

## Configuration and documentation findings

The second review used the same source revisions. It inspected Starmap server
configuration, Starport settings, deployment documentation, and the rendered console.
The user confirmed full embedded and public documentation, Valkey plus PostgreSQL
as the primary fleet recipe, and initial single-region production support.

| ID | Current behavior or gap | Evidence and required disposition |
| --- | --- | --- |
| G13 | Starmap's canonical catalog settings live in an internal package. Starport repeats fields and option translation. | Publish a shared versioned contract and test configuration parity. See S1 and S2. |
| G14 | Starport loads its product-prefixed values from environment and `config.env`. It does not implement the proposed Starmap catalog-setting fallback. | Add allowlisted inheritance with explicit-value and source-credential rules. See S2. |
| G15 | Starmap exposes source aliases and coalescing settings that Starport's catalog config omits. Starmap server environment values can override explicit host and port flags. | Define the superset and preserve explicit CLI precedence during migration. See S1 through S3. |
| G16 | A zero source poll interval does not suppress watcher events. `ACQUISITION_ENABLED=false` controls scheduling, while manual `Sync` can still acquire. | Enforce offline, authority, manual-refresh, and pin policies across all triggers. See S4. |
| G17 | Starport's deployment settings are read-only facts. Catalog actions offer refresh, cancel, and status copy. | Add effective configuration origins, management mode, validation, and applied-revision reporting. See S5 and S6. |
| G18 | The docs route sits behind the console credential guard. Static recovery instructions therefore depend on console access. | Separate static docs from authenticated deployment information. See S7. |
| G19 | Embedded operator docs are brief TSX content. Detailed recipes live in separate Markdown. The embedded text still mentions a Models freshness bar. | Build embedded and public docs from one versioned content tree. Update references to the catalog chip and panel. See S6 through S8. |
| G20 | Deployment guidance describes every fleet replica contacting GitHub and retaining a separate accepted head. Starport now has shared-head and lease composition. | Rewrite the fleet recipe around actual deployment ownership. Measure request budgets instead of assuming one request per poll. See S8 and F07. |
| G21 | The Starmap store contract mentions possible Starport SQL adapters. The current Starport catalog adapter uses KV. | Separate extension examples from shipped storage and supported topology claims. See S9 and F07. |
| G22 | The enterprise guide says a plain shared volume lacks leases and conditional writes. The filesystem adapter adds locked conditional head commits. | Clarify adapter guarantees, refresh leasing, and qualified multi-host operation. Retain the single-writer restriction. See S9 and S10. |

Source references:

- S1: [Starmap catalog settings](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/catalog/settings/settings.go).
- S2: [Starport loader](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/loader.go), [catalog config](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/catalog.go), [translation](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/catalog/settings.go).
- S3: [Starmap server flags and environment](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/cli/commands/serve/command.go#L195), [application configuration](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/cli/app/catalog_runtime.go#L143).
- S4: [Source and acquisition schedules](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/runtime/scheduler.go#L182), [manual acquisition](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/runtime/refresh.go#L255).
- S5: [Deployment settings](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/console/src/components/settings/Deployment.tsx), [settings route](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/console/src/routes/settings.tsx).
- S6: [Catalog panel and actions](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/console/src/components/shell/CatalogPanel.tsx).
- S7: [Console route guard](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/console/src/routes/__root.tsx), [embedded docs](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/console/src/routes/docs.tsx).
- S8: [Deployment topologies](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/docs/DEPLOYMENT-TOPOLOGIES.md), [operator guide](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/docs/OPERATOR-GUIDE.md).
- S9: [Catalog store contract](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/docs/CATALOG_STORE_CONTRACT.md), [Docker deployment guidance](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/docs/DOCKER.md).
- S10: [Filesystem conditional commit](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/pkg/catalogs/storage/filesystem.go#L87), [conditional S3 adapter](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/pkg/catalogs/storage/s3/backend.go).

G22 refers to the [enterprise store guidance](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/docs/ENTERPRISE_CATALOG_SERVER.md#L14).

Starmap's general configuration file loader and connected catalog loader use
different paths. The inspected catalog loader consumes flag overrides and
environment lookup, with a legacy remote-source fallback.
A complete catalog configuration-file mapping needs parity evidence.
The server exposes catalog reads, events, and optional update operations.
Those routes do not prove the proposed subscriber-versus-administrator permission contract.
[Server routes](https://github.com/agentstation/starmap/blob/4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225/internal/server/router.go)

## UX review

The review rendered the actual Starport console routes and CSS with Vite 8.2.2.
The inspection worktree used the pinned revision above. Its dependency lockfile
matched the installed local dependency tree's checkout.
A browser fixture supplied a dummy session marker and empty API responses.
No real gateway, provider, secret manager, or database supplied UI data.
Unknown catalog and gateway indicators in screenshots reflect that fixture.

The review used desktop Chromium at responsive viewport widths of 1440, 390,
and 320 CSS px. Narrow viewports were not native mobile devices.
The captures include development-tool overlays, which are not production UI findings.
Light-theme captures use the application's light palette.
[Raw measurements and capture hashes](evidence/measurements.json)

| ID | Observation | Assessment and target |
| --- | --- | --- |
| U01 | Desktop operator prose is 14 px with 22.75 px line height in a 768 px column. The first sampled line contains 123 characters. | Too dense for long procedures. Use the specification's 16 px prose and `68ch` reading column. |
| U02 | Section headings are 14 px. Code blocks are 12 px with 16 px line height. The page title is 24 px. | Strengthen article hierarchy and increase code readability. These are product targets, not WCAG font-size requirements. |
| U03 | All three audience panels fit a 320 px viewport without horizontal page overflow. Code blocks scroll within their container. | Preserve this behavior when adding tables, configuration names, and wider reference content. |
| U04 | Audience tabs wrap at narrow widths and measure 36 px high. ArrowRight moves focus and Enter activates the focused tab. | Keyboard selection works in the sampled path. Increase touch targets to the 44 px product target. |
| U05 | Audience selection stays at `/docs`. Operator section headings have no IDs. The route stores its persona in component state. | Add stable topic and section URLs, history behavior, and a table of contents. |
| U06 | The command palette indexes the Docs destination alongside application entities. It does not index the documentation article content. | Include an offline documentation index with headings, prose, and environment names. |
| U07 | Sampled body text contrast is 12.46:1 in dark mode and 10.44:1 in light mode. Geist fonts load from bundled files. | Preserve these strengths. These samples do not certify every state or color pair. |
| U08 | The light settings page renders `STARPORT_STORAGE_MODE` at 12 px in `#a1a1aa` on white, about 2.56:1 contrast. | This meaningful instruction fails the 4.5:1 normal-text target. Read-only information does not receive the disabled-control exception. |
| U09 | Embedded operator docs omit a complete topology chooser, storage recipe, GitHub-off procedure, and restore path. | Ship the full operator content and link each configuration screen to its procedure. |

The typography findings come from computed styles and rendered element bounds.
U05 and U06 combine DOM inspection with route and search source inspection.
U08 uses the actual theme colors and relative-luminance contrast calculation.
[Typography tokens](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/console/src/styles/tokens.css)
[Search content](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/console/src/components/palette/PaletteDialog.tsx)
[W3C contrast guidance](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html)

Captured states:

- [Operator docs, 1440 px, dark](evidence/starport-docs-desktop-dark.png).
- [Operator docs, 1440 px, light](evidence/starport-docs-desktop-light.png).
- [Operator docs, 390 px, dark](evidence/starport-docs-mobile-dark.png).
- [Developer docs, 320 px, dark](evidence/starport-docs-build-320.png).
- [Deployment settings, 1440 px, light](evidence/starport-settings-light.png).

The review did not complete native screen-reader testing, 200 percent text
enlargement, actual browser zoom, text-spacing overrides, or a full accessibility audit.
It did not test public-site rendering or full offline docs because those target
surfaces do not yet share the proposed content build.
Existing docs tests cover navigation links, health paths, scopes, and snippet key
references. They do not establish layout or full documentation coverage.
[Docs tests](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/console/src/routes/docs.test.tsx)

## Production qualification gaps

The design now defines ownership and supported architecture targets. Production
qualification remains incomplete. The following evidence groups must close before
the corresponding claims appear in release documentation.

| Claim | Required evidence | Requirement mapping |
| --- | --- | --- |
| Durable offline baseline and consistent product paths | Fresh application startup, native paths, and interrupted migration | P03, P04, A01 through A04 |
| Safe enterprise catalog authority | Cold and warm startup, scope changes, manual-operation guards, and server authorization | P10, P18, P25, A09, A10, A22, A32 |
| Cohesive configuration and credentials | Shared descriptor coverage, override parity, source binding, role isolation, and activation failure handling | P11, P12, P19 through P21, A11, A12, A24 through A28 |
| Supported Starport fleet | Real Valkey and PostgreSQL tests for fencing, failover, configuration drift, and recoverable layers | P14, P26, A13 through A15, A21, A28 |
| Recoverable storage and migration | Consistent KV, SQL, file, and secret restoration, including revocations and budgets | P15, P26, A16, A31, A33 |
| Complete product documentation | Shared content build, validated recipes, offline search, stable links, and accessible configuration screens | P22 through P24, A29 through A31 |
| OpenRouter replacement surface | Real HTTP and official SDK tests for the declared API matrix | P13, A17 |

Redis, MySQL, Valkey Cluster, custom Starmap object-store compositions, and
multi-region active-active operation must not inherit qualification from another backend.
Capacity, recovery-time, and recovery-point commitments need explicit workloads and results.
This report does not certify production support from an interface alone.

## README and demonstration baseline

The README still names the August 29 catalog and its 17 providers and 511 models.
It links to `docs/assets/2026-08-29_starport-console.gif`.
The caption describes catalog, provider, and health views. It does not describe
installation or a successful inference request.
[Inspected README](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/README.md)

`ffprobe` reports a 1440 by 777 px GIF with 11 frames, 17.1 seconds duration,
and 2,208,444 bytes. The asset has Git blob ID
`7dc4e2a976203344318a2fdba107fb6d43006f29` at both inspected local revisions.
Sampled frames show the model catalog and provider views. The model frame still
shows the old freshness bar. The replacement must follow the final product UI.
[Inspected GIF](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/docs/assets/2026-08-29_starport-console.gif)

The existing README verifier depends on specific headings and wording.
It checks temporary state, credential roles, and dynamic release selection.
The implementation must preserve those behavioral checks when the README structure changes.
Text matching alone does not prove that installation and first inference work.
[README verifier](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/scripts/verify-readme-quickstart.sh)

The new README and demo requirements are P27 and P28, with acceptance cases
A34 through A39. No replacement recording or product implementation ran during planning.

## Checks run

The earlier catalog review ran the selected tests on macOS with the race detector.
Both commands exited
with status `0`. Starmap passed 12 top-level tests. Starport passed 15 top-level
tests and 21 subtests. No selected test failed or skipped.

Starmap command:

```bash
go test -race -json -timeout 5m ./runtime ./internal/cli/app -run 'Test(OpenReturnsEmbeddedStateBeforeSourceReply|RuntimeReadsReachNoExternalSystem|RuntimeRefreshMethodsChangeDistinctLayers|BuildKeepsUnlinkedOfferingsOutOfTheEffectiveCatalog|DurableRuntimeKeepsTheDerivedEffectiveIdentity|UnchangedRebuildCommitsNoSecondGeneration|RestartReportsTheCommittedEffectiveIdentity|SourcelessDurableRestartKeepsOneDerivedIdentity|CatalogPathsFreshInstallAreCanonicalSeparatedAndPassive|CatalogPathFollowsTheCanonicalWorkspaceSetting|RuntimeLeaseRejectsStaleEpochAtCommit|RefusedLeaseOpensAsANonOwner)$'
```

Starport command:

```bash
env -u TEST_POSTGRES_URL -u TEST_MYSQL_DSN go test -race -json -timeout 5m ./internal/config ./internal/catalog ./internal/sqlstore -run 'Test(PlatformPathsUseUserConfigDirectory|PlatformPathsUseExplicitDirectory|PlatformPathsRejectRelativeExplicitDirectory|PathsForConfigDirDerivesManagedPaths|CanonicalCatalogSettingsLoad|CatalogConfigRefusesUnusableSettings|ResolveStateDirectoryIsProcessLocal|CatalogCredentialEnvironmentPrecedence|CatalogDerivedCredentialReferenceEnvironment|LeaseFencesOneHolderPerDeployment|SQLitePersistsAcrossOpens|SQLiteInMemory|MigrateRunsEachFileOnceInOrder|MigrateFailureRecordsNothing|DialectMigrationSetsAgree)$'
```

The investigation did not run provider acquisition, live secret-manager
requests, database migrations against external systems, or publication writes.
It did not run the full repository suite, official SDK smoke tests, native
Linux or Windows tests, or real Redis, Valkey, PostgreSQL, and MySQL tests.
Those remain required implementation evidence where the specification names them.

The documentation extension changed no product source. It did not repeat the
earlier Go tests. It added the rendered UX checks recorded above.
The final documentation checks passed:

- `technical-writing lint docs/design/catalog-lifecycle/ --format text`: three files, zero diagnostics.
- `make technical-writing-check`: 743 files, zero diagnostics, and no missing glossary terms.
- `git diff --check`: no whitespace errors.
- Local-link and source-path verification: 12 local links and 55 pinned source paths exist.
- Requirement-ID verification before the README extension: 26 unique product requirements and 33 unique acceptance cases.
- Evidence verification: five screenshot files and seven measurement sets, with recorded screenshot hashes.

The original product checkouts remain clean. The design changes and captured
evidence remain uncommitted in the isolated documentation worktree.

## Subsequent plan audit

The [full plan audit](../../plans/proof/starport-production-catalog/audit-2026-09-04/AUDIT.md)
rechecked the same main revisions and expanded the selected test coverage.
It records ten findings, raw test results, and required contract revisions.
The user confirmed D13 during that audit. Product code remains unchanged.


## Verified review update on 2026-09-05

The [Fable review](../../reviews/FABLE_CATALOG_PRODUCT_REVIEW_2026-09-04.md)
added eleven findings across developer, startup, and enterprise journeys.
The [assessment](../../plans/proof/starport-production-catalog/fable-review-2026-09-04/ASSESSMENT.md)
corrects its claims and distinguishes requirement changes from implementation evidence.
The [review resolution](../../plans/proof/starport-production-catalog/revision-2026-09-05/REVIEW_RESOLUTION.md)
routes every audit and Fable finding to the revised contracts and tasks.

The following findings use the same pinned source revisions.
They describe current behavior or missing design guarantees, not implemented fixes.

| ID | Verified behavior or design gap | Required disposition |
| --- | --- | --- |
| G23 | Starport appends the derived credential environment name after conventional names. The target reverses this order. | CSP9 adds resolver-policy migration and explicit conflict resolution before a changed payer can activate. |
| G24 | The reviewed compatibility requirements lacked client-facing rename, alias, removal, and deprecation transitions. | P29, specification section 2.1, CSP3, and CSP10 define the transition contract and required protocol evidence. |
| G25 | Starport's loader resolves files and environment before application store setup. No complete shared configuration authority contract exists in that flow. | D12 and proposed D15 split bootstrap from deployment settings. CSP16 and CSP16.1 own shared revisions, audit, and authorized edits. |
| G26 | The team-budget reader returns no rule both for absence and for a failed read. Usage read errors also permit traffic. | D14 and CSP12 distinguish unknown policy, unknown usage, confirmed absence, and exhausted budget. |

Source evidence:

- [Credential lookup order](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/credentials/resolver.go#L623).
- [Configuration loader](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/loader.go#L124).
- [Application construction](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/app/app.go#L132).
- [Team-budget lookup](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/budget.go#L116).
- [Usage read failure](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/budget.go#L164).

The user clarified local versus shared configuration authority and selected refusal
when Starport cannot determine whether required budgets permit a request.
Shared SQL for deployment configuration remains an engineering proposal.
The baseline payload, manual source refresh, and four-hour promotion requirements remain intact.

The revised plan has 30 tasks, 40 primary acceptance cases, and named required subcases.
Early first-use checks do not count as final production qualification.
Public docs, released installers, and final recording evidence follow publication.
Native paths, credential migration, and SDK checks move to the tasks that introduce their behavior.

No product tests or browser measurements ran again during this document revision.
Earlier test outputs, screenshot measurements, and attributed reviews remain historical evidence.
The revision proof records current document checks separately.


## Storage recommendations incorporated on 2026-09-05

The [storage review](STORAGE_REVIEW.md) inspected the same pinned revisions and records fourteen additional findings.
Six findings have priority P1 and eight have priority P2.
Its current file inventory, source references, and raw tests remain unchanged.
The [storage resolution](../../plans/proof/starport-production-catalog/storage-revision-2026-09-05/REVIEW_RESOLUTION.md)
records the contract changes and current task assignments.

The specification now defines primary config filenames, native roots, managed files, relative anchors, instance ownership, and complete path reports.
It separates durable KV records from bounded application caches and requires expiry-preserving refill.
Backend work covers Valkey URI and TLS handling, deployment namespaces, effective configuration, and explicit durability.
Temporary development must reject persistent selectors. Container qualification must preserve linked KV, SQL, and blob records through replacement.

The original storage report's task areas were recommendations at review time.
The current plan adds CSP12.1 for cache ownership, capacity, and expiry.
Starport owns storage and cache descriptors. Starmap continues to own the shared catalog settings contract.
The [acceptance map](../../plans/proof/starport-production-catalog/acceptance-map.json) routes SR01 through SR14 to named task and case rosters.

The plan now contains 31 proposed tasks and 43 primary cases with 241 required subcases.
A41 covers backend contracts, A42 covers caches, and A43 covers temporary storage.
No implementation finding is complete merely because the documents now specify its correction.

The storage review ran selected macOS tests with the race detector: 19 top-level tests and 46 subtests across four packages.
There were no selected failures or skips. External service cases did not run and remain UNVERIFIED.
See the [test summary](evidence/storage-review-2026-09-05/local-tests-summary.json) and [raw output](evidence/storage-review-2026-09-05/local-tests.jsonl).
This later document revision did not repeat product tests or qualify native platforms, external backends, or container recovery.
The [revision verification](../../plans/proof/starport-production-catalog/storage-revision-2026-09-05/verification.json) records document checks separately.

## Latency recommendations incorporated on 2026-09-05

The user accepted the [latency review](LATENCY_REVIEW.md) recommendations for the target design.
The review inspected the same Starport revision and its Starmap v0.16.5 dependency.
Its raw measurements and profiles remain historical evidence.
The [latency resolution](../../plans/proof/starport-production-catalog/latency-revision-2026-09-05/REVIEW_RESOLUTION.md) records the task and acceptance mapping.

| ID | Verified gap | Required disposition and owner |
| --- | --- | --- |
| LR01 | Exact-model planning rebuilds all candidates. Starmap offering reads copy complete provider model maps. | CSP3.1 completed locally in `70f01b6a`. CSP10.1 still must precompute candidates. A44 preserves aliases, ownership, authority, and generation leases. |
| LR02 | Account and shared credential decryption repeats Argon2 during requests. | CSP9.1 manages bounded, revision-bound material. A45 preserves encryption strength, grants, rotation, expiry, and revocation. |
| LR03 | Gateway key and account checks read KV records. Team budget policy can query SQL on each request. | CSP10.2 caches valid authorization bundles. CSP16 retains applied configuration in memory. A46 and A28 enforce validity and authority. |
| LR04 | Applicable meters require repeated storage work. Lossy usage capture cannot prove strict concurrent budgets. | CSP12.2 reserves capacity atomically and recovers uncertain attempts. CSP15 qualifies real failures through A15 and A47. |
| LR05 | Cache fills block responses. Stream caching retains events before its final size check. | CSP12.1 bounds cache reads, asynchronous fills, workers, and stream memory. A42 and A48 preserve expiry, isolation, and admission. |
| LR06 | Some advisory peer reads and publications execute from inference callbacks. | CSP10.3 moves exchange into bounded workers. A49 preserves local breaker behavior and authority separation. |
| LR07 | The overhead timer omits middleware, request decoding, final encoding, and connector work that its subtraction includes. | CSP0.4 establishes full-path evidence. CSP22 qualifies A50. UI, README, recipes, and shipped claims use those boundaries. |

The embedded OpenAI case selected one model from 111 active chat routes in 9.4–10.1 ms with 15.3 MB allocated per operation.
The 1,000-route synthetic case took 252–270 ms and allocated 569 MB per selection.
Stored credential decryption took 13.6–14.3 ms and allocated about 64 MiB per operation.
These are separate local benchmark averages on macOS arm64 with an Apple M2 Max.

The review and manifest disagree on the Go version. The latency resolution records this evidence limit.
They are not complete gateway latency, production percentiles, or retained process memory.

The current overhead test passed with an integer p50 and p99 of zero milliseconds.
Its fixture and timing exclusions prevent that result from qualifying whole-gateway overhead.
The existing full server benchmark uses mock storage and a connector delay, so its results also require those limits.
The review's [manifest](evidence/latency-review-2026-09-05/manifest.json) records exact commands, artifact references, hashes, and measurement limitations.

D19 through D21 now define the accepted target recommendation.
P34 through P38 and A44 through A50 require bounded valid memory state, atomic admission, optional-work limits, and complete measurements.
The canonical plan has 38 tasks, 50 primary cases, and 309 required subcases.
Implementation remains unstarted. No benchmark result counts as a completed implementation task.

## Active implementation: 2026-09-05

The user activated the whole-plan goal after the final audit.
The earlier counts and unstarted statements above describe their dated review inputs.
The active roster now has 38 tasks, 50 primary cases, and 324 required subcases.
The [activation resolution](../../plans/proof/starport-production-catalog/activation-2026-09-05/RESOLUTION.md) maps CA01 through CA07 to revised contracts and checks.
CSP0 establishes the red verifier before product changes. No target behavior has implementation acceptance credit yet.


### CSP2 local implementation checkpoint

The [CSP2 proof](../../plans/proof/starport-production-catalog/csp2.md) records baseline export, canonical root resolution, and exclusive runtime-directory locking.
These local changes supersede the earlier baseline observations only within this worktree.
Owner records and canonical seeds now bind persistent identity to the product, deployment, and instance.

The `runtime.PrepareDirectoryMigration` API now records source checksums and checks the recorded identity under an exclusive directory lock.
Legacy identity remains unverified when no owner record exists. Journal recovery preserves interrupted writes and refuses conflicting intent or changed inputs.

The `runtime.StageDirectoryMigration` API now copies files into private staging and verifies their bytes against the manifest.
Five process-exit cases verify recovery during copy and around journal updates. A pending marker blocks runtime initialization.

The `runtime.PublishDirectoryMigration` API now validates retained catalog layers, publishes the target, and retires the source.
Five further process-exit cases verify publication recovery. Retries preserve catalog updates from an active replacement.
Immutable receipts support later migrations and refuse conflicting records.

The replacement runtime can record completion after its active configuration selects the target, owner, and retained identity.
The canonical schema already exposes the retained identity through flags, environment variables, explicit dotenv files, and YAML.
The path report now includes that selection and its origin.

The migration CLI now exposes preparation, staging, publication, and completion through application-owned adapters.
Completion requires a matching saved configuration and verifies publication before runtime initialization.
The file digest binds the check to the bytes that the loader parsed. The command closes its temporary replacement runtime.

Startup initialized a replacement directory after publication preflight in the regression test.
The CLI now requires the published migration during startup, with verification under the directory lock before owner, seed, or catalog initialization.

Operators still save configuration changes and update service definitions through their normal deployment process.
Default-root startup now accepts a completed legacy move when its source inventory, retirement record, target receipts, owner, and identity verify.
A second recognized legacy root still causes refusal. The runtime repeats verification under its directory lock before persistent initialization.
Ordinary catalog reads do not hash the preserved source. Later target catalog updates and journal archival preserve the acknowledgement.

Two child-process tests cover exit after the final journal event and after the completion record.
A retry creates a missing completion record without duplicating journal events.

Legacy migration, full file-policy reporting, and native platform qualification remain incomplete.

Source defaults now use the native cache root. CLI update and server acquisition pass their configured cache and checkout parents.
Per-operation source overrides retain precedence. Logo projection now reads the selected checkout instead of an unrelated global path.
Invalid native defaults cause refusal before acquisition or cleanup. The old cache and checkout directories remain untouched.

The [source-path regression](../../plans/proof/starport-production-catalog/csp2/source-paths-red.json) records the old default before the change.
The [integration checks](../../plans/proof/starport-production-catalog/csp2/source-paths-integration.json) verify selected HTTP cache files, explicit override precedence, and selected provider and author logos.
No released product has acceptance credit for these changes.

The [CLI regression](../../plans/proof/starport-production-catalog/csp2/file-manifest-cli-red.json) records refusal of the previously absent `config paths` command.
The command now reports four roots, 26 default file entries, and seven external roles without opening catalog state or creating files.
JSON and YAML preserve origins, anchors, creation conditions, and recovery rules. They exclude credential values.

The inventory covers all nine files observed after isolated persistent startup and an accepted catalog publication.
Six reserved entries explicitly identify unimplemented writers. File patterns do not prove existence, permissions, or active ownership.
Migration and optional file flows still need complete inventory qualification.

The optional `config paths --inspect` command now reports bounded filesystem observations and stored runtime owner bindings.
The metadata budget includes unmatched entries. Inspection preserves configuration, owner, seed, and staging bytes in local tests.
An active runtime retains its lock after inspection. Workspace names with literal glob characters retain their staging patterns.

Linux and macOS metadata does not prove effective access or ACL safety. Windows permission and owner inspection remain unverified.
The [inspection proof](../../plans/proof/starport-production-catalog/csp2.md) records exact checks and remaining qualification work.

The [permission regression](../../plans/proof/starport-production-catalog/csp2/private-runtime-red.json) showed runtime writes under exposed directory and lock modes.
Linux and macOS startup now checks the runtime directory, lock, owner record, and seed for private mode bits and the effective UID.
The application checks before baseline export. Runtime ownership repeats the check before identity writes.
The guard does not change operator permissions. A local recovery test verifies diagnostic access and successful startup after an explicit mode correction.

Go test directories do not imply owner-only leaf modes. Successful runtime fixtures now set `0700` explicitly.
The exposed-mode tests still select rejected modes. Ancestor access, other private file roles, and Windows enforcement remain unverified.

The [native ACL regression](../../plans/proof/starport-production-catalog/csp2/darwin-acl-red.json) reproduced public access grants despite owner-only macOS mode bits.
Startup now refuses those grants on the runtime directory, lock, owner record, and seed.
Owner-specific allow entries and deny-only entries remain valid. The adapter refuses inherited non-owner grants.

The adapter reads native ACL metadata through a file descriptor and preserves file bytes, modes, and ACL entries.
It enforces an owner-only grant policy rather than computing effective access from ordered allow and deny entries.
The [ACL proof](../../plans/proof/starport-production-catalog/csp2.md#native-macos-acl-guard) records native recovery, malformed metadata, and toolchain checks.

The Windows runtime previously relied on mode arguments that do not establish a restrictive Windows DACL.
The new adapter reads owner and DACL information through an open handle and checks the process account and permitted grants.
Exclusive creation now supplies the owner and protected DACL for runtime directories, locks, staged identity files, and migration staging roots.
The [Windows proof](../../plans/proof/starport-production-catalog/csp2.md#windows-acl-implementation) separates local policy checks from unverified native execution.

Windows-targeted analysis also identified the existing unsupported workspace directory-exchange implementation in `internal/catalog/workspace/swap_other.go`.
Runtime and workspace code previously opened directories for reading before `Sync`.
[Microsoft requires write access for `FlushFileBuffers`](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers), which Go calls for Windows `Sync`.
The shared directory-flush operation now requests writable Windows directory handles before it calls `FlushFileBuffers`.
Native execution and filesystem durability remain unverified.

The earlier Windows migration publisher joined child names to `os.Root.Name()` and called `MoveFileEx`.
A renamed parent could leave that original pathname attached to a different directory.
The shared `internal/filepublish` operation now uses native directory handles for source and target selection.
Windows uses `NtSetInformationFile` with replacement disabled. Its native execution remains UNVERIFIED.
See the [Microsoft rename contract](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information).

Baseline export now uses the same operation. Linux and macOS request exclusive rename in the filesystem call.

The preceding collision test passed because Go checked the destination before rename. It did not prove an atomic refusal contract.
The [publication proof](../../plans/proof/starport-production-catalog/csp2.md#directory-publication-contract) records this distinction and the remaining durability work.
The prepared native runtime job exercises runtime, configuration, paths, baseline, catalog storage, and workspace packages on six native runners.
It has not run on GitHub.

Workspace staging also previously flushed regular files opened with `os.Open`, which requests read access.
Those stage-owned files now open with `os.O_RDWR` before flushing.
The shared directory operation reopens the current Windows directory through an empty NT object name under its existing handle.
It preserves filesystem and access errors. It does not use a no-op fallback or establish power-loss qualification.

The [late-publication regression](../../plans/proof/starport-production-catalog/csp2/close-late-publication-red.json) proved that runtime cancellation did not prevent a late provider result from publication.
Source retention, provider retention, catalog rebuild, and commit now check cancellation before starting their work.
Canceled provider publication no longer advances successful acquisition freshness. These checks do not undo work that already committed.


Release import accepted source-checkout overlap with the human catalog workspace in a regression test.
Sync and import now verify that separation before publication. Projection also checks before workspace writes.
The [regression evidence](../../plans/proof/starport-production-catalog/csp2/source-import-separation-red.json) records the earlier behavior.

Two [relative-anchor regressions](../../plans/proof/starport-production-catalog/csp2/relative-anchor-red.json) reproduced silent path changes before the new guard.
One started a replacement runtime while old state remained under an unknown working-directory anchor.
The other selected a different primary file because both anchors contained the same relative filename.

Legacy relative selections now require an absolute path or an explicit `relative_path_base: config` declaration.
The declaration records intent for new configuration-root paths. It does not infer the old anchor or establish migration completion.
Primary-file selection requires bootstrap intent before file reads. Runtime and workspace resolution refuse ambiguity before baseline writes.
Update workspace and source selectors resolve before catalog or credential access.

The new tests retain old seed bytes, preserve identity across absolute-path restarts, and verify explicit relative anchors from different working directories.
They cover flag, YAML, environment, and explicit dotenv declarations. Empty values restore refusal, and invalid declarations exclude their values from errors.
Starport integration and native platform qualification remain incomplete.

A [file-source regression](../../plans/proof/starport-production-catalog/csp2/relative-file-source-red.json) found another unguarded relative filename after the first guard checks.
Starmap now resolves file-source paths before baseline writes and supplies that absolute filename to runtime composition.
The file inventory reports the same path and anchor. Refresh tests read the configured payload from two other working directories with conflicting filenames.

### Accepted Windows workspace behavior

The user accepted D22 on 2026-09-06: journaled Windows replacement may briefly leave the optional YAML workspace path unavailable.
The accepted design requires retryable Starmap reads and recovery from a recorded operation.
It must preserve the accepted inference catalog.
It does not accept partial source observations, lost operator edits, or weaker authority checks.

The Windows workspace adapter now uses a replacement journal for existing directories.
Native Windows execution and durability qualification remain pending.
The shared reader guard now refuses reads while a writer holds the workspace lock.
It covers each complete workspace load, including the endpoint checksum used to bind acquisition and rollback inputs.
This coordination also applies to Linux and macOS directory exchange.

The [reader regression](../../plans/proof/starport-production-catalog/csp2/workspace-reader-red.json) reproduced normal presence results during an active writer.
The absent-path case could report an ordinary missing workspace during replacement.
The guard now returns a typed conflict and publishes no source result.
A first writer leaves its lock file behind. Its creation invalidates an overlapping passive read even when that writer finishes before the final check.

Passive reads create no files. External editors remain outside the shared lock protocol.

Startup, local sources, pipeline input, release import, rollback input, and bootstrap-manifest input now use the workspace read guard.
Accepted catalog reads in memory retain their existing access path.
A pending journal now refuses complete workspace reads, including when the workspace path is absent.
Projection and repair validate recorded directory identities and contents before recovery.

The [publication regression](../../plans/proof/starport-production-catalog/csp2/workspace-publication-cancellation-red.json) reproduced cancellation before both first publication and replacement.
Both operations incorrectly returned a receipt and changed the workspace.
The projector now checks cancellation immediately before the publication call.
The [corrected check](../../plans/proof/starport-production-catalog/csp2/workspace-publication-cancellation-green.json) preserves the old tree and marker, or leaves first publication absent.

### Windows workspace recovery implementation

The journal implementation stages a complete candidate and retains the previous directory until it saves the new projection receipt.
Recovery checks physical directory identity, file inventories, catalog payloads, and endpoint identity.
A copied candidate, corrupt intent, unexpected workspace, or changed backup produces a refusal that preserves the conflicting files.
Interrupted cleanup accepts only a verified subset of the previous inventory.

The first publication collision fixture did not reproduce an overwrite.
It preserved the competing directory but returned a generic I/O error. Exclusive publication now reports a typed conflict.
The pending-journal fixture did reproduce an incorrect absent-workspace observation. That read now returns a typed conflict.

The file inventory includes the fixed journal, temporary journal writes, and backup trees.
Inventory generation remains passive. Persistent startup attempts recovery only when it has a durable current generation.
A constructor without that generation refuses the pending workspace read.

The prepared Windows test holds an editor directory handle without delete sharing, then checks recovery after closing it.
That test has not run on Windows. Cross-compilation does not establish native behavior or power-loss durability.

### File policy inventory and access observations

The file inventory previously lacked structured selectors, applicability, access, retention, and removal conditions.
Every managed and external role now carries those policies. Unknown roles fail instead of receiving an implicit policy.
The current default report has 28 managed entries and seven external roles. A selected file source adds one managed entry.
The historical 26-entry report predates workspace journal and backup roles.

JSON and YAML expose each full policy. Wide text shows access and retention classes.
Inspection reports visible POSIX conflicts with the owner-only mode and owner requirements.
A conflict does not establish effective exposure through parent directories or ACLs.
A present file without a conflict remains unverified. Inspection changes no bytes or permissions.

The previous retained-evidence writer used `0755` directories and `0644` record files, subject to the process umask.
GitHub discovery also used shared directory modes and changed temporary files to `0644` before publication.
The private runtime parent provided a separate protection boundary.

Both writers now use the shared private-file implementation for creation, bounded reads, and publication.
Existing exposed or linked paths fail before use. The library checks the original directory identity during each operation.
A missing bound directory is a conflict rather than a cold-start record absence.
Unique temporary files preserve legacy scratch files. Cleanup requires the original temporary file identity.

The native ACL implementation moved into the shared package. Runtime identity validation retains its public entry point and error fields.
The native parser tests moved with their owning code. The prepared workflow now includes the shared package and GitHub source tests.
Migration catalog validation uses passive bindings and creates no missing evidence directories.
Native Windows execution and filesystem durability remain unverified.

### Private configuration input enforcement

The YAML reader previously read the selected symlink target without private-access checks or a byte limit.
The explicit dotenv reader used an unrestricted file read.
Three regressions reproduced exposed YAML, exposed dotenv, and an exposed selected symlink target.

Both readers now use the shared bounded private-record reader with a 1 MiB per-file limit.
POSIX checks require effective-user ownership and no group or other permissions. Supported native ACL checks also apply.
Read-only private files and explicit symlink selection remain valid. Broken selected targets cause a conflict instead of optional-file fallback.

Dotenv access failure preserves the process environment. Migration configuration digest verification uses the same reader.

The CLI guide and generated settings reference describe the access requirements and recovery procedure.
These file checks do not establish trusted ancestors or qualify service-owned mounts.
Native Windows execution remains unverified. The prepared test binaries include a public-ACL refusal and explicit correction case.

### Container root and rollout contracts

Classification: verified deployment defects owned by CSP2.
The previous read-only container command mounted `.starmap`, while current startup selected canonical user-data and state roots elsewhere.
A real Linux ARM64 run failed when baseline creation reached the unmounted user-data path.
Compose and Kubernetes also lacked explicit root selectors. Their new contract tests failed before the examples changed.

The corrected recipes select mounted application roots explicitly. Compose retains its existing image-directory volume boundary and selects a separate `starmap` child.
An isolated smoke test starts the current binary with an unusable process `HOME`, no external network, and no provider credentials.
Durable replacement preserves baseline and identity bytes. A separate configuration mount remains read-only.
The ephemeral recipe also reaches HTTP liveness and readiness.

The former Kubernetes Deployment used the default rolling strategy for one filesystem writer.
A replacement can wait for a lock held by the old ready Pod. The revised example uses `Recreate` during upgrades.
It also requires provisioned ownership instead of recursive group-permission changes over private records.
These corrections do not qualify a storage driver, Kubernetes cluster, or the complete replicated Starport recipe.

### Native Linux ARM64 filesystem execution

Classification: verified local execution evidence owned by CSP2.
The ten packages in the prepared native CI roster passed under Docker Desktop's Linux kernel.
The run produced 697 test and subtest pass results, with no failures or skips.
The [evidence audit](../../plans/proof/starport-production-catalog/csp2/native-linux-arm64-audit.json) verifies raw counts, terminal exits, and temporary-resource cleanup.

The unprivileged containers used named volumes for test files, read-only source fixtures, and no external network.
Process-exit tests exercised baseline export, runtime locking, migration recovery, and workspace journal recovery.
The portable Windows ACL policy tests passed on Linux. Windows API behavior and editor-handle conflicts remain unverified.

This execution used Go 1.25.12 with CGO disabled and no race instrumentation.
It does not qualify power-loss durability, service-manager recipes, other platforms, or the released product pair.

### Baseline staging cleanup ownership

Classification: verified data-preservation defect owned by CSP2.
The exporter previously called `RemoveAll` on its temporary directory name when the operation returned.
The [regression run](../../plans/proof/starport-production-catalog/csp2/baseline-stage-cleanup-red.json) recorded 14 failed test and subtest results.
Added files, changed files, replaced files, and replaced directories did not remain at their original staging paths.
Cleanup also deleted a directory that reused the staging name after successful publication.

The exporter now retains creation identities and checks the complete staging inventory before publication and cleanup.
It removes only unchanged files created by that operation, then removes the empty directory without recursion.
The check reads at most three directory entries for the two-file baseline stage.
Successful publication disables cleanup of the former staging name. Original cancellation errors remain available alongside cleanup conflicts.

Review also reproduced cancellation after payload creation but before publication.
The [cancellation regression](../../plans/proof/starport-production-catalog/csp2/baseline-stage-cancellation-red.json) failed because the canceled export still published its directory.
A context check now precedes the rename call. The [CSP2 proof](../../plans/proof/starport-production-catalog/csp2.md#baseline-staging-cleanup) records verification and remaining limits.

### Passive constructor and recovery ownership

Classification: verified D6 contract violation owned by CSP2.
`newClient` previously called `workspace.Repair` after loading a durable current generation.
The [regression](../../plans/proof/starport-production-catalog/csp2/constructor-passivity-red.json) failed for both absent and stale workspaces, plus their parent test.
Supplying a readable durable store caused the read-only constructor to create or replace workspace files.

Construction now selects the durable catalog without repairing its workspace.
The explicit `Client.RepairWorkspace` operation owns that repair and serializes it with catalog publication.
Connected runtime startup invokes repair, preserving automatic application recovery and operator-file protection.
The operation reports completed changes and preserved operator edits. It does not change the durable head or in-memory catalog sequence.

The first native integration run found one migration fixture that still expected passive application access to restore files.
The corrected fixture checks passive file preservation before runtime startup restores the workspace.

Execution also found a task-order error. CSP5 already owns staged-write recovery and retention.
Abandoned Starmap stages remain assigned to CSP5, while CSP8 owns temporary Starport scratch cleanup under A43.
The complete recovery and retention requirements remain in scope. CSP2 retains path, access, migration, and native-startup obligations.

The first macOS race run exceeded the existing one-minute workspace repair budget for the full embedded catalog.
The original fixture and failure remain in the [CSP2 proof](../../plans/proof/starport-production-catalog/csp2.md#constructor-passivity-and-repair-ownership).
The focused lifecycle fixture uses a small valid durable generation. CSP22 retains complete-workload repair qualification.

### Constructor network contract conflict

Classification: verified contract conflict owned by CSP2.
D6 prohibits constructor network requests. The current constructor calls `Current` on caller-supplied storage, including the existing S3 object adapter.
A strict offline constructor and automatic remote-store loading require separate operations. Permitting explicit storage reads would require an accepted clarification to D6.
The owner question remains pending. Network-silence acceptance stays UNVERIFIED.

The [local S3 probe](../../plans/proof/starport-production-catalog/csp2/constructor-network-boundary-probe.json) confirmed one HTTP request during successful construction.
The real adapter used anonymous credentials and a loopback server. The probe used no external service or provider key.

The [worker probe](../../plans/proof/starport-production-catalog/csp2/constructor-remote-workers-probe.json) also recorded two HTTP transport goroutines after construction returned.
Passing default and memory-store checks cannot qualify worker silence across the remote-store boundary.
Both network silence and worker silence remain UNVERIFIED pending the owner decision.

### POSIX ancestor access

Classification: verified filesystem access defect owned by CSP2.
Private directory bindings checked the selected directory and file, while callers supplied no ancestor guard.
The [regression](../../plans/proof/starport-production-catalog/csp2/ancestor-access-red.json) failed create, bind, read, and write cases plus their parent test.
A writable ancestor allowed directory creation and record replacement despite private leaf permissions.

Linux and macOS now check trusted ancestor ownership and directory-entry protection before these operations.
Creation uses directory handles and verifies each child before descent. Retained-file publication checks ancestors again before replacing the record.
Runtime startup checks ancestors before exporting the baseline. Configuration checks run before parsing and include intermediate symlink targets.

The [configuration symlink regression](../../plans/proof/starport-production-catalog/csp2/ancestor-config-symlink-red.json) exposed a route through an unsafe intermediate target directory.
Checking only the selected parent and final target missed that directory. The guard now validates the complete selected symlink route.

The [CSP2 proof](../../plans/proof/starport-production-catalog/csp2.md#posix-ancestor-access) records native checks and remaining limits.
Native Windows ancestors, other file roles, managed-service file exceptions, and full filesystem qualification remain open.

The [traversal regression](../../plans/proof/starport-production-catalog/csp2/ancestor-traversal-red.json) also showed that normalization could remove an unsafe directory before `..`.
The guard now checks path components before parent traversal and bounds symlink expansion. Trusted relative symlinks remain valid.

The next file-role audit targets `pkg/catalogs/storage/filesystem.go`.
Its commit path still uses recursive directory creation and the public catalog tree's `0755` directory and `0644` file defaults.
The shared private-record guard does not cover that store. CSP2 must reconcile its access policy with the private catalog-state descriptor.


### Private filesystem catalog store

Classification: verified access-policy defect owned by CSP2.
The filesystem adapter shared public catalog-tree modes and omitted private metadata and native ACL checks.
Eleven regression results failed before the change, including publication after the root became unsafe.
The adapter now uses private creation, access checks, directory-bound publication, and identity checks for the commit lock.
Existing unsafe stores require operator correction, including before legacy-store migration.

The [CSP2 evidence](../../plans/proof/starport-production-catalog/csp2.md#private-filesystem-catalog-store) records failures, fixture changes, and corrected checks.
Focused verification passed 47 macOS race results and 246 native Linux results across three packages.
Windows storage binaries cross-compiled for both architectures. Native Windows execution and full filesystem durability remain UNVERIFIED.
This work resolves the store access gap identified in the preceding ancestor review. CSP5 retains abandoned-stage collection.


The post-rename flush boundary remains a CSP5 obligation. A failed directory flush can return an error after the new current pointer becomes visible.
The store contract's unconditional failure-preservation statement therefore needs an explicit ambiguous-outcome rule and recovery tests.
The current evidence verifies refusal before publication. It does not prove failure preservation after publication or power-loss durability.


### Windows ancestor access

Classification: verified missing ancestor enforcement, owned by CSP2.
The former `ValidatePOSIXAncestors` adapter returned success without checks on Windows.
The shared `ValidateAncestors` boundary now selects a Windows component walker and native DACL policy.

The [CSP2 evidence](../../plans/proof/starport-production-catalog/csp2.md#windows-ancestor-access) records 18 failed portable policy results before enforcement.
The initial corrected checks passed 96 macOS race results. Integration passed 326 results, and native Linux checks passed 70 results.
These results exercise shared code and the portable Windows policy. They do not execute Windows filesystem APIs.

Native tests now cover canonical user paths, shared-read ancestors, unsafe grants, symlink targets, path forms, and explicit correction.
The native constants test also checks the TrustedInstaller SID and the directory-rights mask against Windows APIs.
All native Windows results remain UNVERIFIED. Service procedures and owner-policy exceptions remain open.

### Catalog store inspection policy

Classification: verified reporting mismatch, owned by CSP2.
The store enforced private access while its file descriptor still declared `deployment-controlled` access.
POSIX inspection therefore omitted mode conflicts on the store root and matching files.

The descriptor now declares `owner-only` access. The regression covers the root, current pointer, and a generation payload through application inspection.
The checks also require unchanged bytes and permissions, without catalog or credential initialization.
Native Windows permission observations remain UNVERIFIED and require their own implementation and execution evidence.

### Windows file inspection

Classification: verified missing native observation adapter, owned by CSP2.
Windows previously returned only `native-permissions-unverified`, although runtime enforcement already inspected native descriptors.
The adapter now reads ownership and DACL metadata through a bound handle and shares the enforcement decoder.

Portable tests verify policy conflicts, retained uncertainty, descriptor summaries, serialization, and owner SID output.
Prepared native tests cover ACL observations without content reads, retained permissions, changed identities, symlink refusal, and native descriptor states.
Native execution remains UNVERIFIED. Compilation and portable tests do not qualify these Windows API behaviors.

### Workspace replacement changes operator permissions

Classification: verified permission-preservation defect, owned by CSP2.
A macOS replacement probe changed the workspace root from `0770` to `0755` and an unrelated private note from `0600` to `0644`.
The content survived. The replacement removed group write access from the root and added read bits to the note.

The initial `internal/catalog/workspace/projector.go` implementation set its candidate root to the default mode and copied files through `os.CopyFS`.
The [review evidence](../../plans/proof/starport-production-catalog/csp2/workspace-permission-review-red.json) records the source, command, failed assertions, and temporary-source cleanup.
Actual exposure depends on ancestor access and ACLs. CSP2 must preserve the declared access policy or refuse before publication.


## Workspace access binding update

CSP2 now includes ownership and native ACL metadata in workspace replacement snapshots.
The atomic path checks the full tree before publication. Version 2 journals require access digests through recovery and backup cleanup.
The macOS workspace race suite passed 115 test events. Native Linux ARM64 passed 107 events without race instrumentation.

Windows AMD64 and ARM64 compiled, but native Windows behavior remains UNVERIFIED.
Permission preservation during staging remained incomplete at that checkpoint. Those checks did not close the earlier workspace mode defect.
See the [snapshot record](../../plans/proof/starport-production-catalog/csp2/workspace-access-snapshots.md).

## Workspace access preservation and review update

Private staging now restores existing access and checks content and access digests before candidate publication.
The assembler preserves tested modes, native ACLs, inherited defaults, and non-YAML operator notes inside model directories.
The [preservation record](../../plans/proof/starport-production-catalog/csp2/workspace-access-preservation.md) records 129 macOS workspace race events and 120 native Linux workspace events.
These component checks passed without failures or skips. Windows and foreign-owner restoration remain UNVERIFIED.

The six-package macOS run passed 695 events and failed `TestRuntimeDirectoryLockSurvivesProcessInterruption`.
Ten focused reruns passed, but the failure remains unresolved. CSP2 owns the diagnosis and required runtime verification.

The [updated conversation review](../../plans/proof/starport-production-catalog/csp2/access-policy-conversation-review.md) confirms the private-store and editable-workspace boundary.
Shared semantic policy ownership remains incomplete. Starport still pins Starmap v0.16.5 and lacks the new file-manifest consumer.
CSP8 owns adoption, and CSP12 owns storage-engine access and recovery checks. No primary acceptance status changes.

## Shared file-role policy and process-lock diagnosis

Starmap now resolves access classes through `pkg/productpaths/policy`. Runtime adapters check their supported class before file access.
Diagnostics and enforcement share POSIX mode and ownership classification. The Windows DACL policy remains shared.
The application retains product-specific selectors, availability, retention, and recovery text.

The process-lock failure came from an unreachable lock in the child test. Forced garbage collection reproduced it in five runs.
An explicit lifetime keeps the descriptor open until the parent releases the child. Production runtime lock ownership remains unchanged.
The [shared-policy evidence](../../plans/proof/starport-production-catalog/csp2/shared-file-policy.md) records 925 passing macOS race events without skips.

Native Linux passed 803 core events across ten packages. Models.dev passed 69 events and failed three tests because native tooling was absent.
Those tests still need Bash, Git, and curl. The runner removed all eleven temporary containers and volumes.
The Windows inheritance correction and prepared native regression compile for both architectures, but native execution remains UNVERIFIED.

Starport adoption, service procedures, foreign-owner restoration, and native qualification remain open. No primary acceptance case changes status.

## Native Linux ownership and tooling qualification

The [native ownership evidence](../../plans/proof/starport-production-catalog/csp2/native-ownership-and-tooling.md) records 72 passing tooling events and 15 passing ownership events.
The ownership fixture tests actual UID/GID changes inside an isolated volume.

An unprivileged service preserves the original workspace when ownership restoration fails. Authorized supplementary-group updates succeed without capabilities.
A parent with scoped capabilities preserves retained ownership and access metadata through both replacement mechanisms.
These tests require no production code change. Other native platforms, service procedures, and Starport adoption remain open.

## CSP3 retained provider evidence validation

The initial regression showed that retention accepted inconsistent digests, mismatched provider identities, missing observation times, and mutable caller-owned payloads.
An invalid later batch member also failed to prevent earlier retention. All 17 initial test events failed.

The [CSP3 record](../../plans/proof/starport-production-catalog/csp3.md) documents batch validation, owned payloads, and restart filename checks.
The focused correction passed the same 17 events. Additional fixtures cover aliases and payloads with multiple providers.
Scope, completeness, field precedence, and deletion remain separate incomplete contracts.

The targeted pricing probe passes a newer positive price but retains the baseline price when the provider supplies explicit zero.
This evidence narrows the field-precedence defect. Presence and scope metadata must survive the complete observation lifecycle before reconciliation can distinguish unknown from explicit zero.

## Owner decisions and explicit-zero pricing correction

The owner resolved constructor storage reads, service-managed primary configuration, and publication authority on 2026-09-06.
The [decision record](../../plans/proof/starport-production-catalog/csp2/owner-decisions-2026-09-06.md) preserves their exact scope.
Service configuration implementation and native qualification remain incomplete.

The pricing review found that the canonical contract already treats a present zero-valued token-cost object as free.
The generic model merge previously skipped zero numbers and combined price components through reflection.
It now selects valid updated pricing as a complete copied object. Missing or invalid updated pricing retains the existing object.

The [focused pricing checks](../../plans/proof/starport-production-catalog/csp3/pricing-presence-green.json) pass 13 race events, including retained evidence after restart.
Scope, completeness, effective-time selection in the runtime, and deletion remain open. No primary acceptance case gains credit.


## CSP3 provider observation order

A [retention regression](../../plans/proof/starport-production-catalog/csp3.md#provider-observation-order) reproduced stale replacement and conflicting equal-time payloads, including retained restart state.
Retention now selects the newest unambiguous provider observation and rejects time regressions before batch writes.
Identical observations require no rewrite. Concurrent provider retention serializes selection and durable writes.

A subsequent regression reproduced retained writes after cancellation between the publication check and retention.
Retention now forwards the context through ordered selection and private-file publication.
The final correction passed 1,111 race events, Ago, lint, and generated-document checks.
It does not establish account scope, completeness, deletion, timestamp authenticity, or transactional multi-file retention.

CSP3 remains in progress. Publication requires final verification and review. The file-classification correction below resolves the earlier frozen-output lint blocker.


## CSP3 retained source receipts

The [receipt regression](../../plans/proof/starport-production-catalog/csp3.md#retained-source-observation-receipts) proved that acquisition and restart lost source completeness and classified evidence metadata.
Provider layers now retain validated receipts. Receipt restoration checks observation identity, payload checksum, record counts, and classified issues without retaining original diagnostic messages.
Partial receipts produce degraded acquisition health. Altered receipts and oversized serialized records cause refusal before batch writes.

The correction passed 1,164 race events, Ago, code lint, generated-document checks, and the pure-Go consumer gate.
All 298 recorded code and module inputs remained unchanged during verification.
Account scope and effective-generation provenance still require integration. Old provider files without receipts require the explicit recovery procedure owned by CSP18.


### CSP3 observation identity boundaries

The [identity regression](../../plans/proof/starport-production-catalog/csp3.md#unambiguous-observation-identity) proved that NUL separators could encode two issue lists as one identity.
Receipt restoration then accepted changed issue boundaries. New observation IDs use versioned byte-length fields and explicit record and issue counts.
Safe legacy identities remain readable. Legacy fields with NUL separators cause refusal and require verified recovery under CSP18.

The completed package runs passed 1,663 race events. The first aggregate attempt reached its five-minute root-package limit.
The root rerun passed with the repository's 20-minute limit. All 474 recorded inputs remained unchanged during these checks.

A subsequent lint correction preallocated field capacity without changing the identity encoding.
Final verification passed 30 focused race events, Ago, and whole-module lint.

This correction does not establish source authority, account scope, or verified upstream coverage.


### CSP3 credential isolation between observations

The [concurrent-observation regression](../../plans/proof/starport-production-catalog/csp3.md#credentials-within-concurrent-observations) proved that a second observation replaced the first observation's selected credentials.
The source shared one memo across concurrent calls and cleared it when a call started.
The source now creates a separate memo per observation. Preflight and fetch use the same resolved material through the public fetcher.
The process resolver retains ownership of credential caching and renewal.

The correction passed 232 race events across five packages, with no failures or skips.
All 82 recorded inputs remained unchanged. Ago, code lint, and generated-document checks passed.
The tests also prove that absent or invalid credentials in one observation cannot affect a concurrent peer's successful request.

Credential profile and material version do not establish account identity.
Named acquisition bindings and their scope revisions remain necessary before source receipts can authorize retention or deletion within an account scope.


### CSP3 provider-binding declarations and receipt integrity

The [binding regression](../../plans/proof/starport-production-catalog/csp3.md#typed-provider-acquisition-bindings) proved that readers silently discarded binding metadata and accepted an unscoped identity.
The typed binding now declares provider scope, revision, region, API surface, and credential role/profile.
Scoped observations use v3 identities that include every binding field. Unscoped v2 and safe legacy evidence remain readable.
Receipt copies and restart preserve bindings and reject altered metadata.

The broader checks passed 723 race events, Ago, code lint, and generated-document checks.
All 175 recorded inputs remained unchanged. Final constructor checks passed 60 focused race events, Ago, and code lint.
The constructor validates selectors before computing the observation identity.

This contract validates declared metadata. It does not verify account ownership or the credentials used by a request.
The runtime still retains one layer per provider. Acquisition selection, active binding revisions, and independent scope retention remain open under CSP3.

### CSP8 candidate dependency compatibility

The [local pair check](../../plans/proof/starport-production-catalog/csp3.md#local-starport-pair-check) failed Starport's acquisition publication test against this Starmap worktree.
The injected observer omits the receipt that the candidate runtime requires. Starport's pinned `v0.16.5` does not exercise that candidate validation.
The check used a temporary module replacement and left all three recorded Starport inputs unchanged.

CSP8 must update the fixture to retain a real source receipt when it adopts the compatible Starmap module.
Passing tests against the existing module pin cannot qualify the candidate pair. The released-pair acceptance gate remains UNVERIFIED.

### CSP3 explicit binding acquisition

The [binding acquisition checks](../../plans/proof/starport-production-catalog/csp3.md#acquisition-profile-selection-for-bindings) pass 15 focused race events.
Explicit source calls select one provider and restrict credential resolution to the binding's declared acquisition profile.
A mismatched profile causes refusal before client creation. Concurrent calls retain separate selections and memos.

The acquirer now exposes explicit binding observations without publication. Its default observer preserves the binding and partial-source receipt in the returned layer.
It rejects unscoped custom observers and receipts for another binding. Scheduled acquisition and runtime scope enforcement remain incomplete.
Account ownership and selector coverage still require separate evidence.

The broader checks passed 287 race events across five packages. All 71 recorded code and module inputs stayed unchanged.
Code lint, Ago, and generated-document checks passed. That run failed aggregate lint on three frozen-output diagnostics.
The primary acceptance gate remains UNVERIFIED.

### CSP3 separate provider scope retention

The [scope regression](../../plans/proof/starport-production-catalog/csp3.md#separate-provider-scope-retention) proved that two scopes competed for one provider record.
Retention now separates provider, binding identity, and revision in memory and on disk. Scoped records use hashed filenames in a private `providers/bindings` directory.
Unscoped records keep their provider filenames and do not become public-scope declarations.

Batch and restart validation require one consistent declaration for each binding identity and revision across providers.
A selector change requires a new revision. The runtime rejects invalid batches before writes and rejects inconsistent retained records during restart.

Active revision selection, revocation, and scoped deletion remain open.
Source inspection found a further risk: `initializeEffective` takes its baseline from the client's current catalog, which can contain previously merged evidence.
CSP3 must prove that revoked records cannot return through that baseline after restart. This risk still needs a behavioral regression.

Broader verification passed 508 race events across five packages. All 157 recorded inputs stayed unchanged.
Ago, code lint, and generated-document checks passed. Active scope policy and baseline revocation still require verification.

### CSP3 compiled baseline provenance

The [baseline regression](../../plans/proof/starport-production-catalog/csp3.md#baseline-provenance-after-durable-publication) confirmed the restart risk found during scope retention.
The accepted current catalog became the reconstruction baseline. Excluding a provider layer then failed to remove that provider from the rebuilt result.

The client now exposes its separate compiled catalog through `EmbeddedCatalogState`. Runtime reconstruction uses that immutable input.
The accessor test proves that stored current state and later updates cannot change its identity or catalog.
The getter reads no storage, decodes no payload, and allocates no memory in the focused check.

A second regression proved that parsing `.local.` could truncate a valid baseline identity. The runtime now treats baseline identities as opaque values.
The existing durable restart tests still verify stable identities and commit counts.

This correction does not decide whether startup may serve an accepted head after a policy change.
Active revision selection and accepted-head admission remain open. Explicit local inputs still require their own reconstruction evidence.

Broader verification passed 458 race events across four packages. All 155 recorded inputs stayed unchanged during that run.
Code lint, Ago, generated-document checks, and all six external consumer compositions passed.
Three focused race events passed after a test-comment correction. Active revision admission and operator revocation remain incomplete.

### CSP3 provider refresh windows

The [window regression](../../plans/proof/starport-production-catalog/csp3/provider-window-red.jsonl) recorded nine failed and one passed test events.
Tracking publications by provider ID suppressed another account, revision, unscoped record, or newer observation returned after an early window.
A modified final receipt also escaped validation. The aggregate retained report hid an unanswered scope when another scope for that provider succeeded.

The runtime now tracks exact validated observations within each provider scope revision.
It validates final receipts before duplicate checks and keeps aggregate retained-provider reporting aware of unanswered scopes.
The [focused correction](../../plans/proof/starport-production-catalog/csp3/provider-window-green.jsonl) passed all ten race events, including disk reads after publication.
This component does not implement active revision admission, binding-level attempts, or scheduled binding selection. CSP3 remains in progress.

The [broader verification](../../plans/proof/starport-production-catalog/csp3/provider-window-verification.json) passed 375 race events across runtime, acquisition, and server packages.
All 117 recorded code and module inputs stayed unchanged. Ago, code lint, and generated-document checks passed.
That run failed the writing check on frozen command output.

### CSP3 explicit active binding policy

The [policy regression](../../plans/proof/starport-production-catalog/csp3/provider-policy-red.jsonl) recorded ten failed and three passed events before runtime enforcement.
Inactive observations returned after restart, including when retained files were absent but a merged current catalog remained.
The runtime now accepts a copied complete declaration set, filters inactive evidence, and rejects inactive publication before writes.
The initial correction passed thirteen race events.

A separate [HTTP regression](../../plans/proof/starport-production-catalog/csp3/provider-policy-http-red.jsonl) found that the server still read the previous underlying client generation.
Explicit-policy startup now aligns the runtime, client, HTTP manifest, and durable current selection before returning.
The [HTTP correction](../../plans/proof/starport-production-catalog/csp3/provider-policy-http-green.jsonl) passed against a real filesystem store and HTTP handler.

This mode uses a memory store if the caller supplies none. Selected identities bind the complete declarations, source identity, and payload checksum.
Publication failure prevents startup. An explicitly restored selection can reuse its immutable retained generation.
The runtime requires a binding-aware acquisition role before provider I/O. The built-in batch acquirer still needs that role.

This is an explicit Go API path. Operator settings, binding-level attempts, configuration-omission protection, and internal authority remain incomplete.
Legacy construction without an explicit set still permits unscoped behavior. It does not qualify the target production policy.

`server/application.go` forwards updates to the injected `Syncer` without checking active declarations.
The current policy governs connected-runtime publication. Operator integration must route server updates through the same policy before support qualification.
The expanded local contracts passed 32 race events. Broader checks passed 397 race events with unchanged inputs.

A final addressed-read test found that restoration checked the checksum but omitted the requested generation identity.
The guard now requires both values before activation. Its focused verification follows the broader run.

Four focused publication tests passed after the final identity guard. They covered HTTP state, immutable restoration, startup failure, and mismatched addressed reads.
The two frozen command-output files now use exact file exclusions under the existing historical-review policy. Their hashes remain unchanged.
Writing rules and severities remain unchanged. Final verification and review still precede publication.

The final writing check passed all 1,047 scanned files with zero diagnostics. Both frozen-output hashes remain unchanged.
The complete repository verification gate now precedes the authorized commit, review, draft PR, and native CI sequence.


### CSP3 binding batches and model membership

The [batch verification](../../plans/proof/starport-production-catalog/csp3/binding-batch-verification.json) closes the built-in acquisition-role gap described above.
Explicit batches preserve separate binding identities through credential selection, receipts, attempt reports, retention, and restart.
Invalid selections cause refusal before credential or provider work. Cancellation prevents late publication.

The runtime integration test also reproduced the F-004 membership filter.
Enrichment now preserves linked models without pricing or limits and carries their missing authored definitions.
The generic merge owns this behavior. Existing definitions still keep precedence.

The final checks passed 1,341 race events, 79 normal package suites, six consumer compositions, code lint, Ago, and generated documentation.
The first broader run remains failed evidence. The final run used Go 1.26.6 and the unchanged five-minute package limit.
Operator integration, scoped field authority, deletion rules, and released-pair acceptance remain open.


### CSP3 scoped reconciliation records

The manual source pipeline exposed another boundary before binding integration.
Collector maps replaced earlier catalogs under the same source type. Shared records also selected facts by input order.
The merger attached the last provider receipt to every model, including models supplied by another binding.

Reconciliation now validates scoped receipts and keeps each selected provider or model record with its observation.
Direct observations take precedence over stale fallback. Observation time then orders shared records within that classification.
Records with equal identity, time, and fallback classification must agree.
Primary-source filtering includes every scoped provider, and counts use unique model and source identities.
Review candidates and field provenance select the receipt for the chosen record.

The [scoped reconciliation proof](../../plans/proof/starport-production-catalog/csp3/scoped-reconciliation-verification.json) records the regressions and checks.
A second regression showed that a newer stale fallback could displace a direct peer observation. The correction preserves the direct observation.

The caller still owns active binding selection. The manual CLI and HTTP adapters do not yet enforce that selection.
Their pipeline must emit separate observations for the selected bindings and preserve runtime retention during publication.
Field-presence handling, scoped deletion, and released-pair acceptance remain open.


### CSP3 manual acquisition bindings

The manual syncer now accepts an explicit set of provider bindings during construction.
It checks each selected profile before source work and preserves separate observations through reconciliation and durable generation links.
An explicit empty set contacts no providers. Operation filters cannot add undeclared providers.
The shared provider-call limit applies across the selected bindings.

Strict mode previously counted only authored definitions. Provider observations can instead contain serving records linked to an authored catalog.
It now accepts either form of model data and still rejects empty, incomplete, failed, missing, duplicate, or mismatched observations.

The volume guard previously compared every provider account against one source-wide history and dropped binding metadata when it changed health.
Field provenance now preserves optional binding identity and revision through JSON and YAML.
The guard uses matching history and retains the binding when it creates a degraded receipt.
History without a matching binding revision supplies no scoped completeness claim.

The [manual binding proof](../../plans/proof/starport-production-catalog/csp3/manual-bindings-verification.json) records component verification.
CLI and HTTP adapters still need shared-setting composition and runtime retention during publication.
Scoped deletion, field-presence handling, and released-pair qualification remain open.


### CSP3 canonical runtime reconstruction

The runtime previously merged provider layers with a separate enrichment algorithm and published no provider receipt links.
Its effective catalog also accepted authored definitions from provider observations, contrary to the canonical reconciliation contract.

Runtime reconstruction now restores original observations and uses the canonical reconciler.
Field provenance, generation links, and excluded-model review candidates retain the corresponding provider receipt.
Legacy layers use the same record selection as scoped observations. The active binding policy remains a separate runtime check.

Stable generated timestamps preserve the payload for unchanged retained evidence.
Each reconstruction checks pricing validity at the current time. Its rejection text now identifies the fixed interval boundary.
Concurrent rebuilds serialize publication and effective-state activation.
A further regression found synthesized provenance for providers outside the primary selection. The filter now applies before provider reconciliation.

Three acquisition tests placed reviewed authored definitions only in their provider observations.
Their fixtures now supply those definitions through an explicit catalog source. Model-retention and partial-failure assertions remain unchanged.
A separate regression proves that a provider observation cannot establish authored identity, even when it includes a definition.

The [runtime reconciliation proof](../../plans/proof/starport-production-catalog/csp3/runtime-reconciliation-verification.json) records verification and earlier failures.
CLI and HTTP composition, upstream manifest lineage, field presence, scoped deletion, and released-pair qualification remain open.


### CSP3 receipt-only generation changes

A regression found that runtime publication reused an identity after an unselected provider receipt changed.
Selected catalog values and the payload checksum stayed unchanged. The durable manifest therefore retained the old receipt.

Effective identity now includes original source links and review candidates, with deterministic ordering.
The catalog payload checksum remains separate. Empty evidence preserves the baseline identity input.

A restore test supplies different manifest evidence under an existing identity.
The existing store contract refuses that update with an immutable generation conflict and preserves current state.
This test required no additional restore implementation.

The [receipt identity proof](../../plans/proof/starport-production-catalog/csp3/receipt-identity-verification.json) records the checks and original publication failure.
Manual source transactions, CLI and HTTP composition, and released-pair qualification remain open.


### CSP3 catalog acceptance before input retention

A failing store reproduced a rejected provider publication that changed retained files and became active after restart.
The runtime now stages immutable input records and a prepared transaction before catalog publication.
It replaces retained source and provider files only after acceptance. Startup recovery resolves interrupted transactions before loading those files.

A lost commit reply leaves a prepared record. Startup compares the loaded catalog with the prior and candidate identities and checksums.
A committed record completes retention after partial writes. Every referenced input must validate before replay writes any retained file.
An unresolved head or invalid record blocks recovery. Migration refuses pending publication.

Reports preserve accepted publication when retention fails. The active catalog remains available while further updates require recovery.
The concurrency fixture previously wrote retained inputs synchronously while the store blocked a catalog commit.
It now issues two complete publication requests. Its catalog, generation, and receipt assertions remain unchanged.

The [input publication proof](../../plans/proof/starport-production-catalog/csp3/input-publication-verification.json) records exact checks and the failed publication regression.
Manual non-provider observations still need retained input representation and CLI and HTTP composition.
Completed-input collection, shared fleet recovery, native qualification, and released-pair acceptance remain open.


### CSP3 shared manual composition and fresh-mode decision

The CLI update command and HTTP server previously constructed syncers without the configured provider binding array.
Both now call `App.CatalogAcquisition`, which applies the resolved set, credential resolver, and source directories.
The shared config accessor preserves omission and explicit emptiness and returns owned declarations.
Tests cover legacy selection, an empty set, separate scoped attempts, and unchanged state during previews.

Source selection and baseline enrichment now belong to the reconciler.
Pipeline acquisition, explicit observation publication, release imports, and runtime reconstruction call the same entry point.
The runtime still excludes acquisition clients from its dependency set.

D24 changes the target for fresh manual acquisition to preserve the embedded or selected upstream baseline.
The existing CLI uses `--force`, and the Go API uses `sync.WithFresh`.
The current pipeline still selects an empty reconciliation baseline. Manual runtime retention must implement D24 before acceptance.
It must also retain original observations, preserve reset scope and previews, and prevent retired binding evidence from returning through local projections.

The [manual composition proof](../../plans/proof/starport-production-catalog/csp3/manual-composition-verification.json) records 1,043 race events and corrected normal coverage of all 79 package suites.
This is component progress. Manual runtime publication, complete ingestion, native qualification, and released-pair acceptance remain open.


### CSP3 retained manual observations

`Runtime.PublishObservations` now joins manual publication to runtime ownership and input recovery.
It retains original payloads and safe receipts without source acquisition. Distinct concurrent calls retain distinct batches.
Observations already in manual history preserve the generation and sequence without another broadcast.

The first replay tests exposed two provider-order defects: older scheduled facts could replace newer manual facts, and a later omission could discard retained values.
Provider observations now share retained history once manual publication starts. Reconstruction uses the reconciler's fallback and observation-time policy.
A further test found that separate reviewed inputs in one batch could hide model definitions. Metadata passes now preserve those definitions through the baseline.
All changes still publish as one transaction.

Restart validates the original aggregate provider payload and receipt together. It excludes retired bindings and rejects selector changes without a new revision.
Recovery validates the complete parent history before installing source, provider, or manual files.
A lost catalog commit reply recovers the accepted generation. Rejected publication preserves the previous history.

The private `manual.json` head references immutable batches and original observations under `publication-inputs`.
Publication version 2 protects these records from older readers. New readers still accept version 1 records without manual history.
Native upgrade and downgrade qualification remains open.

The file manifest previously omitted scoped provider records and publication recovery files.
Its private runtime-evidence entry now lists those paths and manual history. A real publication test checks the files it creates and still rejects unknown paths.

History permits at most 4,096 batches and 64 MiB of encoded observations. Reaching either limit preserves accepted state and refuses new input.
CSP5 owns production compaction, collection, and recovery. These limits are component safeguards, not a production capacity claim.

The CLI and HTTP acquisition paths still need runtime publication and D24 reset scopes.
Local projection provenance, field presence, scoped deletion, native qualification, and released-pair acceptance remain open.
This implementation makes no request latency or allocation claim.

The [manual retention proof](../../plans/proof/starport-production-catalog/csp3/manual-retention-verification.json) records 991 passing race events and normal coverage of 79 package suites.


### CSP3 acquisition ownership and projected binding facts

Manual acquisition needs operation ownership throughout source preparation and catalog publication.
`Runtime.UpdateObservations` now provides that boundary. The callback receives the current catalog and a separate trusted baseline.
`Runtime.ObservationInputs` exposes the same immutable snapshots for read-only preparation. It reads no source and writes no files.
A callback failure or empty result preserves state. Shutdown cancels the callback and rejects its late result.

A focused regression reproduced a retired provider fact returning through a local catalog projection.
The reconciler now accepts a policy for unchanged projected fields. Runtime publication checks the original provider, binding identity, and revision.
The tests cover provider names and model limits, active and retired bindings, changed revisions, another provider's binding, and operator edits.
This closes that field-reuse gap. Scoped reset masks, membership deletion, and full authority enforcement remain open.

The broader race run timed out before a concurrency fixture reached its blocked store.
That fixture rebuilt the full embedded catalog to check publication ordering for two reviewed definitions.
It now uses those definitions without unrelated embedded records. Its deadlines and durable publication assertions remain unchanged.
Three focused race repetitions passed with the smaller fixture. The complete runtime race rerun passed 424 test events.

The [observation update proof](../../plans/proof/starport-production-catalog/csp3/observation-update-verification.json) records 1,004 passing race events and 79 normal package suites.
D24 reset scopes and CLI/HTTP runtime integration remain open. All 50 primary acceptance cases remain UNVERIFIED.


### CSP3 provider selection within retained observations

The first selection contract test failed: an excluded provider still overwrote the baseline.
Its original observation also contained an unrelated provider.

`WithProviderObservationSelection` now selects provider records by original observation identity.
The reconciler validates the original receipt before applying the selection. It preserves the original payload and receipt for the remaining provider.

Selection applies before conflict checks, collection, review-candidate evidence, and primary membership filtering.
Focused tests cover aggregate receipts, input-order stability, excluded conflicts, explicit empty selection, omitted selection, and invalid input.
They also check that later changes to the caller's selection map and slices cannot change the result.

This is a reconciliation component. Runtime reset scopes, retained reset evidence, source reset semantics, and CLI/HTTP adoption remain open.
The option does not remove facts from the selected baseline or independently enforce enterprise authority.

The [provider selection proof](../../plans/proof/starport-production-catalog/csp3/provider-selection-verification.json) records 1,018 passing race events and 3,594 passing normal events across 79 package suites.
All 50 primary cases remain UNVERIFIED.


### CSP3 durable provider resets

The first reset contract test reproduced a retained local limit surviving replacement.
`Runtime.UpdateObservations` now accepts provider reset scopes and requires complete successful replacement observations.
The reset scope includes the provider, binding identity, and revision. Legacy unscoped input is a separate scope.

Reset scopes and replacements enter one retained manual batch and catalog publication transaction.
Earlier scheduled files enter a preceding batch, so restart cannot restore their cleared provider facts.
Replay retains other providers within an aggregate receipt and other bindings of the same provider.
It also rejects unchanged projected facts from cleared observations while preserving actual operator edits.

A reset changes generation identity even when catalog bytes remain equal.
The lost-reply test confirms that recovery keeps both the accepted catalog and reset scope.
Rejected commits and failed preparation retain the previous history. Version checks reject reset fields under the legacy batch version.

Manual heads and batches use version 2. New readers still accept version 1 records without resets.
The byte bound now includes encoded reset scopes, and a request permits at most 4,096 scopes.
General source resets, projection membership, previews, adapter integration, native qualification, and released-pair acceptance remain open.

Component evidence: [provider reset verification](../../plans/proof/starport-production-catalog/csp3/provider-reset-verification.json).
The checks cover 1,041 race events and 3,617 normal events. They provide no primary acceptance credit.


### CSP3 metadata provider selection

The provider selection contract now accepts models.dev HTTP and Git observations.
The first new contract test failed because selection accepted only provider API observations.
Repository review also found that metadata baseline filtering replaced the catalog while retaining the original observation identity and checksum.

Reconciliation now builds a separate provider view after receipt validation. It applies selection before canonical alias mapping.
The original observation remains available for receipts and shared authored definitions. A filtered view cannot replace that evidence.
Tests cover baseline enrichment, peer providers, aliases, empty selections, original receipt checks, and protected source refusal.
General source reset records, projection membership, previews, and CLI/HTTP adoption remain open.

The [metadata selection proof](../../plans/proof/starport-production-catalog/csp3/metadata-selection-verification.json) records 1,055 passing race events and 3,631 normal events.
All 50 primary cases remain UNVERIFIED.


### CSP3 local developer integration

The working branch connects CLI and HTTP acquisition to retained runtime publication.
Metadata reset scopes now cover models.dev HTTP and Git. Manual history version 3 stores these scopes while accepting earlier provider reset records.
The production pipeline previously treated an explicit reset as a volume collapse. Fresh mode now accepts complete replacement omissions and preserves its baseline.
Normal refresh and strict publication keep their existing health checks.

Actual binaries verified offline Starport startup, CLI acquisition, Starmap server refresh, HTTP reset, and Starport activation.
The deterministic provider changed the GPT-4o mini context limit to 9,999 tokens. Reset restored the embedded limit of 128,000 tokens.
Starport retained the accepted generation and payload checksum after restart.
These results use a temporary Go workspace with the candidate Starmap package. The published Starport module pin remains unchanged.

Starport's old acquisition test fixture omitted the source receipt required by the candidate runtime.
The fixture now creates a validated original observation and provider layer. The focused pair test passes.
The [local developer milestone](../../plans/proof/starport-production-catalog/local-developer-flow.md) owns commands, failures, and remaining checks.
All 50 primary cases remain UNVERIFIED. Native qualification, full production integration, review, and release gates remain open.


## CSP3 reset projection verification

Commit `81b666a6` adds runtime tests without changing production behavior. Ten focused race test events pass through reset and restart.
The tests cover acquired-only offering removal, baseline provider membership, explicit zero limits, unknown and missing limits, and changed operator values.
The [proof](../../plans/proof/starport-production-catalog/csp3/reset-projection.md) records exact checks, digests, and unsuccessful fixture attempts.

The source observation contract still has no explicit tombstone field. Reset clears retained acquisition while preserving the selected baseline.
It does not establish source-scoped deletion authority or authorize lower-layer membership suppression. CSP3 must implement and verify that separate contract.
No primary acceptance case gains credit.


## CSP3 fresh CLI correction

Commit `4df74524` corrects two mismatches between the accepted reset contract and the CLI.
The CLI lacked `--fresh`. Its old confirmation stated that force mode deleted all model files and also prevented a declined dry run from starting.
The command now accepts `--fresh`, retains `--force` and `-f`, and asks once after the preview. Dry runs ask no confirmation.
Reset-only output now reports acquisition work even when model values remain equal.

The [proof](../../plans/proof/starport-production-catalog/csp3/fresh-cli.md) records 59 focused race test events and 285 broader CLI race events.
The built binary passed preview and apply against a local provider fixture. The preview left 1,275 workspace files unchanged.

The deletion investigation also identified a boundary that CSP3 must preserve.
Starport's `internal/providers/state/store.go` projects routing state by provider and model, without a Starmap acquisition binding.
Its `internal/providers/keyring/keys.go` uses Starport account identity for credential storage. That identity does not establish an upstream provider account.
Account-specific catalog removals must not become global offering withdrawals through either representation. Scope-aware routing enforcement remains unverified.


## CSP3.1 offering lookup correction

Commit `70f01b6a` replaces full-provider copies during offering lookup with an immutable identity index.
Canonical and alias lookups retain their errors and caller-owned values. Concurrent mutation tests preserve retained catalog values.

The [proof](../../plans/proof/starport-production-catalog/csp3.1.md) records the allocation regression, benchmark samples, 843 catalog race events, and both assigned A44 checks.
At 10,000 models, canonical lookup changed from 17,398,242 bytes and 160,047 allocations to 1,104 bytes and 11 allocations.
Full Starport request overhead remains outside this measurement. CSP10.1 owns its remaining candidate preparation and lookup work.


## CSP0.2 installation review

The advertised Compose recipe failed before readiness on native Linux ARM64. The local admin token required rotation before a network bind.
The recipe also lacked volumes for Starport's relational, file, and catalog state.
Starport commit `ee3f2e1` retains both Starport directories, selects a durable catalog path, and adds the rotation command.
The corrected run preserves the gateway key and a SQLite account template through container replacement.

E02 now passes for the reviewed README with 11 native installation entries and the prior real streamed inference.
The [first-use proof](../../plans/proof/starport-production-catalog/csp0.2.md) records archive, Homebrew, source, container, and Compose scope.
This correction supports one Compose gateway. Shared PostgreSQL, replicated operation, and disaster recovery remain separate qualification work.
The full operator guide retains 48 existing prose diagnostics. The README and changed container procedure pass their prose checks.


## CSP0.3 first-use demonstration

Starport commit `02efa34` replaces the README's automatic console-tour animation with a static preview linked to a first-use demonstration.
The 38-second GIF shows the actual v1.2.0 archive, catalog access before credentials, provider setup, and real streamed inference.
It preserves provider response timing and discloses the shorter credential-entry wait. The transcript and uncut capture remain available.

E03 passes with source-bound media evidence. E02 passes again after the README header change.
The [media proof](../../plans/proof/starport-production-catalog/csp0.3.md) records dimensions, bytes, reading time, local browser checks, and cleanup.
This recording qualifies the early demonstration only. CSP24 still owns the final released-pair recording.


## CSP2 service-managed primary configuration

Commit `707d63db` implements D23 for the Starmap primary file. Operators select an explicit file path and service-managed access.
The default remains owner-only. Dotenv files and catalog state keep their private-access requirements.
Migration rereads and file diagnostics use the selected primary policy.

The [service configuration proof](../../plans/proof/starport-production-catalog/csp2/service-configuration.md) records 394 application and file-access race results.
The final focused checks pass 42 results, and shared callers pass another 686 results.
An unprivileged native Linux child reads root-owned `0640` configuration and rejects unreadable, writable, and foreign-owned fixtures.
Windows compilation passes. Native Windows ownership and ACL qualification remain UNVERIFIED.

Code lint, Ago, all six consumer compositions, workflow parsing, and maintained prose pass.
The implementation adds no library dependencies. Required pre-PR review remains pending.
CSP8 owns Starport adoption. All primary acceptance cases retain their existing status.


## CSP2 Windows service ownership and denied reads

Commit `fb54f39e` adds the missing native administrator-owner and denied-read fixture.
It also checks access recovery and diagnostics that distinguish a compatible descriptor from proven effective access.

The [fixture record](../../plans/proof/starport-production-catalog/csp2/windows-service-qualification.md) preserves AMD64 and ARM64 compilation and static checks.
Native execution remains UNVERIFIED. GitHub Actions must fail if it cannot exercise administrator ownership.
This adds no product behavior and gives no primary acceptance credit.


## Starport integration preparation and owner decision audit

Starport commit `16e9dd2` records the remaining catalog integration tests and corrected runtime-identity comments.
The [local pair check](../../plans/proof/starport-production-catalog/csp2/starport-integration.md) passes 134 race results with package lint and changed-file prose checks.
The published Starmap module pin remains unchanged. Full publication gates and released-pair qualification remain open.

The [owner decision audit](../../plans/proof/starport-production-catalog/csp2/owner-blocked-audit.md) records the remaining prerequisites after three goal turns.
No other `todo` task is eligible under the current dependency graph. The complete objective remains unfulfilled.


## Owner approvals and execution resume

The owner approved the shared review-helper patch and explicit account-scope links on 2026-09-07.
The [approval record](../../plans/proof/starport-production-catalog/csp2/owner-approvals-2026-09-07.md) supersedes the earlier decision blocker.
The helper now uses 300,000-byte prompts. Syntax validation and deterministic self-tests pass.
D25 records the scope requirement. Native qualification, required review, scoped-removal implementation, and released-pair acceptance remain open.


## Publication checks after owner approval

The complete Starmap code bundle exceeds the helper's eight-pass limit after the approved prompt change.
The [two-part review stack](../../plans/proof/starport-production-catalog/csp2/review-stack-2026-09-07.md) preserves every source change and executable verifier.
Both complete diffs pass prompt preflight. Actual model review remains required.

Starport commit `0acf476` corrects direct runtime test fixtures that supplied non-private state directories.
The [fixture evidence](../../plans/proof/starport-production-catalog/csp2/app-private-fixture.md) records five passing race results and the initial failures.
All 12 ownership checks now pass. Production permission enforcement remains unchanged.

## Discovery and readiness audit: 2026-09-07

The independent audit confirms several product gaps against Starport `de0e00297428f16115a068f5a39ec6702e0674f5` and its Starmap v0.16.5 dependency.
Candidate Starmap evidence uses `9693899b`.
[Verification and retained probes](../../plans/proof/starport-production-catalog/discovery-readiness-2026-09-07/verification.json) record source hashes, outputs, and test-development failures.
No primary acceptance status changes.

| Finding | Verified behavior | Existing owner |
| --- | --- | --- |
| DR01 | Production provider activation needs no operator credential material for structural registration. The fixture exposes 511 definitions, 14 providers, and 614 structural routes. | CSP9, CSP10 |
| DR02 | Proxy model lists and catalog views derive membership from structural routes. They do not provide complete permitted catalog discovery. | CSP10, CSP17 |
| DR03 | The pinned runtime uses enrichment for provider layers. Candidate Starmap uses canonical reconciliation with explicit authority order. | CSP3, CSP8 |
| DR04 | Pinned pricing merges retain USD output 4 under a new EUR input 2. A separate fresh-baseline test retains input 1 after explicit zero. | CSP3, CSP8 |
| DR05 | Pinned enrichment drops a valid linked model without pricing or limits. The candidate retains it. | CSP3, CSP8 |
| DR06 | The production cache returns stale model and provider lists after adapter removal without a catalog generation change. | CSP10, CSP12.1 |
| DR07 | Provider facet counts use the authored ID prefix. The filter also accepts offering providers, so counts and selection disagree. | CSP17 |
| DR08 | Chat defaults can select a model without usable operator providers. Model actions do not establish current caller readiness. | CSP9, CSP10, CSP17 |
| DR09 | The committed pin cannot compile `internal/catalog`: `runtime.NewProviderLayer` is undefined. `internal/catalog/view` passes separately. | CSP8 |

DR01 preserves existing useful behavior. Its fixture counts do not establish caller readiness or HTTP compatibility.
The operator provider-status store already provides broader diagnostics. DR02 does not claim that every console surface lacks diagnostic information.
Candidate pricing and membership tests pass 19 race test events. These fixes do not reach Starport through its current pin.

DR06 uses the real local model cache and production proxy wrapper. Its unused response store uses the repository mock.
The probe records two passing and three failing test events, including the parent failure.
Models and providers remain stale. Endpoint results stay current because the typed cache read rejects the stored JSON object and calls the service again.

The probe confirms that the endpoint entry exists. Repairing its decoder also requires correct dependency identity.
This evidence establishes discovery defects, not an inference authorization bypass.

The TypeScript probe imports the actual filter and default-selection functions.
A Meta-authored model offered by Groq and DeepInfra counts under Meta, while both Meta and Groq filters accept it.
The same fixture becomes the default when the usable-provider set is empty.
Browser behavior, accessibility, caller isolation, replica behavior, and released-pair compatibility remain unverified for this audit.

The plan maps these findings to existing tasks and acceptance subcases.
The target separates accepted membership, permitted discovery, structural support, and current caller readiness.
D24 preserves the selected baseline during fresh acquisition. D25 limits account-specific withdrawals to explicitly linked inference profiles.
Neither decision permits public fallback around internal authority.


## Catalog encoding and restart limits, 2026-09-08

An upgrade probe used the current runtime to publish a 23,683,266-byte generation into the filesystem catalog store.
Both the original reader and the legacy-recovery candidate refused to reopen it. The decoder enforced 16 MiB, while encoding imposed no byte limit.
The same mismatch also permitted JSON nesting beyond the decoder's 64-level limit.

A first repair enforced the existing 16 MiB limit during encoding. Four existing acquisition tests then failed, including ordinary tenant publication.
That repair prevented unreadable writes but rejected supported catalog composition. It is not the selected implementation.

The selected candidate separates canonical catalog capacity from the raw source limit. D26 assigns 32 MiB to catalog payloads and retains 16 MiB for raw source JSON.
Both codec directions now enforce the catalog limit and the shared nesting bound. The 32 MiB candidate reopens the recorded generation without changing its digest.
Layer envelopes, cumulative history bounds, and embedded review budgets remain independent.

The legacy-recovery prototype remains unapplied. It passes independent-evidence selection, private-fact refusal, embedded publication, and restart checks.
After the byte-limit repair permits decoding, an older accepted legacy history still fails the new candidate's startup validation.
Checked startup recovery remains the next legacy boundary. No production qualification follows from the isolated prototype checks.

The current manifest compatibility range identifies schema versions only. It does not declare reader capacity.
Older 16 MiB readers still reject larger payloads with a matching schema version. Released-pair and downgrade qualification remain open.

## Provider binding removal during startup, 2026-09-08

The startup probe found two failures on source ad2c09e3.
Without retained inputs or a binding option, the runtime served a stored provider with withdrawn scope.
With retained inputs, the effective catalog removed that provider, but the client and durable head still selected the prior generation.
The HTTP provider endpoint returned 200 instead of 404.

The cause spans startup selection and publication.
`initializeEffective` trusted an empty layer set when the binding option was absent.
`publishBindingStartup` skipped publication under that same condition.
The candidate inspects stored provenance scope and applies binding removal before serving.
It preserves unscoped store-only startup and explicit empty-set recovery.

An independent design probe rejects generation identity as the sole legacy recovery check.
Its identity comparison detects changed binding declarations and baseline generation IDs.
It cannot detect changed baseline bytes or missing original history when it reuses manifest evidence.
That probe describes an unapplied recovery candidate, not a new generation-ID contract.
Legacy startup recovery still needs independent continuity evidence.

## SDK credential-chain checks, 2026-09-08

The earlier secret-source tests injected SDK reads and did not exercise the default credential factories.
New tests call both production factories and route their real SDK requests through local TLS fixtures.
AWS checks cover environment credentials, shared files, environment precedence, web identity, missing credentials, and denied secret reads.
Azure checks cover environment credentials, workload identity, and denied secret reads.

The full authentication suite passes 78 test events and one package result for both existing and proposed SDK dependencies.
The updated dependencies pass the same suite on Go 1.25.12.
All runs use race detection and report zero failures or skips.
The factory files remain unchanged, so these tests require no production behavior repair.

The [credential-chain proof](../../plans/proof/starport-production-catalog/pr-audit-2026-09-08/sdk-credential-chains/README.md) records exact sources, toolchains, and coverage limits.
The first Azure fixture incorrectly excluded three standard MSAL scopes. The corrected fixture retains the vault audience and checks the complete scope set.
Live cloud IAM policy, SSO, device login, and managed identity services remain outside this evidence.

## Dependency qualification and MySQL test isolation, 2026-09-08

Starport PR #368 passes the listed dependency checks against current main.
The [dependency evidence](../../plans/proof/starport-production-catalog/pr-audit-2026-09-08/starport-dependencies/README.md) records exact source trees and results.

The shared MySQL contract fixture deletes migration history without removing every migrated table.
Its second migration repeats `ADD COLUMN request_id` and fails on both dependency versions.
Separate disposable databases pass migration, read/write, and rollback checks.
CSP15 must repair the fixture isolation before it claims complete shared-backend qualification.
No production SQL change belongs to this dependency update.

## Scoped membership and acquisition removal, 2026-09-10

Local Starmap commit `251b1431` carries schema-2 bindings and schema-7 membership records.
The [scope evidence](../../plans/proof/starport-production-catalog/csp3/scoped-membership-2026-09-09/verification.json) records 1,455 passing runtime, acquisition, and reconciler test events.
HTTPS trust checks, publisher isolation, receipt retention, restart, and migration also pass focused checks.
This branch remains unpublished. Starport profile links and released-pair qualification remain incomplete.

The acquisition volume guard still marks omitted attributed models as partial, degraded evidence.
The owning function is `guardObservationVolume` in `internal/catalog/pipeline/observation_health.go`.
It applies this rule to bound observations as well as unscoped observations.
Thus reconciliation and transport support do not prove that ordinary acquisition can apply authorized removals.

CSP3 must connect explicit replacement authority to the operator review policy at this boundary.
D27 later rejected an automatic removal threshold. Internal authoritative withdrawals retain their separate immediate enforcement path.
D30 later selected alias retention without automatic expiry. Alias implementation and cross-authority qualification remain incomplete.

## Portable artifact membership evidence, 2026-09-10

Verified artifacts can contain membership scopes.
`acquisition.ImportRelease` previously dropped incoming scopes during fact reconciliation and omitted original provider receipts from publication evidence.
The regression tests reproduce this loss for scope-only imports and imports that also add model facts.

The local repair retains independent incoming and current scopes with their original receipts.
Conflicting records for one publisher and binding revision fail before publication.
Repeated identical imports do not publish another generation.
This low-authority merge does not select replacement authority. Configured source selection and explicit trusted activation own replacement.

The [scope proof](../../plans/proof/starport-production-catalog/csp3/scoped-membership-2026-09-09/verification.json) records eight passing focused events on each Go toolchain.
Commit `8a4c14d9` also passes all 100 acquisition test events. Pre-PR review is active. The repair has no merge or task completion credit.

## Observed availability preserves discovery

On 2026-09-10, the owner rejected automatic deletion when ordinary provider acquisition stops reporting a model.
D27 through D29 require visible catalog entries, scoped automatic-routing exclusion, and explicit removal limited to the affected entry by default.
A replacement Starmap baseline can remove entries. Internal permission withdrawal remains a separate immediate enforcement rule.

The prior local branch filtered canonical offerings through `MembershipState.Permits` after a complete public provider inventory.
It also pruned their provenance and review candidates. That behavior conflicts with the new decisions.
New tests reproduce disappearance during reconciliation, partial refresh, baseline refresh, and restart.
The repair preserves catalog facts and carries absence only in scope inventory evidence.

The prior Sol and Opus review passed for commit `8a4c14d9`. It does not qualify the revised behavior.
The scoped branch remains unpublished. Pipeline integration, explicit removal, Starport routing, full qualification, and fresh review remain required.


## Canonical rename alias retention, 2026-09-10

The owner selected D30: retain old model IDs as aliases until explicit operator removal or replacement baseline removal.
This decision supersedes the proposed 30-day expiry in specification section 2.1.
Provider omission must not remove an alias. Current authority and provider/account availability still govern requests through it.

The [decision proof](../../plans/proof/starport-production-catalog/csp3/alias-retention-decision-2026-09-10.md) assigns time, restart, removal, and authority checks to existing A08 coverage.
The final local A08 report passes 38 registered commands and 745 events, but alias and scoped-removal subcases remain unverified.
The decision adds no implementation or merge credit.

## Canonical removals across renames, 2026-09-10

The alias candidate at `3609f171` retains an operator removal under the original canonical ID.
Its effective removal index does not follow that ID to the renamed definition.
A consumer that resolves the alias before checking removal can therefore miss the exclusion.
The catalog and runtime regressions reproduce this behavior.

Repair `13fe3a75` indexes retained removal targets through the current alias history during catalog construction.
It preserves the original serialized operator target for audit and explicit restore.
Removed alias edges still preserve model identity.
The query allocates no memory and does not traverse rename chains.

The [rename exclusion proof](../../plans/proof/starport-production-catalog/csp3/canonical-removal-rename-2026-09-10/verification.json) records 1,437 passing catalog events and both focused Go toolchains.
The runtime regression covers rename, restart, explicit restore, and another restart.
Full task qualification, review, native CI, and merge remain open.
CSP10 must preserve this identity during Starport admission.

## Merged native qualification reconciliation, 2026-09-10

CSP2 merged through Starmap PR #136 at `7be0a02701447ef0f3f044bbbd5cfd9ea5524f9e` on 2026-09-09 UTC.
The [merged qualification](../../plans/proof/starport-production-catalog/csp2/merged-qualification-2026-09-09/verification.json) passes all 22 assigned subcases and fifteen CI checks.
Six native jobs cover Linux, macOS, and Windows on AMD64 and ARM64.
These results supersede the earlier native-execution gaps for the assigned component tests.

The engineering specification now distinguishes those completed checks from power-loss durability, abandoned-stage cleanup, and complete service procedures.
The historical entries above retain their original results.
Starport composition and released-pair qualification remain open.

## Combined alias and removal qualification, 2026-09-10

The preceding alias findings describe earlier source revisions.
Combined source `b5af1e9c` retains aliases without expiry and preserves scoped and canonical operator removals across refresh and restart.
Its complete tree matches `b3af02c3`, which passed full repository verification.
Both required reviewers report zero findings against main `b9eca889`.

The [task proof](../../plans/proof/starport-production-catalog/csp3/canonical-removal-rename-2026-09-10/verification.json) records ten passing CSP3 subcases, 111 commands, and 907 test events.
Full catalog tests pass 1,437 events. Runtime, acquisition, and reconciler race tests pass 1,503 events.
All 81 packages pass each full normal and race suite. All fifteen coverage thresholds pass.

PR #145 carries this reviewed source. Native CI and merge remain necessary before CSP3 completion.
Starmap exposes alias identity and removal state, while later Starport tasks must enforce both during discovery, pricing, and routing.
The alias lookup does not grant permission or establish authenticated enterprise authority.
CSP4 owns retained authority, permission envelopes, and the refusal conditions recorded in its [preparation](../../plans/proof/starport-production-catalog/csp4-preparation-2026-09-10.md).

## CSP3 merged qualification, 2026-09-10

PR #145 merged at `a87262e3b830f285eee2a82ebd85e3816d7d94c5`.
All fifteen checks pass, including six native platform jobs.
The [merged proof](../../plans/proof/starport-production-catalog/csp3/merged-qualification-2026-09-10/verification.json) confirms exact tree equality with the reviewed and qualified source.
CSP3 is complete. D27 through D30 remain the accepted absence, removal, and alias contracts.

CSP4 continues with independent permission envelopes, transport, and committed authority bindings.
Its runtime state and source adapter remain local work.
Starport integration and released-pair acceptance remain incomplete.

## Runtime authority and client publication, 2026-09-10

Local Starmap `ea4f7f11` connects configured authority identities, independent permission refresh, private retention, startup, and readiness.
The [runtime proof](../../plans/proof/starport-production-catalog/csp4/runtime-integration-2026-09-10/verification.json) records 29 passing test events and two package outcomes on both supported toolchains.
Static checks pass. CSP4 remains in progress.

The audit found that direct client mutations could bypass runtime authority and add incompatible alias history.
Cumulative publication guards now reserve update, activation, and rollback for the owning runtime.
The offline API inventory includes the passive guard. Construction and reads never invoke it.
The existing alias-history validation remains intact.

The audit also found that ordinary receipt renewal interrupted a valid confirmed lease.
The runtime now preserves that lease until its original expiry while it retains the renewal.
A manifest cannot restore a rejected receipt. Only a valid receipt can reopen receipt confirmation.
These checks use cached state and clock evidence. They do not qualify complete gateway latency.

An intermediate broad run passed 1,012 test events and failed three settings checks.
The complete settings repair suite passes 98 test events and two package outcomes.
The proof preserves those failures, the earlier root API inventory failure, and two command syntax errors that ran no tests.

Publisher receipt issuance, clock qualification, shared-store follower activation, and exact authority bindings through the serving store still need verification.
Full CSP4 acceptance, required review, native CI, and merge remain incomplete.

## Authority generation preservation, 2026-09-10

The next audit found that runtime publication rebuilt an ordinary manifest and discarded the authority head.
Local `a9b369dc` activates the complete original generation instead.
The [binding proof](../../plans/proof/starport-production-catalog/csp4/generation-binding-2026-09-10/verification.json) records the failure and 27 passing authority test events on each toolchain.
The regression compares the entire generation after refresh and retained restart.

An unchanged generation requires exact manifest and payload equality before the runtime skips a store write.
This preserves diagnostics after a write outage without accepting a different manifest under the same generation ID.
The full root race suite passes 105 test events on unchanged root source.
The full runtime, remote, and artifact race suite remains running. CSP4 remains incomplete.

## Authority head snapshot, 2026-09-10

Local `671b54d5` exposes `CurrentAuthorityHead` without catalog-store reads or payload copies.
The [head proof](../../plans/proof/starport-production-catalog/csp4/authority-head-2026-09-10/verification.json) records zero storage reads and zero allocations in the focused test.
The final root/runtime check passes 17 test events and two package outcomes. The minimum-toolchain check passes one test and one package outcome.
Static checks pass across 1,402 prose files with zero diagnostics.

The snapshot follows construction, activation, ordinary publication, and rollback. Failed publication preserves the previous head.
It does not authenticate the publisher, prove fleet freshness, or renew permission.
Receipt issuance and complete CSP4 qualification remain open.

The latest full root suite passes 106 test events and one package outcome at `671b54d5`.
The complete runtime, remote, and artifact suite passes 864 test events and three package outcomes at predecessor `a9b369dc`.
Both runs finish without failed or skipped test events. The head snapshot proof preserves their exact logs and source revisions.

## Permission relay and wire schema, 2026-09-10

Local `cc4e2a01` adds the server permission route and a runtime reader of confirmed upstream receipts.
The [relay proof](../../plans/proof/starport-production-catalog/csp4/relay-2026-09-10/verification.json) records the exact 17-file source change.
The runtime read uses cached state, allocates no memory in the focused check, and starts no upstream read.
It preserves the original receipt when the catalog or permission schema cannot activate.

The final focused Go 1.25.12 run passes 21 test events. The server suite passes 329 events before the later schema correction.
The complete schema-normalizer suite passes 63 events. Static and external-consumer checks pass.
The cold-fixture repair controls upstream availability and waits for the initial scheduled read. It preserves the cold-refusal assertion.

The pinned Swag generator placed a string schema under the vendor media type and the object under `application/json`.
The build normalizer now emits the exact vendor JSON object contract and refuses unexpected generator output.
The wire-schema regression fails before the correction and passes afterward.

The complete runtime suite remains running. Origin issuance, standalone clock qualification, shared-store follower recovery, and product acceptance remain open.

The new TLS regression confirms a CSP4 defect: a trusted withdrawal manifest leaves old admission active during a stalled payload transfer.
The runtime also reports the previous required revision. The final test captures both assertions and completes cleanup.
Current `runtime.readSource` observes authority after a successful `Source.Read`.
Propagate the requirement before payload processing and rerun qualification before publication. This repair requires no owner decision.

## Requirement arrival before payload processing, 2026-09-10

Local `5155c9f7` repairs the preceding TLS regression.
The [manifest-arrival proof](../../plans/proof/starport-production-catalog/csp4/manifest-arrival-2026-09-10/verification.json) records the exact 16-file change.
A current manifest reaches runtime permission state after transport trust and manifest validation, before payload processing.
A stalled transfer or unsupported payload cannot delay a known withdrawal.
Historical addressed reads do not advance permission requirements. Invalid publisher trust never reaches the observer.

The binding is single-owner and must precede manifest reads. Shutdown joins active callbacks and refuses late callbacks.

Both supported toolchains pass 40 focused race events and two package outcomes.
The complete protocol suite passes 60 events. Static checks and all six external consumer checks pass.
The preceding relay runtime suite completed 875 events. Current broad runtime qualification continues against the new observer behavior.
Origin receipt issuance, qualified clocks, shared-store follower recovery, authority transitions, and full product acceptance remain incomplete.

## Issuer storage consistency, 2026-09-10

The source at `44a400a4` has no production caller of `Client.CurrentAuthorityHead`.
Its focused tests prove local snapshot behavior. They do not qualify permission receipt issuance.
The runtime relay reads confirmed upstream receipts from its own retained permission state.

The current `storage.ObjectBackend` contract requires conditional writes and exact object-version tokens.
It does not require current reads. `storage.Object.Current` reads a pointer, then loads the selected manifest and payload.
The filesystem store also resolves its pointer through the complete generation.
The caller-owned S3 adapter does not establish a freshness guarantee for every compatible endpoint or transport.

The [issuer storage inspection](../../plans/proof/starport-production-catalog/csp4/issuer-storage-inspection-2026-09-10.md) defines the missing read guarantee and required qualification cases.
Receipt validity must start before the read. Metadata migration must cover existing generations and repeated identical commits.
These findings concern the second CSP4 delivery. No stored format, interface, or implementation changed in this inspection.

## Independent authority storage, 2026-09-10

Local Starmap commit `2b4310d1d24fa22279993417b4e92f55af7aee29` adds optional current-head observations to memory, filesystem, and qualified object stores.
An immutable version-1 `authority.json` record binds the head independently of catalog manifests and payloads.
Pointer promotion follows record persistence. Explicit identical commits repair missing legacy metadata, while reads remain passive.

The legacy relocation code previously required exactly two generation files.
Its new regression failed on the third authority record before the repair.
Relocation now validates the record against the complete generation and preserves it. Recordless legacy generations remain recordless.

The [storage proof](../../plans/proof/starport-production-catalog/csp4/authority-record-2026-09-10/verification.json) records 142 storage/private-file race events and seven migration events on both toolchains.
All six external consumers pass on Go 1.25.12. Ago and prose checks pass.
This checkpoint is local and unmerged. Origin issuance, clock qualification, shared followers, and Starport consumer checks remain incomplete.

## Library permission issuer, 2026-09-10

Local Starmap commit `48205c51886c084c81b65f06ce57fe9389036dd8` adds `permission.NewIssuer`.
It starts a bounded receipt interval before a current-head storage read and accounts for issuer uncertainty.
Concurrent older replies cannot replace a newer observed head. Storage failures do not renew receipts.
A new regression found a panic for an uninitialized issuer. The API now returns a typed error for invalid receivers and contexts.

The [issuer proof](../../plans/proof/starport-production-catalog/csp4/issuer-library-2026-09-10/verification.json) records 33 issuer events and 21 runtime identity events on each supported toolchain.
All six external consumers and the 1,430-file prose check pass. Lint and ago pass.
The complete catalog and storage suites pass 1,240 and 88 events respectively.
The broad runtime run reaches its default ten-minute timeout after 337 passing events.
Its retry uses the planned thirty-minute limit and remains unverified.

The issuer retains its highest observation only in process memory.
Durable publisher ordering, issuer authorization, production clock adapters, server wiring, and shared follower activation remain incomplete.
This checkpoint is local and unmerged. All eight CSP4 acceptance subcases remain UNVERIFIED.

## Durable authority publication order, 2026-09-10

Local commit `4031c084ef18a4f6e272eb77c7fa897c1cc4c409` adds the explicit `permission.Publisher` store wrapper.
The failing regression shows that generic storage accepts an older authority sequence after reopen.
Generic storage does not claim to enforce that policy. The wrapper adds the authority-specific contract without changing ordinary storage behavior.

Publication checks the durable predecessor and then uses the store's atomic compare-and-swap.
A delayed writer cannot replace a newer committed generation. Exact retries still succeed, while changed immutable bytes fail.
An existing ordinary store requires explicit bootstrap. An established authority cannot use bootstrap to replace its policy.
The publisher rejects unsupported permission semantics while its independent head remains observable to receipt consumers.

The [publication-order proof](../../plans/proof/starport-production-catalog/csp4/publisher-order-2026-09-10/verification.json) records 46 issuer and publisher race events on each toolchain.
Lint, ago, and the 1,434-file prose check pass. The full documentation check finds five stale runtime source links from the earlier identity extraction.

Commit `60f7b9cd` corrects those generated links. The complete documentation check then passes.
The runtime suite passes 813 race events in 1,286.36 seconds with no failures or skips.
Origin revision construction, clocks, server wiring, shared followers, authority transitions, and Starport qualification remain incomplete.

## Origin generation construction, 2026-09-10

Local commit `99ad3cfb8f7804e15f480096909f0dcdc10d380d` adds `permission.PrepareGeneration`.
It derives required permission revisions from complete catalog semantics under explicit authority and policy identities.
The source catalog must already reflect the selected policy. Preparation refuses existing authority generations and preserves exact ordinary payload bytes.

The [origin proof](../../plans/proof/starport-production-catalog/csp4/origin-generation-2026-09-10/verification.json) records 80 race-test events on each supported toolchain.
Its 34 new events cover withdrawal, scope changes, alias retention, explicit removal, source validation, identities, and independent copies.

Lint, ago, complete documentation, and the 1,427-file prose check pass. The proof retains the missing-API failure and the fixture error.

The revision includes all semantic catalog facts, including scope evidence. Metadata-only changes can therefore require a new enforced revision.
Provenance and manifest observation metadata do not affect it. These hashes run during publication, outside inference admission.

Origin authorization, durable sequence selection, clocks, server wiring, shared followers, transitions, and consumer qualification remain open.
All eight CSP4 acceptance checks remain UNVERIFIED. The work remains local, with no PR or merge.

## Origin durable sequence selection, 2026-09-10

Local commit `8a71193bc7587a025231be0d72f2002327fd67be` adds `Publisher.PublishCatalog` and `BootstrapCatalog`.
The publisher derives the sequence from durable state and uses the existing conditional commit contract.
An exact retry keeps its identity after reopen. A stale proposal cannot replace a committed withdrawal.

The [publication proof](../../plans/proof/starport-production-catalog/csp4/origin-publication-2026-09-10/verification.json) records 89 race-test events per supported toolchain.
Nine new events cover restart, bootstrap, identity, unsupported semantics, exhaustion, unreadable state, invalid calls, and concurrent writers.
Lint, ago, complete documentation, and the 1,442-file prose check pass.

The [native clock survey](../../plans/proof/starport-production-catalog/csp4/native-clock-survey-2026-09-10.md) records a successful read-only macOS clock query.
It selects no adapter and gives no production qualification.
Origin authorization, serving-client activation, server configuration, clocks, followers, transitions, and Starport consumers remain incomplete.
All eight CSP4 checks remain UNVERIFIED.


## Origin runtime transaction integration, 2026-09-10

Local commit `717ba9148a75ad74ca1758aa51cbb121b0f5fd18` connects authority preparation, the retained-input journal, guarded activation, and receipt issuance.
The [qualification record](../../plans/proof/starport-production-catalog/csp4/origin-runtime-2026-09-10/verification.json) lists current commands and failures.
The journal names the final authority generation before catalog storage changes. A lost commit reply no longer leaves recovery bound to an ordinary source identity.

The runtime preserves an unchanged accepted authority after restart and filesystem reopen.
It requires explicit bootstrap for an ordinary store and rejects a changed authority identity.
A regression test exposed a bootstrap write before caller-supplied publication guards. Bootstrap now executes inside the guarded client commit.

Both supported Go toolchains pass all seven origin runtime cases after that correction.
The earlier focused run also passes six root preparation events and 44 origin library events on Go 1.25.
Final transaction regression passes 36 events. Completed root and permission suites pass 112 and 90 events.
Lint, ago, complete documentation, and the 1,451-file prose check pass.

The intermediate runtime suite passes 820 events. Its source predates the alias-validation extraction and guarded-bootstrap repair.
These checks do not complete the standalone production recipe or any of the eight CSP4 acceptance cases.

The origin option selects the sole publication store, including when client options name another store.
Its complete effective catalog defines the permitted catalog. The deployment still controls mutation authorization and supplies qualified clock evidence.
Canonical server settings, native clock qualification, shared-store followers, authority transitions, and Starport consumer qualification remain open.
