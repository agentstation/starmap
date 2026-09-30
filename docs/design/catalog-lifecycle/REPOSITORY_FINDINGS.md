# Catalog lifecycle repository findings

## Apple silicon support decision, September 26

D39 removes Intel Mac support from both products. The previous release matrices included six targets, including `darwin/amd64`.
The revised matrices retain five targets. Linux and Windows retain both x86-64 and ARM64.
The implementation changes CI, release verification, native evidence selection, Homebrew requirements, and publisher promotion checks.

Historical six-target qualification remains evidence for its recorded commits. It does not require future Intel Mac runs.
The current plan records pending review, CI, and merge evidence.


## Shared runtime and diagnostics checkpoint, September 26

Consumer `ee563ce61` connects fleet publication to the gateway runtime and its accepted-store diagnostics.
Shared startup requires an approved PostgreSQL recovery record and the selected Valkey process identity.
A read-only probe retains identity checks and cannot mutate the shared store.

Producer `4cd0db603` revalidates retained startup under the new owner grant.
The [checkpoint](../../plans/proof/starport-production-catalog/csp11/consumer-runtime-2026-09-26/verification.json) records focused passing tests and review.
Shared migration handling, acquisition-capability preflight, concurrent gateway qualification, and both merges remain open.


## Native fleet adapter checkpoint, September 26

Starport commit `01ca416dc` adds native Valkey expiry checks, process identity checks, shared recovery inputs, and atomic route acceptance.
PostgreSQL holds the independent recovery approval. The gateway runtime and diagnostics do not use this adapter yet.

Starmap commit `d56e6e73c` repairs unchanged catalog publication after owner takeover. A new grant gets a new publication revision without changing catalog content.
The native job also exposed a renewal test that expected the superseded epoch-change behavior. The test now requires refusal and unchanged catalog state.

The [checkpoint](../../plans/proof/starport-production-catalog/csp11/native-adapter-2026-09-26/verification.json) retains passing real-backend tests and the failure evidence.
It excludes the subprocess helper from case counts. Full CSP11 qualification and both merges remain open.

The current repositories implement most catalog distribution infrastructure.
They do not yet satisfy every requested bootstrap, directory, credential, and
authority behavior. The [PRD](PRD.md) defines the product requirements.
The [engineering specification](ENGINEERING_SPEC.md) defines the proposed changes.

This report records source inspection and selected local tests on 2026-09-04.
It does not certify production availability or full API compatibility.
Dated entries retain their original qualification limits. The canonical plan owns current task status.

## September 26 runtime integration checkpoint

Producer commit `eb64a9d57` connects the fleet contract to runtime startup and publication.
Followers recover the selected inputs before takeover and preserve pin acceptance after local directory loss.
A requested pin that the refresh owner has not accepted causes a follower startup conflict.
Recovered catalog data does not supply authority permission or undo a known withdrawal.

The [runtime checkpoint](../../plans/proof/starport-production-catalog/csp11/runtime-integration-2026-09-26/verification.json) records 64 passing race events across 56 leaf cases.
Refinement `dd348601a` passes 134 race events across 117 leaf cases and clears seven lint findings.
The broad local run timed out after 900.683 seconds with 536 passing events and no named failure events.
It remains incomplete. Vet, goago, pure-Go checks, and source writing checks pass. Sol and Opus report no pre-PR findings.

The adapter in these tests runs in one process. Starport still needs real storage, recovery-witness, and acquisition-capability qualification.
CSP11 remains in progress.

## Fleet contract review: 2026-09-26

CSP11 remains in progress. The [contract audit](../../plans/proof/starport-production-catalog/csp11/contract-audit-2026-09-26.md) identifies two confirmed shared-publication defects.
The real Valkey probe accepts a stale writer after expiry, release, or separate-process takeover.
The filesystem recovery probe loses an earlier model after removing the former leader directory and applying a partial update.

The producer work also found unsafe renewal behavior.
A renewal could replace the original grant or overwrite newer local ownership state.
The prepared lease repair rejects changed ownership evidence and late responses after loss, shutdown, or a newer grant.
The [producer checkpoint](../../plans/proof/starport-production-catalog/csp11/producer-contract-2026-09-26/verification.json) records the failing regressions and subsequent checks.

The new contract retains a process session and an independently approved backend identity with each grant.
A separate publication revision binds the selected catalog and private recovery inputs.
The revision distinguishes retained-input changes that do not change the visible catalog.
Replay checks baseline and acquisition-policy compatibility before comparing the rebuilt catalog checksum.

The contract and replay code are preparation work.
Runtime integration, the real Starport adapter, and cross-process qualification remain incomplete.
These checks do not qualify A13 or A14 and do not close CSP11.

## Toolchain policy revision: 2026-09-16

The owner selected Go 1.27.1 for both products under D36. CSP6.1 owns the coordinated migration.
Starmap previously declared Go 1.25.0 and selected Go 1.26.6.

At that baseline, its Devbox bootstrap selected Go 1.26.5.

Starport declared Go 1.26.0 and selected Go 1.26.5 in its module and Docker builder.
These differences require updates across modules, workflows, development tools, release tools, and maintained documentation.

Starmap run `35128783572` completed all four minimum-Go groups, covering 113 packages with 7,319 passing test events and 22 skips.
It also passed the release capacity check, four real-storage jobs, and three native jobs before the policy changed.
The agent canceled the remaining jobs to avoid superseded execution. The complete hosted duration remains unqualified.
The [decision proof](../../plans/proof/starport-production-catalog/csp6.1/verification-speed-2026-09-16/toolchain-decision.json) preserves these results.
Final hosted Go 1.27.1 qualification passes as recorded below.

The local Starport check roster passes on Go 1.27.1, including 3,055 test events and 29 shell checks.
Its Docker builder produces a cgo-disabled Linux ARM64 binary with Go 1.27.1 metadata.
Initial Valkey qualification passes 77 selected race events. Final published-dependency qualification passes 170 events from a broader selection.
The known MySQL test-reset defect remains with CSP15.
Both MySQL contracts pass against separate fresh databases.

Starmap passes its local repository checks on Go 1.27.1.

The compiler adds standard-library packages, so three total-package budgets no longer represent the intended dependency boundary.
Their replacement budgets count product and third-party packages. Forbidden-import checks still inspect every package.
The [qualification record](../../plans/proof/starport-production-catalog/csp6.1/verification-speed-2026-09-16/go127-qualification.json) identifies the source and remaining gates.

The paired Go 1.27.1 race measurements retain every selected test and assertion.
Bootstrap falls from 83.01 to 14.62 seconds. Runtime falls from 129.76 to 19.50 seconds, and catalog concurrency from 13.89 to 3.99 seconds.
These single paired runs exclude compilation. They do not establish a statistical latency guarantee.

Initial Linux CI exposed a golangci-lint 2.12.2 parser crash on Go 1.27 syntax.
Version 2.13.2 passes fresh-cache analysis for both products with the same enabled checks.

Devbox also needed Apple SDK 15.5 for the Go 1.27 development-tool linker.
Starport native startup passes on Linux and macOS. Both Windows jobs fail before readiness.

The corrected verifier preserves that failure and redacts generated credentials. Its nine regression tests pass.

The Windows diagnostic identifies Python temporary directories that use the OWNER RIGHTS identity.
Starmap rejects this identity as foreign, although Windows resolves it to the current object owner.
Commit `71a96cee0` resolves the identity after validating the owner. Foreign owners and Creator Owner grants remain refused.

The regression fails before the fix. Local ACL and private-file tests pass 180 race events.
Starport now pins published Starmap commit `71a96cee0`. Its complete local roster and Docker checks pass without a workspace override.
Both final pre-PR reviews pass with no findings.

The six native jobs now pass in both repositories. The complete Starmap run takes about 55 minutes, above the 30-minute target.
Starmap's runtime race group fails the ten-second publication barrier. Starport's Windows race group fails permission-state writes with `ERROR_NOACCESS`.
Native startup results do not qualify those failing race paths.

Starmap commit `334ab50cd` uses the existing small runtime fixture and explicitly observes the waiting collector before cancellation.
Its 15 input-collection race events pass. The affected filesystem and workflow packages pass 192 race events.

The same commit aligns the Windows file-identity buffer and preserves its serialized bytes.
Windows lint, cross-compilation, and native file-publication race checks pass. Starport's complete Windows race suite also passes with the new dependency.
The workflow adds a Windows AMD64 race check for file publication and private files.

Both renewed reviews pass. Starport `fff38dff` pins Starmap `334ab50cd`. Its local roster, real-Valkey contracts, and Docker checks pass.
Starport passes all ten CI jobs and all six native jobs. Starmap passes all 19 hosted jobs.

Starmap PR #164 merged at `6d4bcb5e7`. Starport PR #375 merged at `f5353a8b7`.
Both merged trees match their reviewed candidates. Starmap's final hosted run took 48m42s, above the 30-minute target.

Starmap merged-source filesystem and workflow race checks pass.
Starport merged-source verification found a repeated startup-test failure: cold local initialization under race detection exceeds its ten-second deadline.
The response-barrier repair at `8ef9151` holds authority responses until startup returns and retains a one-minute safety deadline.

Both focused race cases and 3,055 full-suite test results pass, with 39 existing skips.
All 29 required shell checks, vet, lint, and build pass. Both reviewers report no findings.
Starport PR #376 merged at `773ba4568` after all ten CI jobs passed.
Merged-source authority checks pass. CSP6.1 is complete, with the missed 30-minute hosted target retained in its proof.

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

## Publisher qualification findings: 2026-09-16

The configured App private key authenticates successfully. The publisher does not use the OAuth client secret.
Hosted run `35097794323` acquired thirteen fresh scopes, then failed promotion validation before any public write.
Its original failure log omitted the staging command's diagnostics.

A local capture reproduced the loss of membership scopes during YAML projection.
Repair `f8302a033` preserves those records and staging diagnostics.
Testing that captured catalog as embedded input then found that bootstrap replaced its original source observation links.
Commit `931ab7528` retains and validates the complete accepted generation manifest.

Runtime smoke tests then found that startup classified compiled provider evidence as undeclared local evidence.
Commit `985331d94` distinguishes the exact compiled baseline and preserves its scope observations during rebuilds.
Nineteen focused race events pass on each supported Go version. Six smoke events pass against the captured catalog.

The [repair proof](../../plans/proof/starport-production-catalog/csp6/membership-promotion-2026-09-16/runtime-verification.json) records source identities, failures, and current verification.
Sol and Opus report zero findings for PR #164. Required CI, merge, and all three hosted A05 subcases remain open.
These findings do not grant publication qualification.

## Public publisher clarification: 2026-09-14

The owner confirmed that the scheduled Starmap publisher uses provider accounts with public models.
Its public checkpoint can reside on GitHub. This profile needs no checkpoint encryption key or private object store.
Provider API keys remain GitHub Actions secrets. Enterprise publishers retain their own private storage policy.

Local CSP6 source `9aff3fac5` defines the checkpoint in `internal/catalog/publication/state.go`.
The encoder stores the baseline, accepted artifact, observation receipts, and retained catalog payloads.
It does not serialize the credential resolver or raw diagnostic messages.
Observation receipts retain source binding selectors, so a private enterprise profile still needs restricted storage.
Restore checks a separately trusted digest, canonical encoding, and retained-input replay.

These observations describe local implementation, not completed publication support.
GitHub checkpoint retention, channel receipt verification, checked promotion, and final qualification remain open under CSP6.
The [engineering specification](ENGINEERING_SPEC.md#53-public-publisher-checkpoints) records the accepted storage scope.

## Public publisher integration: 2026-09-14

The [local integration proof](../../plans/proof/starport-production-catalog/csp6/publication-integration-2026-09-14.json) records receipt retention, promotion checks, and replay compaction.
Channel v2 binds a current run receipt, public checkpoint metadata, and the promoted source commit.
The release command validates local checkpoint replay and exact promoted catalog files before it stages the channel.
The workflow must still verify provenance and confirm the default-branch merge.

Runtime source retention keeps current run evidence separate from the original immutable catalog artifact.
Restart tests preserve the accepted receipt and original observation times.
Status exposes a fixed-size summary. An explicit accessor returns a copy of the full receipt.

The first capacity regression retained eight full observations after eight unchanged polls.
The corrected provider, metadata, and unresolved-metadata fixtures retain two observations.
A thirteen-run test compares compacted state against all original observations.
It covers field changes, scoped absence, return, explicit account removal, and checkpoint restore after every run.
Large public profiles and distinct input changes still need sustained capacity qualification.

The proof distinguishes final focused checks from earlier integration checks.
Both broader runtime attempts reached their five-minute test deadline after 250 passing test events across six selected packages.
Those attempts do not qualify the complete runtime suite.
CSP6 still requires workflow integration, controlled bot promotion, final checks, review, and merge.

## Public baseline and source profile: 2026-09-14

Local commit `bb68cc695` applies an explicitly trusted compiled baseline when the publisher restores a checkpoint.
It preserves the separate acquisition baseline when compiled input matches the publisher's own accepted catalog.
This prevents repeated promotion from making provider facts permanent after explicit source removal.
Tests cover baseline model replacement, alias removal records, invalid successors, checkpoint restore, and retry identity.
They do not yet qualify authored edits to a catalog that already contains promoted acquisition results.
Workflow integration must preserve the distinction between those edits and unchanged acquisition fields.

The checked profile selects models.dev HTTP and twelve provider APIs.
Bindings contain no account or project selectors and limit membership changes to their own scope.
DeepInfra uses its declared public acquisition profile. Other selected providers use API-key profiles.
The initial draft required models.dev evidence no older than 24 hours and permitted provider outages.
D35 supersedes that draft cutoff with explicit stale retention and source-age reporting.

The [proof](../../plans/proof/starport-production-catalog/csp6/public-baseline-2026-09-14.json) records 115 passing package race events and seventeen passing minimum-Go events.
It preserves the initial test compilation error and the missing-profile failure before configuration existed.
Policy and strict prose pass. No live provider request ran.
Pending-promotion recovery, workflow integration, actual-profile capacity, native qualification, review, and merge remain open.

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

Complete `make verify` passes at `717ba914`, with 82 package outcomes in both ordinary and race modes.
The final runtime race package completes in 1,533.210 seconds. All 15 coverage thresholds, container smoke, documentation, prose, and offline CLI checks pass.

Catalog pointer reads take 8.100–11.43 ns with zero allocations across three runs. This measures the accessor, not complete gateway overhead.
Required Sol and Opus review passes with zero findings. The changed-content secret scan passes.
[PR #148](https://github.com/agentstation/starmap/pull/148) is open, and native CI tests the reviewed head. A reviewed merge remains required.

## Complete permission clock samples, 2026-09-10

Local `0d1920927164a3a64216d692912587256511fb26` adds `runtime.WithPermissionClock` for one complete time and uncertainty sample.
Admission, receipt relay, and permission status use that same callback contract. The scheduler retains its separate clock.
The legacy uncertainty-only option remains available. Selecting both contracts fails before construction.

The [clock proof](../../plans/proof/starport-production-catalog/csp4/permission-clock-2026-09-10/verification.json) records eleven focused Go 1.26 race checks and 51 Go 1.25 authority/status checks.
Concurrent unsafe samples remain denied. Admission retains zero allocations, and clock failure preserves diagnostic metadata.
Static checks and the 1,452-file prose check pass. Full repository verification is active.
This API does not qualify native clock evidence or complete CSP4. The parent origin PR #148 still awaits CI and merge.


## Bounded clock cache: 2026-09-10

Local `d5ef510b020aeec260c2e108d80f962ba4ae5858` adds `permission.NewClockCache`.
The cache separates explicit observations from permission reads and accounts for query delay, counter error, rate drift, and expiry.
Invalidation also prevents an unfinished observation from restoring evidence. The host still qualifies native sources and counters.

The [cache proof](../../plans/proof/starport-production-catalog/csp4/clock-cache-2026-09-10/verification.json) records 127 permission test events per supported toolchain.

The checks explicitly select Go 1.25.12 and Go 1.26.6.

The 37 new cases include concurrency, invalidation, cancellation, suspend, uncertainty rounding, and issuer refusal after expiry.
Static checks, complete documentation, and the 1,454-file prose check pass. The isolated benchmark excludes native calls and gateway overhead.

Read-only macOS probes measure the existing `unix.ClockGettime` binding at 36–44 ns, with zero measured allocations.
The typed `purego` binding measures two allocations per call. These observations do not qualify production clock accuracy or supported platform coverage.
The [native survey](../../plans/proof/starport-production-catalog/csp4/native-clock-survey-2026-09-10.md) owns adapter research.

The current host defaults to Go 1.27.0. The parent full verifier uses that toolchain for its running race suite.
Earlier unpinned artifact names do not establish Go 1.26.6 qualification.
An explicit Go 1.26.6 rerun at `0d192092` passes all 51 authority/status events.
The [parent proof](../../plans/proof/starport-production-catalog/csp4/permission-clock-2026-09-10/verification.json) records that correction.
All eight CSP4 acceptance checks remain UNVERIFIED.

## Provider documentation and public publication, 2026-09-11

Workflow `34538080930` fails provider validation at source `faa9cc8b`.
The models.dev parser places a general documentation URL inside an empty catalog-acquisition configuration. The shared fetch path copies that configuration.
Cohere has no catalog-acquisition endpoint, so the resulting endpoint type fails validation.

Local repair `b81b8270` maps this URL to `Provider.DocsURL` in both paths.
The shared fetch path copies absent documentation and preserves existing documentation.
Reconciliation retains Cohere and OpenAI acquisition, inference, credential, and curated-documentation contracts for both HTTP and Git observations.

The [repair proof](../../plans/proof/starport-production-catalog/csp4/provider-docs-2026-09-11/verification.json) records 846 passing race events per supported toolchain across four packages.
Lint, ago, generated documentation, and the 1,415-file prose check pass. The full repository verifier passes all forty stages at the repair checkpoint.
Both test modes pass 81 packages, and all fifteen coverage thresholds pass.
The initial allocation test failure remains recorded. Thirty isolated runs and both final combined suites pass without a threshold change.

The public channel remains at sequence 19 from September 8's publication. Its catalog declares only schema 6 compatibility.
Current Starmap emits the schema 9 format and explicitly supports schema 6 reads. The emitted schema does not identify every supported read schema.
Read-only GitHub CLI checks verify the channel and archive attestations against the expected workflow.

Live probes at `b81b8270` also verify the source adapter and connected runtime without credentials.
The runtime starts usable on embedded fallback. Explicit refresh activates the signed public generation and clears fallback.
Source health becomes healthy. Catalog freshness remains critical because publication stopped at September 8. Provider acquisition stays disabled.

The failure blocks freshness, not legacy decoding. This review made no release or channel changes.

CSP6 owns the publication repair. CSP4 remains the only active task.
The combined source `e12253fc` includes the repair, actual main `21e7356b`, and signed public-catalog acceptance fixtures.
Its [acceptance proof](../../plans/proof/starport-production-catalog/csp4/public-acceptance-2026-09-11/verification.json) records all four A07 checks passing after main integration.
Signature, checksum, size, and replay failures retain the accepted catalog. Restart preserves that catalog while the source is unavailable.

The complete runtime, remote, and artifact race suite passes 912 events across three packages without failures or skips.
The minimum-toolchain check passes fourteen scoped events across two packages. Static checks, complete documentation, and prose also pass.

An isolated copy runs the scheduled generation script without provider credentials.
All four validation components pass with zero issues. The validator checks seventeen providers and 618 model entries across those providers.
Thirteen credentialed providers skip acquisition without a request. DeepInfra supplies 191 models through its public endpoint.

Four Starport consumer subcases remain UNVERIFIED. Review, native CI, and merge remain required for this delivery.
CSP4 receives no task completion credit from these local checks.

The provider-list API also read documentation only from the acquisition contract.
Final source `f0a2d3d2` prefers `DocsURL` and retains that existing fallback.
Both supported toolchains pass 82 handler, cache, and OpenRouter events. Fresh and cached responses preserve the documentation policy.
The 912-event runtime suite predates this API-only correction. Its runtime and catalog-source files remain unchanged.


## Explicit public fixture setup, 2026-09-11

Source `079c390b` prepares the immutable archive before tests and verifies its captured size and checksum.
The final tracked tree excludes the archive. Catalog tests embed the prepared bytes and use their local HTTP fixture.
Missing archives skip ordinary tests and yield UNVERIFIED acceptance cases. CI explicitly prepares and requires the archive.

Seven setup tests cover integrity, cache reuse, and failed publication. The anonymous download verifies 411,974 bytes.
All four A07 checks pass. The minimum-toolchain race suite passes fourteen test events, and workflow tests pass 28 events.
Static checks, complete documentation, and the 1,458-file prose check pass.

The required review initially refused the output location, then a binary change. Neither attempt ran a reviewer.
Explicit fixture setup resolves the binary refusal. Required Sol and Opus review passes against the complete final branch diff with zero findings.
The [acceptance proof](../../plans/proof/starport-production-catalog/csp4/public-acceptance-2026-09-11/verification.json) retains both refusals and the replacement checks.


## Windows fixtures and consumer permission, 2026-09-11

PR #149 fails signed channel verification on both Windows architectures at `079c390b`.
A fresh Git checkout with Windows text conversion reproduces changed fixture digests. Source `f29ec2a4` preserves the signed bytes through attributes.
Eight setup tests and fourteen runtime/artifact race events pass locally. Native rerun remains required.

Starport accepts a routing catalog after Starmap activates a candidate. Runtime readiness alone does not identify the catalog Starport can still enforce.
Local source `3d023778` adds `AllowsCatalogAttempt` for a consumer's validated authority head. The consumer must authenticate that head and bind it to its catalog.

An old required revision refuses after withdrawal. Equal permission revisions preserve service during route preparation.
Known sequence contradictions also refuse. The [consumer proof](../../plans/proof/starport-production-catalog/csp4/consumer-permission-2026-09-11/verification.json) retains each regression.

Both supported toolchains pass 307 scoped race events. The complete catalog suite passes 1,253 events without failures or skips.
Canonical checksum validation now preserves the same accepted syntax without temporary decode-and-encode buffers.
The valid consumer check and checksum validator pass zero-allocation assertions. These measurements do not establish total Starport request overhead.

The new Starport consumer worktree compiles three packages against the prepared Starmap source through an isolated workspace.
It runs no behavior tests and changes no module pin. All four required Starport consumer checks remain UNVERIFIED.


## Atomic catalog authority snapshots, 2026-09-11

Consumer source `20b1ee41` exposes catalog state and authority heads through separate reads.
A publication between those reads can pair metadata from different generations.
The existing catalog snapshot contract does not include the authority head.

Commit `44f10ac2` adds `CatalogState.AuthorityHead` under the existing publication lock.
Runtime source selection, origin publication, and retained startup preserve that binding. Ordinary reconciliation clears upstream authority.
A failed activation preserves the retained catalog head while permission state records the withdrawal.

Five regression cases fail before propagation. Both supported toolchains pass six focused cases.
They cover publication, concurrency, retained startup, source selection, origin rebuild, and failed activation.
The combined client snapshot passes zero-allocation and zero-storage-read assertions.

The [snapshot proof](../../plans/proof/starport-production-catalog/csp4/authority-snapshot-2026-09-11/verification.json) records these checks and the combined source.
Broader verification, required review, native CI, and merge remain required. Starport admission still needs integration.


## Starport authority snapshot binding, 2026-09-11

Local `aceaf0b` retains the authority head in each routing snapshot.
Availability-only rebuilds preserve it. Ordinary catalog replacement clears it, while previously retained snapshots remain unchanged.
Acceptance compares exact authority metadata with the stored generation before advancing the head or accepting an idempotent repeat.

The [binding proof](../../plans/proof/starport-production-catalog/csp4/starport-snapshot-2026-09-11/verification.json) records fourteen failing events before the fix.
Fourteen focused events and 148 complete catalog race events then pass against local Starmap `768347ab`.
The new tests use Badger directly and include close and reopen. Snapshot reads pass their zero-allocation assertion.
Vet and lint pass. A fresh lint cache resolves findings that named deleted source paths without changing code or policy.

The unchanged published module lacks the permission API package. The implementation remains local and requires a compatible published module.
Current permission checks on attempts and cached responses remain incomplete. All four mapped Starport consumer checks remain UNVERIFIED.


## Starport authority acceptance order, 2026-09-11

Local `1979d34` uses the Starmap authority sequence contract when accepting an authoritative generation.
Timestamp order previously rejected a newer sequence after a clock correction and accepted an older sequence with a later timestamp.
The fix rejects replay, conflicting content at the same sequence, and implicit authority changes. Ordinary generations retain timestamp ordering.

The [order proof](../../plans/proof/starport-production-catalog/csp4/starport-order-2026-09-11/verification.json) records nine failing events before the fix.
Twenty-two focused events and 158 complete catalog race events then pass against local Starmap `768347ab`.
Tests use real Badger storage and verify the retained manifest and payload after reopen.
Explicit source transitions and current request permission checks remain incomplete.


## Starport snapshot permission owner, 2026-09-11

Local `c505a09` binds accepted catalog snapshots to the current Starmap permission owner.
The new snapshot check refuses cold internal metadata, known withdrawal, and a closed runtime. It preserves retained metadata for diagnostics.
Every publication path preserves the binding. Ordinary standalone catalog reads still work without a connected runtime.

The [permission proof](../../plans/proof/starport-production-catalog/csp4/starport-permission-2026-09-11/verification.json) records three failures before the API addition.
Three focused tests and 171 complete catalog/registry race events then pass. Valid permission reads allocate zero times.
A deterministic source exercises the real Starmap runtime. Production composition and shutdown use real Badger storage.

These tests do not qualify native clocks, network trust, or request-path enforcement.
Route selection, every provider attempt, and exact or semantic cache delivery must call the new check.


## Starport request permission, 2026-09-11

Local `52970f7` checks current catalog permission before route selection and each provider attempt.
Exact and semantic cache delivery also check permission. A refused cached stream remains refused if permission later returns.
Both API formats return HTTP 503 before stream headers when the first read refuses permission. Admitted streams can complete.

The [request proof](../../plans/proof/starport-production-catalog/csp4/starport-request-permission-2026-09-11/verification.json) records 25 focused race events and 645 package race events.
One opt-in overhead benchmark skipped. The successful permission check allocates zero times.
The tests do not qualify total gateway latency, native clocks, fleet behavior, or the released dependency pair.

Vet, lint, six dependency checks, and new-comment checks pass. Thirty-five existing prose diagnostics match the prior source.
Four Starport consumer cases remain UNVERIFIED until configured composition and its dependencies pass their required checks.


## Starport authority configuration, 2026-09-11

Local `95354f0` loads authority and policy identity pins and forwards them to Starmap.
The `require_authority` policy requires a Starmap source and disabled acquisition.
Other startup policies reject identity pins. Starmap owns exact identity validation.

The [configuration proof](../../plans/proof/starport-production-catalog/csp4/starport-authority-settings-2026-09-11/verification.json) records twelve focused tests and 394 package race events.
One Valkey integration case skips. Cold construction preserves metadata diagnostics and refuses inference without source I/O.
Vet, lint, and six dependency checks pass. Existing prose diagnostics match the prior source.

The operator guide states the remaining clock limitation. No qualified host clock adapter makes authority inference ready yet.
The four consumer cases and the released dependency pair remain UNVERIFIED.


## Native clock observations, 2026-09-11

Local Starmap `0868781d` adds Linux and macOS host clock adapters.
Explicit observations read kernel synchronization evidence and reject unsafe states, timestamps, and uncertainty.
The elapsed counters include system sleep and pass zero-allocation checks.
At that commit, Windows supplies neither a UTC observation nor an elapsed counter.

The [native proof](../../plans/proof/starport-production-catalog/csp4/host-clock-2026-09-11/verification.json) records 25 Linux and 15 macOS events.
The macOS binding returns a sample with 500.001 ms uncertainty and agrees with the host UTC interval.
The Linux container supplies unqualified clock state, which the adapter refuses.
A positive synchronized Linux observation remains UNVERIFIED.

Both supported toolchains pass 142 permission events. All 28 workflow tests, lint, vet, ago, documentation, and prose checks pass.
No host clock setting changed. Production error profiles, native Windows support, and host scheduling remain incomplete.
The four Starport consumer cases remain UNVERIFIED.

## Windows clock prototype, 2026-09-11

Local Starmap `41071ef4` adds an elapsed counter through the Go runtime's Windows interrupt-time reading.
The [counter and prototype proof](../../plans/proof/starport-production-catalog/csp4/windows-counter-2026-09-11/verification.json) retains the implementation and supported runtime source hashes.
Both supported toolchains cross-compile the Windows AMD64 and ARM64 tests.
Native Windows interval and allocation tests have not executed. UTC observation still refuses permission.

The Darwin permission suite passes 142 race events. Windows lint and vet selections, ago, generated documentation, and prose checks pass.
The complete prose check covers 1,473 files with zero diagnostics.

A separate W32Time prototype passes twelve race events on each supported toolchain.
The tests cover QueryStatus decoding, blocked-bind cancellation, fixed transport selection, reply bounds, and deadlines.
They do not prove native Windows pipe access, process identity checks, cancellation after query dispatch, or complete response fragmentation.

The standalone Windows probe uses 126 nonstandard packages and builds for both architectures.
A shadow module graph adds eighteen modules without changing existing versions.
The product dependency files remain unchanged. Native access, UTC uncertainty, operational profiles, and host scheduling remain open.

## Windows observer candidate and authority merge, 2026-09-11

Starmap PR #150 merged as `852a548c` after fifteen successful checks.
The merged tree exactly matches reviewed `e3943c97`. Both queues were empty at that merge checkpoint.

Local host-clock candidate `5418420c` includes that main revision and the Windows observer.
Its [proof](../../plans/proof/starport-production-catalog/csp4/windows-observer-2026-09-11/verification.json) records 215 race events per supported toolchain and eight Windows test cross-builds.
Scoped lint, vet, ago, generated documentation, and the 1,489-file prose check pass.

All 41 repository verification stages now pass across the initial run and corrected remainder.
Sol at xhigh and Opus at high report zero findings. The secret scan passes.

The initial [PR #151](https://github.com/agentstation/starmap/pull/151) candidate is `5418420c`. All four Linux/macOS native jobs pass.
Both Windows architectures fail the direct counter API lookup and the legacy time-service pipe test.

PR #151 now contains correction `0dfa9a90`, which passes local checks and both reviews. Native workflow `34583505814` passes both Windows counter tests.
Both W32Time status calls return RPC access denied. Authentication and caller-privilege diagnosis remains open. The [binding proof](../../plans/proof/starport-production-catalog/csp4/windows-binding-2026-09-11/verification.json) retains counts and native limitations.

The candidate requires explicit source bounds, checks the service and pipe process, and limits the RPC exchange.
Tests cover cancellation before and after dispatch, fragmented replies, malformed limits, source-age bounds, and recovery from a blocked native call.
One native query can remain active after caller cancellation. Its slot prevents more native calls until it finishes.

The candidate adds go-msrpc and go-winio without changing existing module versions.
Native tests will distinguish pipe interoperability from a source that meets the test profile.
Neither result alone qualifies production UTC accuracy or nonadministrator service accounts.
The later host configuration candidate below supplies refresh scheduling. Production qualification and the four mapped Starport cases remain incomplete.

### Host clock lifecycle candidate

Local commit `ab84ba16` adds a passive clock monitor with explicit startup and shutdown.
The [lifecycle proof](../../plans/proof/starport-production-catalog/csp4/clock-lifecycle-2026-09-11/verification.json) records six focused cases and 193 permission race events per supported toolchain.
Lint, vet, ago, generated documentation, and the 1,480-file prose check pass.

The monitor keeps source queries outside admission reads. Failed observations clear permission time and remain visible in local status.
Cancellation prevents late samples from restoring evidence. Successful cached reads pass a zero-allocation assertion.
This monitor alone supplies no canonical settings or host composition.

### Managed runtime clock candidate

Local commit `5f4ce61d` gives one permission clock monitor to the connected runtime.
The [runtime proof](../../plans/proof/starport-production-catalog/csp4/clock-runtime-2026-09-11/verification.json) records six focused race cases per toolchain and 65 related runtime cases.
Runtime lint and vet, repository ago, generated documentation, and 1,482-file prose pass.

Failed startup cancels an owned monitor. A duplicate ownership attempt preserves the first runtime.
An origin can use the managed clock for receipt issuance. Successful cached runtime reads allocate zero memory.
The later configuration candidate connects the Starmap CLI. Starport adoption remains open.

### Canonical host clock configuration candidate

Local commit `e97d7fc3` adds eight environment variables, CLI flags, and YAML keys for native permission clocks.
The public schema marks each value as node-scoped and requiring restart. Catalog source changes preserve their independent precedence.
Application composition validates a complete native profile and gives the runtime ownership of its monitor.

The [settings proof](../../plans/proof/starport-production-catalog/csp4/clock-settings-2026-09-11/verification.json) records 35 focused race events per toolchain and one Windows-only skip.
Four Windows cross-builds pass. Corrected settings packages pass 112 race events, and the fixture checks pass 39 events.
Lint, vet, ago, generated documentation, 73 verifier regression tests, and the 1,490-file prose check pass.

The broad initial run records 580 passes and ten failures from missing example entries and obsolete fixtures.
The corrected focused runs pass. All 41 repository verifier stages now pass at `e97d7fc3`. Parent integration, final review, and publication remain open.
Production clock bounds, Starport composition, and the remaining CSP4 authority cases still need qualification.

## Authority selection on restart, 2026-09-11

The regression at `e97d7fc3` permits ordinary startup after origin configuration disappears.
Both retained and new runtime directories reproduce the failure. The stored catalog remains authoritative while ordinary admission no longer requires its permission.

Candidate `8206a2a8` rejects that startup with a configuration error before workspace recovery or background work.
The [proof](../../plans/proof/starport-production-catalog/csp4/authority-selection-2026-09-11/verification.json) records twelve passing race events per toolchain and exact store preservation.
Existing origin and authoritative-subscriber restarts still pass. Static and documentation checks pass.
Integration, explicit authority transitions, full repository verification, review, and publication remain open.

## Windows authenticated status candidate, 2026-09-11

Commit `437dc13c` adds native SSPI authentication after the local pipe identity check.
The same pipe supplies the service principal and carries the protected status query.
The process uses its Windows identity with packet privacy and identification-only access. Product code enables no privilege and changes no clock setting.

The [authentication proof](../../plans/proof/starport-production-catalog/csp4/windows-authentication-2026-09-11/verification.json) records 194 portable race events per toolchain and eight Windows cross-builds.
Windows lint and vet, repository ago, generated docs, full docs checks, and the 1,495-file prose check pass.
Native tests cover privacy, changed headers, changed payloads, changed signatures, replay, and native handle cleanup. They pass on both Windows architectures.

Both service reads fail at the combined principal check. Both read-only `w32tm` queries succeed.
Sol and Opus report zero findings for `437dc13c`. The [diagnostic candidate](../../plans/proof/starport-production-catalog/csp4/windows-principal-2026-09-11/verification.json) separates each rejected reply category without changing security policy.
CSP4 retains all remaining clock, authority, and Starport qualification requirements.

Both Windows architectures at `e4a80fa6` confirm that W32Time returns a successful principal reply with an empty name.
Each passes 1,913 test events and fails only the service read. The native SSPI tests pass.
Correction `e6bf2fce` selects a null SSPI target for this verified local endpoint and preserves all required authentication flags.
Its [proof](../../plans/proof/starport-production-catalog/csp4/windows-unnamed-2026-09-11/verification.json) records local verification, zero review findings, and native execution.

Both Windows preflights pass 22 events and fail the status RPC with access denied.
The service query remains unqualified, and both full Windows suites remain unrun at this source.

## Configured authority origins, 2026-09-11

The Starmap origin API existed before the CLI could configure an origin.
The fail-before check rejects both enabled and disabled declarations as unknown catalog settings.
The implementation now accepts one complete declaration, composes the native clock, and uses the canonical catalog store.
Its [proof](../../plans/proof/starport-production-catalog/csp4/origin-settings-2026-09-11/verification.json) preserves the fail-before and local verification.

Tests verify whole-declaration replacement without inherited authority, policy, bootstrap, or lifetime fields.
They also verify origin publication and refusal of implicit ordinary startup after origin disablement.
Local configuration checks pass 120 events per toolchain. Runtime and CLI checks pass 49 events per toolchain.
Native parent integration, full verification, review, publication, and merge remain open.
The plan's delivery checklist preserves the remaining shared-store, transition, and Starport acceptance work.

## Origin follower adoption, 2026-09-11

Local `9ffcb0f8` adds `Client.Reload` and periodic accepted-store checks for shared-lease origin followers.
The [proof](../../plans/proof/starport-production-catalog/csp4/origin-adoption-2026-09-11/verification.json) records 33 passing race events per supported toolchain.
Pinned lint, generated documentation, and the 1,511-file prose check pass.
The fail-before test shows that a follower previously lacked a refresh schedule for accepted storage.

Followers adopt compatible generations without shared writes or lease acquisition. An unchanged authority head avoids a full catalog read.
Invalid updates preserve the accepted catalog. A running follower needs matching retained inputs before it can take publication ownership.

The tests use a memory store and a stub lease. Cold restart eligibility and full fleet failover remain unqualified.
CSP11 owns shared input recovery, equivalent acquisition capability, and atomic backend fencing.

CSP4 still requires explicit authority transitions, native clock qualification, production composition, full delivery checks, review, and merges.
Its eight mapped component checks pass with deterministic clocks. These checks do not prove the remaining production paths.
Both Windows jobs remain red in PR #151 at `a6545493`. Both Linux and both macOS jobs pass.

## Windows RPC correction and replica restart ownership, 2026-09-11

Native diagnostic `082c1daa` compares pipe and RPC impersonation separately on Windows AMD64 and ARM64.
Both architectures reject identification-only RPC, including after time privilege enablement. Both accept local RPC impersonation with identification-only pipe access.
The [diagnostic proof](../../plans/proof/starport-production-catalog/csp4/windows-security-2026-09-11/verification.json) retains every profile, failure, and source hash.

Production correction `75ac9d07` changes the RPC setting and preserves the pipe restriction, packet privacy, peer checks, and bounded replies.
It enables no privilege and changes no time setting. Sol and Opus report zero findings.
The [production proof](../../plans/proof/starport-production-catalog/csp4/windows-rpc-2026-09-11/verification.json) records four passing cross-builds and static checks.
PR #151 contains the correction.

Both Windows access preflights pass in native workflow `34609335965`. Full native and repository gates remain in progress. No merge credit applies.

Origin `bb28a0bd` fixes initial ownership when a shared-store replica lacks retained acquisition inputs.
The replica retains the accepted catalog without an initial lease request or shared write. Matching inputs permit ownership without replacing that catalog.
The [restart proof](../../plans/proof/starport-production-catalog/csp4/origin-restart-2026-09-11/verification.json) records the failing startup regression and 21 origin checks per toolchain.
Two additional lease checks pass per toolchain.

These memory-store and stub-lease tests do not qualify a fleet backend.
CSP11 retains shared input recovery, equivalent acquisition capability, and atomic fleet fencing.

The prior current plan incorrectly kept restart eligibility in its implementation backlog after this local fix.
The corrected record separates completed local checks from pending delivery gates and merges.
Explicit authority transitions and production clock composition remain unfinished CSP4 work.

## Subscriber transitions and native runtime qualification, 2026-09-11

The positive authority-transition test fails when the replacement omits the previous authority's alias inventory.
Commit `5066856d` scopes alias history to an authority and policy pair. Same-context and ordinary updates retain the existing alias checks.
A fresh runtime directory also exposed an invalid startup comparison against embedded aliases.
The runtime now retains available catalog diagnostics until an approved source generation arrives.

The [transition proof](../../plans/proof/starport-production-catalog/csp4/authority-transition-2026-09-11/verification.json) records 109 passing race events per toolchain. Go `1.25.12` and `1.26.6` both pass.
Tests cover changed authorities and policies, failed writes, restart recovery, exact replacement activation, and retained offline startup.
The first implementation attempted a retry before journal recovery. The final test follows the existing restart contract and verifies the exact storage failure.
Static checks, generated documentation, and the 1,514-file prose check pass.

All six native runtime jobs pass at `75ac9d07` in PR #151. The [production proof](../../plans/proof/starport-production-catalog/csp4/windows-rpc-2026-09-11/verification.json) retains every artifact and identifies skipped diagnostics.
Only the repository Verification Gate remains before the native parent merge.
Combined origin source `d1bb47e1` integrates that reviewed parent and starts full repository verification.
Its result remains unverified. Starport clock composition, delivery review, publication, native CI, and merges remain open.

## Starport native clock configuration, 2026-09-11

The previous Starport loader silently ignored permission clock settings. Twenty configuration test events failed before the change.
Candidate `9669b61d` reads both product prefixes through Starmap's canonical parser and gives the native monitor to the runtime.
Its [proof](../../plans/proof/starport-production-catalog/csp4/starport-clock-2026-09-11/verification.json) records 429 package race events, including 26 new clock events.
The optional Valkey integration case skips without its test URL. Six Windows cross-builds and the affected static checks pass.

The first lifecycle test assumed construction-context cancellation closed the runtime. The existing runtime detaches that context and requires explicit Close.
The corrected test verifies that ownership contract and monitor shutdown. Both the initial failure and final pass remain in the proof.
New prose has zero diagnostics. The full operator guide retains 48 unrelated baseline diagnostics.

Tests use a temporary workspace with Starmap `d1bb47e1`. The committed published dependency predates the clock API.
Published dependency qualification, full delivery gates, required review, native CI, and merge remain open.

The [integrated task gate](../../plans/proof/starport-production-catalog/csp4/integrated-task-2026-09-11/verification.json) passes eight selected subcases against Starmap `d1bb47e1` and Starport `9669b61d`.
It records 54 race events and no skips. The complete task race command also passes 986 events without failures or skips.

The repository verifier later failed production lint. The [portable query correction](../../plans/proof/starport-production-catalog/csp4/windows-portable-entry-2026-09-11/verification.json) passes all six native jobs.
Its 11,704 passing events include both Windows clock preflights. PR #151 merged as `76b91d34` after fifteen passing checks.

Combined source `6419ea85` passes all 41 repository stages across two invocations and all eight CSP4 subcases.
The original verifier failed only an ignored PR draft prose check. The continuation passes the remaining stages without changing tracked source.

Both runtime reviewers report zero findings. GitHub serves reviewed `6419ea85`, and Starport `a8708e03` pins that public module.
Starport passes all 33 repository commands with `GOWORK=off`. Runtime integration `33118919` preserves the tested `6419ea85` tree.
The published pair passes all eight CSP4 subcases with 54 race events. Both runtime publication reviewers report zero findings.

Starmap PR #152 merged as `f9951ee6` after all fifteen CI checks passed.
Starport correction `ba9b0d8e` fixes the startup-test race and lets Starmap create its state directory with native access rules.
All 33 required repository commands, eight task subcases, and both review panels pass.

Starport PR #373 merged as `cb6f03a2` after all ten CI checks passed.
The [completion proof](../../plans/proof/starport-production-catalog/csp4/merged-qualification-2026-09-11/verification.json) confirms both merged trees equal their reviewed source.
The final pair records 56 mapped race events with no failures or skips. CSP4 is complete.
Full released-pair, fleet, and production latency qualification remain separate plan work.

## Real Valkey follow-up, 2026-09-11

The [Valkey integration proof](../../plans/proof/starport-production-catalog/csp4/valkey-2026-09-11/verification.json) records 35 passing race events at Starport `9669b61d`, with no skips.
The application, storage, pub/sub, and KVStore contract checks use an isolated Valkey `7.2.14` container.
The test runner pins the image digest, binds a loopback port, and removes the container afterward.
This supplies the missing real-service evidence for the earlier skipped application check.

The check uses the local combined Starmap workspace. It does not qualify fleet recovery, PostgreSQL, native clock bounds, or paid inference.
The original skipped result remains in its historical proof.

## CSP5 update controls, 2026-09-11

The [control proof](../../plans/proof/starport-production-catalog/csp5/update-controls-2026-09-11/verification.json) records local changes on Starmap base `f9951ee6`.
Local checkpoint `eeeba376` contains the controls. Starport still uses the merged CSP4 dependency.

The initial tests confirm that unknown offline settings permit HTTP requests and manual mode subscribes to watcher events.
A corrected cascade fixture confirms that the previous refresh path opens an event stream during explicit manual refresh.
Manual acquisition and preview also resolve credentials before any offline refusal.

Local corrections add separate source-refresh and network controls, finite cascade reads, and pre-acquisition access checks.
Verified local imports remain available and survive restart. Configuration descriptors and deployment examples now include both settings.
Focused race checks pass at their recorded snapshots. Remote, public configuration, acquisition, and corrected settings package tests pass.

The cascade shutdown regression confirms that runtime shutdown left a constructed source stream active.
Local checkpoint `48a9a6e5` adds `WithOwnedSource` and uses it for cascades that Starmap constructs.
Caller-supplied sources remain caller-owned unless the caller selects the new option.

The [lifecycle proof](../../plans/proof/starport-production-catalog/csp5/cascade-lifecycle-2026-09-11/verification.json) records 27 passing focused race events and preserves the original failure.
Canceled startup closes the constructed stream. A manual offline replacement can reopen the same runtime directory.

The later timeout regression confirms that the runtime released its directory while a reconnect worker remained active.
Checkpoint `52c84e28` replaces the owned-source close contract with `Shutdown(context.Context)`.

The [shutdown proof](../../plans/proof/starport-production-catalog/csp5/source-shutdown-2026-09-11/verification.json) records 36 passing focused race events and 87 remote/settings events.
Runtime cleanup retains the directory after its caller receives a timeout. Failed startup preserves its error and releases ownership only after cleanup.
Standalone `Source.Close` remains bounded. Starport needs this coordinated lifecycle through CSP8.

The earlier broad runtime check at `48a9a6e5` completed with 989 passes and two missing-fixture skips. It cannot qualify the timeout correction.
Pins and retained-state recovery also remain open.

## Configured generation selection checkpoint: 2026-09-11

Local Starmap `32951a7b` adds generation pin configuration. Parent `b3cf9b68` rechecks publication guards after queued mutations enter their transaction.
The [pin proof](../../plans/proof/starport-production-catalog/csp5/generation-pins-2026-09-11/verification.json) preserves the guard bypass, ignored selection, and unpin startup mismatch.
Corrected focused checks pass 86 race events. Public configuration passes 111 events. Corrected internal settings pass 24.

The selected configuration authority owns `CATALOG_GENERATION_PIN`. No separate pin configuration file exists.

The runtime selects a verified retained artifact before startup rebuilds. Clearing the pin restores consistent runtime and client state from retained inputs.
Pinned metadata survives permission withdrawal, but new inference attempts remain blocked. Pins cannot approve a different internal authority.

Checkpoint `32951a7b` has no durable rollback acceptance record. The later acceptance work below adds origin rollback under a new authority revision.
The complete CSP5 contract, full runtime/storage run, repository verification, review, native CI, and merge remain open.

## Durable pin acceptance checkpoint: 2026-09-11

Starmap `39922ecf` records the latest pin operation in the private `catalog-runtime/generation-pin.json` file.
The selected configuration authority still owns the pin setting. The file records recovery state and has a 32 KiB limit.
The runtime recovers failed publication with the same operation ID. An origin restores older content under a new authority sequence.

The [acceptance proof](../../plans/proof/starport-production-catalog/csp5/pin-receipts-2026-09-11/verification.json) preserves both original failures and source snapshots.
Go 1.26.6 checks pass 47 focused race events and 52 isolated events without failures or skips.
The earlier full runtime/storage command failed at 30 minutes after 969 passing events. Its stack identifies Go 1.27.0.
All recorded checks are terminal. The passing focused checks do not replace full verification.

Complete pin recovery, generation retention, owned-stage recovery, history compaction, and ambiguous filesystem publication remain open.
Task acceptance, repository checks, review, native CI, and merge remain required. CSP4 stays complete.

## Filesystem durability and pin restart checkpoint: 2026-09-11

Starmap `de8b5abe` corrects two publication boundaries.
The filesystem could expose its new pointer with an ordinary I/O error and skip synchronization on an identical retry.
A pending pin could then reach readiness after restart while the store still reported unconfirmed durability.

The [durability proof](../../plans/proof/starport-production-catalog/csp5/filesystem-durability-2026-09-11/verification.json) preserves both original regressions.
The store now reports a typed publication error and confirms durability on retry.
Pending-pin startup confirms its original commit before readiness. Accepted startup reasserts the same receipt without a new operation or acceptance time.
The tests also refuse incompatible pending configuration and unrelated catalog heads.

Go 1.26.6 checks pass 236 storage/private-file/error events, 37 pin events, and 31 consumer events without failures or skips.
The filesystem test injects a synchronization failure after real publication. It does not qualify physical power-loss behavior or native platforms.
Owned-stage recovery, generation retention, history compaction, and full CSP5 qualification remain open.

## Retained record inventory correction: 2026-09-11

Starmap `ceef5480` includes `catalog-runtime/removals.json` and `catalog-runtime/generation-pin.json` in the runtime evidence inventory.
The [inventory proof](../../plans/proof/starport-production-catalog/csp5/file-inventory-2026-09-11/verification.json) preserves the omission in both canonical and explicitly selected runtime directories.
Both records retain the existing owner-only policy. Inspection reports metadata without exposing receipt contents or opening application state.

Corrected checks pass 8 application and 86 product-path race events without failures or skips.
The ago and prose checks pass. The engineering path table now includes the pin receipt and distinguishes it from configuration authority.

Both regression runs record optional workspace repair timeouts after durable catalog activation. This correction does not qualify workspace projection.
Baseline export still lacks persistent staging ownership. Recovery, retention, compaction, full verification, and merge remain CSP5 work.

## Baseline stage recovery: 2026-09-11

Starmap `55c8bc19` adds persistent ownership records for unfinished baseline exports.
The [recovery proof](../../plans/proof/starport-production-catalog/csp5/baseline-recovery-2026-09-11/verification.json) preserves the original process-interruption and replaced-lock failures.
Both corrected regressions pass. Journals bind private writer locks, native identities, metadata, and exact file digests.

Recovery preserves active writers, changed or unknown content, and every published baseline.
The scan permits 4,096 combined directory entries and 64 MiB of declared snapshot content per pass.
Repeated verification can reread bytes. Unrecognized records and oversized stages remain preserved.
The file inventory declares the private metadata directory and retained writer lock.

Final checks pass 46 baseline, 30 path, and 7 later policy race events without failures or skips.
These checks overlap. Consumer dependency, ago, and corrected prose checks pass.
Linux AMD64 and Windows AMD64 cross-compilation pass. Native runtime and power-loss behavior remain unqualified for this change.

Other staging roles, published generation retention, history compaction, full task checks, review, native CI, and merge remain CSP5 work.

## Migration recovery: 2026-09-12

Starmap `9e875a35` fixes migration initialization, partial ownership, scan bounds, and failed directory reopen.
Initialization recursively deleted unknown operator files and left stages after process exit.
Partial-copy recovery removed files without proving ownership. Source scanning did not bound empty directories.
The [migration proof](../../plans/proof/starport-production-catalog/csp5/migration-recovery-2026-09-12/verification.json) preserves each original failure and its exact source.

Initialization now resumes a stage bound to its manifest, parent, journal lock, and native identity.
Partial recovery requires a retained ownership record and exact source-prefix bytes. Changed identities, unsupported records, and unknown content remain preserved.
Source scans permit 40,000 entries and 4 MiB of path names. Stage scans derive their entry limit from the allowed manifest layout.

Final checks pass 150 migration and 32 baseline/publication race events without failures or skips.
Consumer, ago, and final prose checks pass. Final Linux and Windows compilation includes the directory flush and reopen correction.
Only a test assertion correction follows those builds. Native runtime and power-loss qualification remain open.

Unrecorded stages and interrupted atomic owner-record scratch require explicit recovery.
Workspace, evidence, and discovery staging, generation retention, history compaction, full task checks, review, native CI, and merge remain CSP5 work.

At `68e98614`, workspace preparation and projection still use recursive cleanup.
`internal/catalog/workspace/stage_access.go` removes preparation and candidate trees.
`internal/catalog/workspace/projector.go` removes candidate, desired, and verification trees.
CSP5 must add verified ownership before those paths can satisfy its cleanup contract.

The moved-stage regression exposed deferred cleanup through a nil handle after a failed directory reopen.
The initializer now returns the filesystem error and preserves the moved stage.
The corrected regression inspects joined errors through `errors.Is`. All 150 final migration race events pass.

## Workspace cleanup and repair: 2026-09-12

Starmap `536791a0` replaces recursive removal of completed candidates with checked cleanup.
The [workspace proof](../../plans/proof/starport-production-catalog/csp5/workspace-cleanup-2026-09-12/verification.json) preserves eight original cases that delete operator content.
Cleanup now verifies native identities, content, access, and remaining entries. A same-content replacement file remains preserved because its identity differs.
The visible publication receipt survives a cleanup conflict.

Repair also rendered and validated two candidates for one operation. It now publishes the first validated candidate.
All 149 workspace race events pass without failures or skips. Final ago, prose, and Linux/Windows compilation checks pass.
Native runtime qualification remains open. No CSP5 PR or merge credit applies.

The canonical-runtime profile passes before and after this change, taking 117.50 and 74.64 seconds respectively.
Each version has one local run with race instrumentation. The earlier timeout does not recur.
The [latency review](LATENCY_REVIEW.md#workspace-repair-observation-2026-09-12) records allocation totals and qualification limits.

At `536791a0`, private preparation and verification-tree cleanup still use recursive deletion. Assembly still uses an unbounded directory read.
Persistent workspace recovery, retained evidence, discovery staging, generation retention, and compaction remain required under CSP5.

## Workspace preparation and legacy rollback: 2026-09-12

Starmap `a05ca541` records created preparation entries and actual written bytes before checked cleanup.
The [preparation proof](../../plans/proof/starport-production-catalog/csp5/workspace-preparation-2026-09-12/verification.json) preserves the original data-loss and path-name-limit failures.
Unknown children, changed files, replacement entries, and verification edits remain preserved.
Assembly now uses bounded directory reads and an index of expected child counts.
All 1,424 workspace and catalog race events pass. Application integration, dependency, policy, prose, and compilation checks pass.

Source inspection identifies another CSP5 defect in `internal/catalog/workspace/migration.go:rollbackLegacyMove`.
It compares semantic catalog content before recursively removing the projected workspace.
Unrecognized operator notes do not change that checksum. The checksum therefore does not establish ownership of everything that deletion can remove.
At `a05ca541`, the separate path still needed a failure test. The next section records that test and the correction.

Private preparation ownership remains in memory. Persistent recovery, active-writer exclusion, retention, compaction, and full qualification remain open.
No CSP5 PR or merge credit applies.

## Legacy rollback ownership: 2026-09-12

Starmap `fcfda255` preserves operator content during legacy layout rollback.
The [rollback proof](../../plans/proof/starport-production-catalog/csp5/legacy-rollback-2026-09-12/verification.json) records seven original unsafe rollback cases and their passing corrections.
Rollback now checks the published candidate inventory and original store identity. It preserves projection-marker paths and the stable writer-lock file.
Both directory moves refuse an existing destination. Cancellation retains a separate cleanup context.

Review also found that assembly could adopt a replacement file containing identical bytes. Its failure test proves that the earlier code publishes the replacement.
Assembly now checks the identities and bytes recorded during file creation before publication.
All 175 workspace and two CLI command race events pass. Final ago, prose, and Linux/Windows compilation checks pass.

At `fcfda255`, source inspection found unbounded directory and manifest reads in legacy preflight. The next section records the correction.
At `9d8b4b8b`, the projection-marker writer still removed its temporary path without checking recorded ownership.
The workspace record section below records the marker correction. Persistent workspace and legacy relocation recovery remain open.
These local results do not add PR, merge, native-runtime, or released-pair credit.

## Bounded legacy preflight: 2026-09-12

Starmap `9d8b4b8b` bounds fixed-layout reads and scans retained generations in batches of 128.
Every retained generation still requires validation. Cancellation stops the scan before another batch or generation.
The [preflight proof](../../plans/proof/starport-production-catalog/csp5/legacy-preflight-2026-09-12/verification.json) preserves the oversized-record and retained-manifest failures.

Review found that the filesystem reader previously used each file's own size as its read limit.
The reader now enforces 32 MiB payloads, 64 MiB manifests, and 16 KiB pointers and authority records.
Commit rejects unreadable records before creating generation state or changing current. The pointer limit includes its newline.

All 350 workspace, storage, private-file, and CLI command race events pass without skips.
Tests preserve unknown entries and every generation in a 129-generation migration. Exact pointer-limit restore and retry also pass.
Final ago, prose, and Linux/Windows compilation checks pass. No native execution, PR, merge, or released-pair credit follows from this checkpoint.

The first migration test expected the wrong error field. Its corrected test still fails before the migration change.
The proof retains both results. The initial prose failure also remains separate from the corrected passing check.
The next section records marker ownership. Persistent recovery, retention, compaction, and complete CSP5 qualification remain open.

## Workspace record ownership: 2026-09-12

Starmap `64f905db` replaces unchecked temporary removal in three marker and journal writers.
The [record proof](../../plans/proof/starport-production-catalog/csp5/workspace-records-2026-09-12/verification.json) retains twelve unsafe cleanup cases and their passing corrections.
A separate preserved overlay proves that the original marker reader accepts a valid record larger than 4 MiB.

The shared writer records native identity, access, and actual written bytes. It checks both the candidate and the destination before publication.
Changed files, identical-byte replacements, recreated paths, and partial-write replacements remain preserved.
Cancellation removes unchanged temporary files through a separate cleanup context. Access changes also prevent cleanup.

All 213 workspace and two CLI command race events pass without skips. ago, prose, and Linux/Windows compilation checks pass.
The result does not qualify native execution or recovery after process exit. The runtime retains these temporary ownership records only in memory.

At `64f905db`, `finishReplacementRecord` still removed a completion journal after comparing parsed content.
The next section records its correction. Full CSP5 acceptance, review, native CI, and merge remain open.

## Journal completion and backup identity: 2026-09-12

Starmap `e2cbbc6b` binds completion cleanup to the journal accepted during publication or recovery.
The [journal identity proof](../../plans/proof/starport-production-catalog/csp5/journal-identity-2026-09-12/verification.json) preserves six original failures and their corrections.
Both paths previously deleted identical replacement files, JSON whitespace edits, and journals with changed access metadata.

Publication returns the staged file state. Recovery reads exact bytes, native identity, and access metadata from one bounded file read.
Completion requires that state before removal. Cancellation preserves the journal, and a later attempt validates it again.
All 226 workspace and migration CLI race test events pass. ago, prose, and Linux/Windows compilation checks pass.

A separate Go overlay proves that backup recovery still deletes an identical replacement child.
The version 2 journal records the backup root identity and child content and access metadata. It does not persist native child identities.
The next section records its correction and existing-journal compatibility.
The passing completion tests do not resolve this separate failure or qualify native execution, full CSP5 acceptance, or a merge.

## Persisted replacement child ownership: 2026-09-12

Starmap `e7bfdf67` persists native child identities in version 3 replacement journals.
The [child identity proof](../../plans/proof/starport-production-catalog/csp5/backup-children-2026-09-12/verification.json) preserves original file and directory deletion failures.
The old recovery also accepted version 2 journals without child identities. The new reader preserves version 1 and 2 journals for explicit recovery.

Recovery compares persisted identities before moving live or candidate trees and deleting backup children.
Cleanup checks the root path binding before each child removal. Tests preserve identical replacements and a file replaced during cleanup.
They also refuse invalid identity inventories before workspace changes. Each map must cover its exact inventory, within the existing journal limit.

All 245 workspace and migration CLI race test events pass without skips. Process-exit recovery passes at six replacement phases.
ago, prose, and Linux/Windows compilation checks pass. Native execution and full CSP5 qualification remain open.
Preparation ownership still lives in memory. Persistent preparation and relocation recovery remain the next work, with retention and compaction.

## Workspace writer binding: 2026-09-12

The [writer identity proof](../../plans/proof/starport-production-catalog/csp5/workspace-writer-2026-09-12/verification.json) records a prerequisite for persistent preparation recovery.
Both native-directory and journal publication previously continued after another process acquired a replacement writer lock.
Starmap `092bf7ce` now retains a checked lock handle, path binding, and native identity through workspace operations.

Publication, directory moves, backup cleanup, and journal completion recheck the held writer. Version 3 journals also require `lock_identity`.
Recovery preserves records when the current lock differs. Tests replace the lock at all six journal phases and verify that no later changes occur.

All 257 workspace and migration CLI race test events pass without skips. ago and the final prose check pass.
Linux and Windows test binaries compile. Windows can refuse to rename the open lock. Its conditional test path remains unqualified until native CI.

Preparation ownership remains in memory. The next step must bind a durable preparation journal to the checked writer identity.
Legacy relocation recovery, other staging, retention, compaction, full CSP5 acceptance, review, and merge remain open.

## Private preparation recovery: 2026-09-12

Starmap `873a53c5` persists private workspace preparation records in `.preparation.jsonl`.
Records bind the target, enclosure, journal identity, writer-lock identity, and each entry's identity, contents, and access settings.
Projection and repair recover unchanged entries under the checked writer lease. Unknown files, changed entries, and invalid journals remain preserved.

The [preparation recovery proof](../../plans/proof/starport-production-catalog/csp5/preparation-journal-2026-09-12/verification.json) records 278 passing race test events.
Process-exit tests cover initialized staging, partial writes, and completed render.
Other tests cover active writers, changed files, replaced journals and locks, malformed records, cancellation, partial cleanup, and bounded scans.

Journals have a 32 MiB limit, a 120,000-event limit, and at most four trees. The parent scan stops at 4,096 entries before cleanup.
Linux and Windows test binaries compile. Native execution remains unqualified until the required platform checks pass.

Candidate handoff, temporary records, legacy relocation, other staging, retention, compaction, and full CSP5 qualification remain open.
A process exit without a complete receipt preserves uncertain state, including an empty enclosure after journal removal.

## Candidate handoff and recovery: 2026-09-12

Starmap `042b261e` retains preparation journals through candidate publication and cleanup.
Version 2 handoff records bind the candidate path, prepared inventory, original workspace inventory, and native child identities.
Version 1 journals remain readable for private preparation cleanup only.

Publication verifies the original journal receipt and complete candidate inventory before changing the workspace.
The replacement protocol repeats the candidate check before recording its ownership.
Changed bytes, identities, or journal receipts prevent publication. Recovery preserves changed files and never selects the live workspace for cleanup.

Workspace recovery settles replacement journals before collecting preparation and candidate state.
The [candidate recovery proof](../../plans/proof/starport-production-catalog/csp5/candidate-journal-2026-09-12/verification.json) records 314 passing race test events.
Eight process-exit cases cover first installation, replacement, prepared and installed states, and four replacement-journal phases.
Other checks cover invalid handoffs, partial cleanup, changed files, active writers, and version compatibility.

The proof preserves the original test-method error, corrected crash failures, changed-ownership failures, and one recovery integration failure.
Final policy, prose, and compilation checks pass. Native Linux and Windows execution remains subject to the required platform checks.

A concurrent goago migration remains outside this catalog commit. Its additional policy check passes, and the proof preserves both tooling states.

Unknown enclosures after journal removal remain preserved. Temporary records, legacy relocation, other staging, retention, compaction, and full CSP5 qualification remain open.

## Temporary publication record recovery: 2026-09-12

Starmap `2a4454fd` adds durable ownership for temporary projection markers and replacement journals.
Version 3 preparation receipts bind each temporary file's destination, basename, native identity, contents, size, and access metadata.
The publisher records empty files and returned partial or complete writes. It checks the original receipt before publication.

Recovery removes only the recorded temporary name when its identity, contents, and access still match.
It preserves unknown files, changed state, and incomplete receipts. A missing temporary file permits journal cleanup to resume.
Cleanup does not select the published destination. Older preparation and candidate journals retain their original contracts.

The [temporary-record proof](../../plans/proof/starport-production-catalog/csp5/record-journal-2026-09-12/verification.json) preserves four original crash failures.
The expanded tests cover twelve process-exit scenarios, changed ownership, malformed receipts, interrupted cleanup, and version compatibility.
The focused race run passes 80 events. The full suite passes 347 race test events without failures or skips.

Policy, prose, and Linux/Windows compilation checks pass. Native execution remains subject to the required platform checks.
The proof archives the concurrent goago module inputs, which remain outside the catalog commit.
Legacy relocation, other staging, retention, compaction, and full CSP5 qualification remain open.

## Legacy relocation recovery: 2026-09-12

Starmap `57bb6d17` records legacy relocation before moving the catalog store.
Version 4 preparation receipts bind both parents, store metadata, retained generations, workspace entries, and the optional Windows lock alias.
Explicit migration retry checks those receipts and both advisory locks before restoring the store and retrying migration.
Ordinary projection and repair refuse pending relocation. Changed files, unknown entries, and incomplete ownership remain preserved.

The [relocation proof](../../plans/proof/starport-production-catalog/csp5/relocation-journal-2026-09-12/verification.json) preserves three original crash failures.
Five process-exit scenarios and separate ownership, cancellation, writer, scan-limit, alias, and rollback checks cover recovery.
The full suite passes 384 race test events without failures or skips. Go policy, prose, and Linux/Windows compilation checks pass.
Native execution remains subject to the required platform checks. The proof archives the concurrent tooling inputs outside the catalog commit.

A completed migration removes its journal. Repeating that completed command reports the existing destination.
Unrecorded enclosures and incomplete receipts remain preserved for explicit recovery.
Evidence and discovery staging, retention, compaction, and full CSP5 qualification remain open.

## Private record recovery API: 2026-09-12

Starmap `9f82730e` adds an opt-in private record publication and recovery API.
The writer records native identity, access, mode, timestamp, size, and digest before publication.
A private `.record-publications` directory holds its stable writer lock and bounded JSONL receipts.
Recovery excludes active writers, preserves changed or unrecognized files, and never deletes accepted destinations.

The [private record proof](../../plans/proof/starport-production-catalog/csp5/private-record-recovery-2026-09-12/verification.json) preserves the original process-exit failure.
Nine process-exit scenarios and separate ownership, cancellation, scan-limit, flush, and retry checks cover the new API.
All 502 final race test events pass without failures or skips. Policy, prose, dependency, and Linux/Windows compilation checks pass.
Native Linux and Windows execution remains unverified. The proof archives concurrent tooling inputs outside the catalog commit.

At that API checkpoint, runtime evidence and GitHub discovery still used the ordinary writer. The next section records their integration.
Integration must preserve passive inspection, migration receipts, canonical file reporting, and replay floors across source instances.
Generation retention, history compaction, and full task qualification also remain open.

## Runtime record integration: 2026-09-12

Local Starmap commit `d0009a18` corrects three reproduced integration failures under CSP5.
Startup previously left abandoned runtime record stages. Concurrent GitHub source instances could replace a newer replay floor with an older result.
Migration accepted receipts tied to native identities from the original directory. The first correction also claimed unrelated directories with the reserved metadata name.

The final implementation recovers before retained-state startup and compares discovery state under the writer lock.
Migration refuses pending receipts only at declared product paths. Three unrelated directory layouts retain their contents through migration staging.
Passive file inspection reports the new private files and preserves their bytes.

The [integration proof](../../plans/proof/starport-production-catalog/csp5/record-integration-checkpoint-2026-09-12/verification.json) preserves the fixture errors, corrected failures, and interrupted broad run.
Expanded checks pass 19 events. Separate migration and file/retry checks each pass five events. The counts overlap.
Policy, prose, dependency, and compilation checks pass. The corrected broad race suite remains active, and CSP5 remains incomplete.

GitHub construction currently calls recovery with `context.Background()`. CSP5 must verify caller cancellation through that constructor before final qualification.

The broad suite found a privacy fixture that treated the new recovery directory as a file. Follow-up `989310c7` scans all nested files.
It preserves the diagnostic-sentinel assertion. Its focused race test, policy check, and Linux/Windows compilation pass. The active broad binary still contains the original fixture.

## Discovery construction cancellation: 2026-09-12

Local Starmap commit `91b4fbc6` passes the runtime caller context through GitHub construction and local record recovery.
Cancellation before construction creates no discovery state. Cancellation during recovery preserves accepted records and unfinished receipts for a later retry.
The legacy constructor retains its background context. Runtime cleanup releases ownership after failed construction.

The [cancellation proof](../../plans/proof/starport-production-catalog/csp5/discovery-context-2026-09-12/verification.json) records 153 passing race test events and five original failures.
Policy, prose, dependency, and Linux/Windows compilation checks pass. Native execution and full CSP5 qualification remain open.

## Repeated provider history: 2026-09-12

Local Starmap commit `cdcf4fff` compacts repeated provider inventories before a new acquisition exceeds the retained history limit.
It retains the first and latest successful observations for each identical payload and complete binding declaration.
Those original receipts preserve model change times and current evidence. Distinct inventories preserve omitted offerings.
Partial observations and equal-time evidence also remain intact.

The [history proof](../../plans/proof/starport-production-catalog/csp5/repeated-history-2026-09-12/verification.json) records 57 passing race test events, including both capacity limits and reload.
It preserves the original capacity failure, fixture errors, and a corrected timestamp defect. Policy, prose, and Linux/Windows compilation checks pass.
The earlier broad suite ended with a known fixture failure and runtime timeout. Full task qualification remains open.

Metadata histories and reset operations retain their existing boundaries and still require compaction support.
Immutable observation files and catalog generations remain subject to the separate collection contract.
CSP5 retains these requirements and its complete acceptance scope.

## Bounded history checkpoints: 2026-09-12

Local Starmap commit `964f1c36` adds version 4 checkpoints for metadata, provider, and mixed histories with ordered resets.
Checkpoints share distinct payload bytes and retain original receipts, batch order, and reset scopes.
Each checkpoint permits 64 MiB encoded data and 65,536 ordered observation references. Later linked records also count toward the history byte limit.
Versions 1, 2, and 3 remain readable. Older readers reject version 4 heads.

The [checkpoint proof](../../plans/proof/starport-production-catalog/csp5/checkpoint-history-2026-09-12/verification.json) records 166 passing race test events across two disjoint selections.
Cases cover both history limits, mixed replay, replacement baselines, reset exclusions, later appends, invalid records, and publication recovery.
Policy, prose, and Linux/Windows compilation pass. Native execution and full task qualification remain open.

Checkpoints do not retire distinct superseded inventories or collect predecessor files and catalog generations.
Required distinct data can still exceed capacity, which preserves the accepted catalog and returns a conflict.
CSP5 retains safe compaction, collection, and its complete acceptance contract.

## Catalog serialization and immutable provenance: 2026-09-12

Local Starmap commit `c8ec797a` implements the following changes.
Classification: in-scope correctness and measured update cost under CSP5.
Allocation checks reproduced redundant normalization and repeated payload encoding. Mutation checks reproduced shared nested values in immutable provenance.
The initial snapshot change exposed typed YAML comparisons that lost original reasoning and verbosity receipts after JSON normalization.
Canonical JSON comparison repairs the mismatch while preserving existing field-scoped YAML aliases. Payload and workspace regressions pass.

The [runtime cost proof](../../plans/proof/starport-production-catalog/csp5/runtime-cost-2026-09-12/verification.json) records 2,120 passing race test events across four packages.
Policy and prose checks pass. The proof preserves the original failures and all intermediate source snapshots.
The import/restart profiles estimate 6,092,786,599 allocated bytes before the change and 5,066,953,937 bytes afterward.
These sampled totals are cumulative allocation, not resident memory or Starport request overhead.

Distinct-inventory retirement, collection, full task verification, required review, native CI, and merge remain open.

## Explicit memory retention: 2026-09-12

Local Starmap commit `78102bca` implements the following changes.

Classification: in-scope retention under CSP5.
Two initial tests failed because Memory lacked the new optional retention capability.
The implementation now protects current content, caller-required generations, and independent read leases during collection.
Race tests cover concurrent publication and lease release. Persistent adapters and runtime adoption remain incomplete.

The [retention proof](../../plans/proof/starport-production-catalog/csp5/memory-retention-2026-09-12/verification.json) records 112 passing storage race test events on each Go toolchain.
These macOS runs repeat the same cases on Go 1.26.6 and 1.25.12. Policy, package lint, and prose pass.
The proof preserves the original failures and exact tested source inputs.

Persistent generation collection, observation-file collection, distinct-inventory retirement, full task verification, required review, native CI, and merge remain open.

## Recoverable filesystem retention: 2026-09-12

Local Starmap commit `199cdaaf` implements the following changes.

Classification: in-scope persistent retention under CSP5.
The initial tests reproduce the absent filesystem retention capability.
The implementation adds native read leases, bounded scans, checked directory retirement, and recoverable cleanup.
The existing private-record publisher owns retirement journal staging. Smaller publication functions clear the repository lint diagnostics without changing their contracts.

The [retention proof](../../plans/proof/starport-production-catalog/csp5/filesystem-retention-2026-09-12/verification.json) records 250 passing race test events on each Go toolchain.
These macOS runs repeat the same storage and private-file cases on Go 1.26.6 and 1.25.12.
Policy, package lint, and prose pass. Linux and Windows test binaries compile. Native platform execution remains unverified for this change.

Object storage, runtime adoption, observation-file collection, history retirement, full task verification, required review, native CI, and merge remain open.

Follow-up commit `b10721b8` makes empty filesystem collection succeed and normalizes missing generation and requirement errors.
A nonempty expected head conflicts with an absent store.
The [empty-store proof](../../plans/proof/starport-production-catalog/csp5/filesystem-empty-store-2026-09-12/verification.json) records 257 passing race test events on each Go toolchain.
It preserves the original four failing subcases, their failing parent, and the direct probe. Existing qualification limits remain unchanged.

## Object collection backend operations: 2026-09-12

Classification: in-scope storage capability work under CSP5.
Local Starmap commit `8f76544d` closes the missing inventory and conditional-deletion primitives in memory and S3 backends.
The [backend proof](../../plans/proof/starport-production-catalog/csp5/object-collection-2026-09-12/verification.json) preserves three original capability failures.
It records 223 passing storage race events on each Go toolchain, plus passing lint, policy, and prose checks.

The object catalog store still lacks coordinated generation collection. Backend primitives alone cannot establish that contract.
Live S3 compatibility and native platform execution remain unverified for the new operations. Runtime adoption and full CSP5 qualification remain open.

## Runtime pin retention: 2026-09-12

Classification: in-scope retention defect and read-policy regression under CSP5.
Local Starmap commit `03590139` protects the original pin selection after origin publication changes the current generation ID.
The [pin proof](../../plans/proof/starport-production-catalog/csp5/pin-retention-2026-09-12/verification.json) preserves the original collection failure.
It also preserves the first implementation's configured-read bypass and its correction.

Both final pin selections pass 62 race events. Both recovery selections pass 240 events. These counts overlap and repeat across toolchains.
Lint repairs preserve migration and checkpoint behavior. Policy, lint, dependency gates, and prose pass.
Object retirement, runtime collection, history retirement, and full CSP5 qualification remain open.

## Checked private record removal: 2026-09-12

Classification: missing local collection capability under CSP5.
Local Starmap commit `98961b9f` adds checked removal that shares ownership with private record publication.
The original test fails because that operation is absent. Automatic runtime deletion remains incomplete.

The object-catalog coordination question remains pending. The documented fleet recipe assigns coordination to shared KV and immutable bytes to S3.
No accepted architecture or scope changed. Local collection work proceeds independently.

The [removal proof](../../plans/proof/starport-production-catalog/csp5/record-removal-2026-09-12/verification.json) records 116 passing private-file race events on each Go toolchain.
Policy, package lint, and source prose pass. Runtime collection and full CSP5 qualification remain open.

## Runtime input collection: 2026-09-12

Classification: local runtime input collection capability added under CSP5.
Local Starmap commit `bbd01f43` collects validated unreachable inputs while preserving accepted and pending references.
The original test fails because this operation is absent. The final checks cover legacy bytes, checkpoints, limits, partial cleanup, and shutdown.

The object coordination question remains pending. No accepted architecture or scope changed.
Distinct-inventory retirement and automatic collection remain open.

The [collection proof](../../plans/proof/starport-production-catalog/csp5/input-collection-2026-09-12/verification.json) records 42 passing race events on each Go toolchain.
Policy, package lint, and source prose pass. Full CSP5 qualification remains open.

## Current provider review evidence: 2026-09-12

Classification: an in-scope compaction prerequisite under CSP5.
Twelve observations previously created twelve reviews for one unresolved offering.
Repeated-history compaction reduced that set to two after baseline replacement, changing generation evidence despite equal payload checksums.
Local Starmap commit `8f8951bf` retains one current review per offering and binding revision, using original receipts.

The valid baseline records three failing test events and one passing event before the fix.
Two earlier fixture attempts remain archived as inconclusive. They do not prove the behavioral defect.
Task criteria, scope, accepted architecture, and the goal remain unchanged.

The [review retention proof](../../plans/proof/starport-production-catalog/csp5/review-retention-2026-09-12/verification.json) records 74 passing race events on each Go toolchain.
Policy, package lint, and source prose pass. Full CSP5 qualification remains open.

## Configured controls and qualification scope: 2026-09-12

Classification: required CSP5 verification and evidence scope.
The first task invocation finds twelve unregistered subcases. Local Starmap commit `2af77d76` connects audited runtime checks and new background-control tests.
A trial registration exposed a qualification boundary: producer tests could complete consumer cases.

The final verifier confines eight producer bindings to CSP5. Five new boundary tests fail before that repair and pass afterward.
The final task command passes twelve component subcases. A22 and A23 remain unverified for consumer qualification.
Package lint also required a descriptor helper. All 39 descriptor records retain identical output.

The [component controls proof](../../plans/proof/starport-production-catalog/csp5/component-controls-2026-09-12/verification.json) records 42 passing task race events and 78 passing verifier tests.
Policy, package lint, and source prose pass. Complete CSP5 implementation and qualification remain open.

## Provider history retirement: 2026-09-12

Classification: an in-scope CSP5 retention correction.
The distinct-inventory capacity regression fails before the repair. A later multi-account test shows a 00:13 model timestamp changing to 00:14 after retirement.
The original repeated-inventory policy also fails through an isolated overlay. First and latest reports alone do not preserve cross-account change times.

Local Starmap commit `292fe261` retains the reports that establish those transitions.
The final selection passes 152 race events on each supported Go toolchain. Initial fixture and compilation failures remain separate from product evidence.
Incoming-reset capacity, automatic collection, and complete CSP5 qualification remain open.

The [retirement proof](../../plans/proof/starport-production-catalog/csp5/provider-retirement-2026-09-12/verification.json) records source identities, commands, failures, and final results.
Full CSP5 verification, review, native CI, and merge remain open.
## Starport catalog retention boundary: 2026-09-13

Classification: a verified consumer gap under CSP8, alongside CSP5 Starmap retention.
The [storage probe](../../plans/proof/starport-production-catalog/csp5/shared-storage-boundary-2026-09-13.md) uses the unchanged Starport adapter with actual in-memory Badger.
After 33 accepted generations, its history contains 32 entries while all 33 generation descriptors and 33 payload chunks remain stored.
The oldest generation remains readable. The adapter exposes neither collection nor generation read leases.

Starport stores these catalog records in KV. Its object-storage backend owns uploaded file bytes.
Starmap's optional object catalog adapter has a separate collection boundary.
CSP8 must protect accepted and candidate heads, readers, shared chunks, and pending writes during retention.
The passing diagnostic supplies no acceptance credit. Valkey and concurrent deletion remain unverified.

## Scheduled publication recovery implementation, 2026-09-14

Starmap commit `96127f40aad4417516d4afbbf890c2ba205d4306` connects public checkpoint recovery, pending publication, checked promotion, and channel advancement.
The [workflow proof](../../plans/proof/starport-production-catalog/csp6/workflow-recovery-2026-09-14/verification.json) owns source hashes, test counts, preserved failures, and qualification limits.
It also records the main-branch protection response and the pinned GitHub App token action.

The public checkpoint stores public model data without encryption or a private object store.
The dedicated App key authenticates GitHub PR operations. Provider API keys remain confined to acquisition.
The implementation restores the existing candidate checks, budget report, and optional OCI mirror.

Recovery tests use real Git repositories and catalog tools with simulated GitHub responses.
The full-catalog draft and the smaller normalized fixture both pass their recorded checks.
Fixture responses do not qualify App installation, actual workflow triggering, remote provenance, or hosted promotion.

CSP6 remains incomplete until mixed authored-baseline selection, public-profile capacity, channel defaults, full checks, required review, and merge complete.
The canonical plan's current resume state owns the next action.

## Authored publication baseline implementation, 2026-09-14

Starmap commit `1b8633769ff3011c91a83984fbeac6a81c2b86b3` separates authored edits from unchanged acquired results.
The [authored baseline proof](../../plans/proof/starport-production-catalog/csp6/authored-baseline-2026-09-14/verification.json) records seven files, source hashes, preserved failures, and final qualification.
The changes retain explicit edits, additions, removals, required identities, and alias targets across checkpoint restore.
An explicit source removal can still discard acquired limits, scopes, and unresolved reviews.

The final Go 1.26.6 checks cover three complete packages through disjoint selections, with 164 passing race test events.
The minimum Go version passes twenty focused events. All ten controller recovery tests pass.
Policy and strict prose pass. An initial-baseline identity regression failed the first full CLI check and passes after repair.
The proof preserves that failure and distinguishes invalid fixtures from valid behavior failures.

This commit remains local. Public-profile capacity, channel defaults, full CSP6 verification, required review, hosted qualification, and merge remain open.


## Public record quality and channel migration, 2026-09-14

The captured models.dev input contains four Novita display names ending in a tab.
The original adapter quarantined them and accepted 5,894 records. Required complete-source admission then rejected the whole publication.
The owner approved isolated record quarantine and required visible status and repair evidence.

The adapter trims surrounding display-name whitespace. It preserves model IDs, internal control-character checks, and exact original source bytes.
HTTP and Git regression tests cover all four names and their structured correction events.
One adapter rule handles all four records. YAML controls quarantine admission and retention policy, without model-specific repair entries.

The publication policy permits valid siblings while preserving rejected records' prior facts.
Receipts retain degraded source status, counts, identifiers, and reason codes. Recovery clears current quarantine without changing historical receipts.

Commit `0ce46cd77f62436a98e7b64211e1885b4591054e` contains these local changes in the CSP6 implementation worktree.
The new channel default is v2 with explicit v1 compatibility. A captured signed v1 catalog survives reopening with unavailable v2 updates.
The initial complete-source and workflow checks exposed failures that the qualification record preserves.
The full runtime and full-profile race commands exceeded their default ten-minute deadlines. Neither timeout qualifies as a passing check.

The profiled capacity run passes eight simulated runs, including an outage and checkpoint recovery.
Its retained history stabilizes at 26 observations. The largest checkpoint measures 28,162,293 bytes.

The full runtime suite passes 1,192 race events with an explicit thirty-minute timeout.
Final publication/artifact checks pass 264 race events. The source suite passes 80, and minimum Go passes 28.
Generation and all 68 distribution checks pass. Sol and Opus pass required review at P0.
[Draft PR #160](https://github.com/agentstation/starmap/pull/160) contains the integrated flow and native timeout adjustment.
Native CI, hosted qualification, and merge remain open.
The canonical plan's current resume state owns exact active commands and next actions.

## Acquisition policy and Go composition, 2026-09-14

The local CSP7 implementation adds persistent credential selection history and a public acquisition resolver factory.
Fresh standalone startup records current precedence before catalog writes. Legacy installations compare complete selections before accepting each provider migration.
The first application regression exposed a policy write during passive construction. The repair keeps passive catalog access free of that write.

A separate regression exposed expiry during policy acceptance. The resolver now checks material again before returning it.

`acquisition.OpenCredentialResolver` supports explicit source references and optional private policy storage.
It reads no credential source during construction. The embedding host owns installation classification and state selection.
The factory keeps Starmap's ambient order. Starport role integration remains under CSP9.

The [CSP7 proof](../../plans/proof/starport-production-catalog/csp7/policy-composition-2026-09-14/verification.json) records 714 package race events and 226 final task race events.
Minimum Go passes 38 focused events. Ninety verifier tests and six producer component cases pass.
The final component cases contain 76 passing test events. They do not qualify product cases A11 or A12.

The initial full repository run passes 6,834 events, fails one fixture, and skips eight.
The fixture sets an empty variable while expecting absence. Its repair retains the missing-credential assertion with an absent variable.
Three further regressions cover selected origins, empty credentials, and expired credentials.
Both supported Go toolchains pass all 124 auth and table race events after repair.

Final pinned package lint, Go policy, and strict prose pass. The frozen full repository suite and required publication review remain open.
The canonical plan owns the current source and running commands.


## Draft publication recovery review, 2026-09-14

A required publication review found that REST lookup by tag cannot locate draft releases.
The original test fixture returned drafts and concealed the failure. The corrected fixture reproduces the recovery failure.
The repair uses GitHub CLI's draft lookup strategy and verifies the selected release ID before reuse.
Published release verification still uses the public tag lookup.

Commit `567691400acdce5b96710ffdf0150f7939d4ab3f` contains the repair. Thirteen publisher and transport tests report success.
The [repair proof](../../plans/proof/starport-production-catalog/csp6/windows-job-budget-2026-09-14/verification.json) retains the accepted finding, failed regression, and current review state.
Hosted publication remains unverified and requires the pending owner authorization.


## Credential PR qualification, 2026-09-14

[PR #161](https://github.com/agentstation/starmap/pull/161) contains the integrated Starmap credential policy and public acquisition resolver.
Implementation commit `4ef4d2ad4` precedes integration commit `d46411958`, which includes the merged S3 module update.
The full repository suite passes 6,838 events with eight explicit skips before the module update.
Final auth, acquisition, and S3 suites pass 268 race events after integration.

Six producer subcases, external consumers, generated documentation, strict prose, lint, and Go policy pass.
Sol and Opus report no actionable findings in the required review. The PR uses guarded auto-merge and awaits native CI.
The [credential proof](../../plans/proof/starport-production-catalog/csp7/policy-composition-2026-09-14/verification.json) owns source hashes, preserved failures, and exact results.
CSP7 completed its Starmap contract through PR #161. Product cases A11 and A12 still require Starport integration.

## Scheduled correction reports and owner decisions, 2026-09-14

The publisher discarded acquisition stderr after parsing its success result. That discarded the four models.dev correction events.
Commit `3da87515ecb90dd08f49dee8a27d1935d98b5dcc` retains recognized correction metadata and adds process outcome and counts to the Actions job summary.
The report excludes raw messages and unknown fields. Failed acquisition and timeout paths retain complete events without staging a publication.

The [diagnostic proof](../../plans/proof/starport-production-catalog/csp6/acquisition-diagnostics-2026-09-14/verification.json) preserves four failing tests and one timeout error before the change.
All six focused tests and all nineteen publisher tests pass after the change. The catalog-generation gate passes six Go packages and the provider-fixture contract.
Required review, integration with current main, and native qualification remain open for this commit.

The owner approved Valkey or Redis coordination for shared S3 cleanup. The owner also authorized App setup and checked public catalog publication.
Public provider acquisition remains best effort with retained facts and visible failures. D35 resolves the models.dev freshness choice on 2026-09-16 UTC.
The private publisher App registration and client ID variable now exist. Private-key generation, installation, secret storage, and hosted publication remain incomplete.

## Coordinated object retention, 2026-09-15

Commit `03e2cb570480766fccb20c640b56ad661084b591` adds object publication, reader protection, and bounded collection through a separate Valkey or Redis record.
A regression exposed a retained reader claim after an accepted write lost its response and the next read failed.
Failed acquisition now attempts cleanup with an independent timeout. Persistent coordination failure can still require fenced owner recovery.

The [current proof](../../plans/proof/starport-production-catalog/csp5/coordinated-store-2026-09-15/verification.json) records 242 storage and workflow events on each Go toolchain.
Valkey and Redis each pass fifteen native cases per toolchain, including process exit, service restart, concurrent publication, and near-limit payload transfer.
The helper process case skips in the parent suite and executes twice in child processes.

Classification: in-scope CSP5 storage implementation. Shared runtime references, full product qualification, required review, native CI, and merge remain open.
The fixed object-service fixture does not establish maintained production-service support.

## Starport setup across roots, 2026-09-16

Commit `066cacffb9d0bed794a07d31041034ec416b480d` replaces the grouped-directory setup transaction with publication at the selected configuration and Badger paths.
It uses Starmap `productfiles` at `486510bc5` for native private-file operations. The change removes the old Starport rename and sync adapters.
The temporary Go workspace supplies this unpublished dependency. The branch contains no module replacement.

Ten process-exit boundaries recover through a new public initialization call.
The database binding blocks runtime startup after an interrupted setup, including Go configuration without a primary file path.
The runtime holds its local storage guard until its stores close.
Changed files, malformed journals, and conflicting restore destinations remain preserved.

The [setup proof](../../plans/proof/starport-production-catalog/csp8/setup-recovery-2026-09-16/verification.json) records 475 passing race events across five packages and one Valkey skip.
Lint reports zero issues. Vet, strict prose, and Linux/Windows cross-compilation pass. Native Linux and Windows execution remains unverified.
The proof preserves intermediate application failures and all nineteen initial lint findings.

Classification: in-scope CSP8 implementation. Development isolation, migration, complete diagnostics, storage, acquisition, and release gates remain open.
Required review, native CI, a compatible published Starmap module, and merge still gate completion.

## Starport development scratch ownership, 2026-09-16

Commit `3a2fd6b873235c09d847b93fd1b5bd602a301fa6` replaces repeated recursive path deletion with session ownership and bounded recovery.
The original repeated-close regression deletes an operator directory that reuses the former scratch path.
Another regression shows that failed composition deletes scratch before the caller can establish complete resource shutdown.

The host now publishes its recovery record after application construction succeeds.
Native identities, private access checks, exact record bytes, and a lifetime lock govern cleanup.
Recovery preserves changed state and incomplete initialization. A process-kill test proves live exclusion and later recovery.

The [scratch proof](../../plans/proof/starport-production-catalog/csp8/scratch-recovery-2026-09-16/verification.json) records 526 passing race events across configuration, catalog, and application packages.
The optional Valkey test skips. Lint, vet, authored prose, and documentation links pass.
Native Linux and Windows execution remains unverified. The local workspace still supplies the unpublished Starmap dependency.

Classification: in-scope CSP8 implementation. Migration, complete diagnostics, storage, acquisition, required review, native qualification, and dependency publication remain open.
No task receives completion credit before its acceptance checks and implementation merge.

## Starport metadata collector and source cache, 2026-09-16

The previous Starport composition supplied only a provider collector. A selected models.dev source reached no collector during explicit refresh.
It also omitted every collector with `ACQUISITION_ENABLED=false`, which prevented the independent manual behavior required by the specification.

Commit `546cd6cc9208082174d23b01f90e111cf747f594` supplies both passive collectors and leaves scheduling, network access, and authority checks with Starmap.
The application passes Starport's cache root. Development passes its private session cache.
File diagnostics use canonical source selection and shared file-role access policy.

The [acquisition proof](../../plans/proof/starport-production-catalog/csp8/source-acquisition-2026-09-16/verification.json) records 646 passing race events and one optional Valkey skip.
All twelve ownership checks pass against the local dependency workspace.
A controlled HTTP response reaches the real collector, creates the selected cache, and retains source observation links through acceptance and offline restart.
The limited fixture omits prior models. Its degraded source status and retained baseline models confirm that those omissions do not delete baseline entries.

The restart test reuses a test KV adapter. It does not qualify database durability or fleet recovery.
Native execution, real Git integration, legacy migration, remaining authoring artifacts, and published-dependency qualification remain open under CSP8.

## Starport upgrade path conflicts, 2026-09-16

Commit `e39c77e55b11f7c9b267c6e4ba5da41f5723cc18` prevents implicit path changes from abandoning recognized prior state.
The failing regression opened new Badger, SQLite, and blob stores while an old database remained under the configuration directory.
Startup and initialization now refuse before these writes. Explicit selectors remain authoritative.

The [legacy path proof](../../plans/proof/starport-production-catalog/csp8/legacy-paths-2026-09-16/verification.json) records 551 passing race events and one optional Valkey skip.
Final source passes 67 focused events. Diagnostics report previous locations without opening databases or reading their contents.
Runtime migration, completed-receipt adoption, and native qualification remain open under CSP8.

## Shared workspace file inventory, 2026-09-16

Starmap `e8104b5f3b65058da3f05f648259d29ae139b5d8` exposes the canonical workspace inventory through `productpaths.WorkspaceFiles`.
Starport `32f40b40afc85530e389b45a9ad8b018cf84a926` consumes it for adjacent receipts, journals, locks, staging, and backup files.
The [proof](../../plans/proof/starport-production-catalog/csp8/workspace-manifest-2026-09-16/verification.json) records 373 passing Starport race events and 92 minimum-Go Starmap events.
Inspection preserves selected origins and access classes. It excludes neighboring workspaces and file contents.
Full producer verification, native Starport inspection, dependency publication, required review, and merge remain open.

## Configured initialization upgrade guard, 2026-09-16

Starport `7b59be6ece0768871cd9b60ca556e9530941e05d` extends the upgrade check to `starport init --configured-storage`.
The failing regression created a gateway key and a new store while recognized legacy data remained.
The command now refuses before those writes. The old store remains unchanged.
The [proof](../../plans/proof/starport-production-catalog/csp8/legacy-paths-2026-09-16/configured-init-verification.json) records 21 passing race events and a clean command-package lint check.


## 2026-09-17: Promoted catalog exposes billing and protocol gaps

The prepared catalog replaces complete pricing records as the current authority contract requires.
This removes manually derived Google page prices when models.dev supplies a new token-price record.
`TestEveryRecognitionOfferingCanBeBilledByThePage` fails for `google-ai-studio/gemini-2.5-flash`.
The owner selected actual billing units and separately labeled estimates under D37.

Starport’s `internal/catalog/control_plane.go` admits recognition only with `Operations.PageInput`.
`internal/proxy/parser.go` uses page prices for admission and extraction cost.
The Google recognition connector already returns token usage. The repair must carry that evidence into accounting instead of substituting a page estimate.
Its cheapest-page admission estimate cannot prove that the selected route fits a required budget.

Google’s [document-processing documentation](https://ai.google.dev/gemini-api/docs/document-processing) describes PDF token reporting and model-dependent resolution controls.
A universal fixed page charge does not express that contract. This review checked the documentation on 2026-09-17.

Starmap’s `pkg/catalogs/offering_views.go` also excludes chat based on nonzero media-operation prices.
After refresh converts realtime audio pricing to token units, three realtime-only models incorrectly gain chat eligibility.
The models are `openai/gpt-realtime-2.1`, `google-ai-studio/gemini-3.1-flash-live-preview`, and `google-ai-studio/gemini-3.5-live-translate-preview`.
The existing residual-operation test detects this change. Keep its behavioral protection while replacing price-based capability inference with explicit service facts.

CSP6.2 owns both production contracts. CSP6 retains the failed publication evidence and must prepare a corrected catalog after those contracts pass.
No failed candidate may advance discovery channels or count as completed qualification.


## 2026-09-17: D37 local implementation and qualification

Starmap commit `8345fd30a` adds an independent recognition billing record and schema 10.
It preserves atomic price selection and uses explicit delivery protocols for realtime eligibility.
Eleven Google offerings now declare token billing. The repair removes their derived fixed page prices.

Starport commits through `606494c` retain measured usage, selected offering prices, and recognition charges across failed and streamed requests.
Its API and console expose billing units and label optional input estimates.
Google usage metadata preserves the distinction between missing and explicit zero values.
The operator guide records the incomplete reservation contract owned by CSP12.2.

Local evidence includes 124 billing race events against real Valkey, with no failures or skips.
The full Go suite passes 3,088 events and skips 40 optional cases.
The console passes all 459 tests with four workers. The proof also retains four failures from an unrestricted concurrent run.
The Starport checks use an ignored workspace with unpublished Starmap code.
These results provide no native CI, review, publication, or merge credit.

The [CSP6.2 proof](../../plans/proof/starport-production-catalog/csp6.2/verification.json) records commands, logs, hashes, and remaining work.


## 2026-09-19: CSP9 local credential qualification

Starport commits `aa346d9` and `9baa216` implement persisted inference selection policy and opt-in Starmap fallback.
The resolver rejects conflicting versions of one secret resource before reading material.
Rotation publishes complete profiles and retains usable prior material after an unavailable refresh.
Request-time cache reads do not read policy files or secret sources.

Starmap commit `99f9db801` registers all twelve A11/A12 subcases against the local paired worktrees.
All twelve pass. The verifier runs 65 distinct Go commands and reuses 17 duplicate references within that invocation.
Its 81 tests pass, including missing-test, skipped-test, and producer-versus-consumer evidence boundaries.

Real Valkey and PostgreSQL checks pass eight events without skips.
Secret-manager adapter tests use controlled SDK boundaries. Live cloud account access remains untested.
The [CSP9 qualification proof](../../plans/proof/starport-production-catalog/csp9/acceptance-registration.json) records commands and logs.

These results do not qualify a published pair. Pre-PR gates, structured review, native CI, and merge remain required.
Complete CSP6 and CSP8 before publishing this pair.


## 2026-09-19: CSP9 terminal credential failures

Starport commit `66738fc` clears cached inference material after denied, invalid, or removed source credentials.
Only temporary source failures retain valid prior material.
Commit `c7e1b9a` preserves denial diagnostics when runtime publication fails.
The regressions failed before both changes.

The full Go suite after `66738fc` passes 3,451 events with 48 optional skips.
The final provider, state, and application race run passes 161 events with two optional service skips.
The repository gates pass 32 of 33 checks against the preceding snapshot.
The remaining check requires the unpublished Starmap API in the committed module dependency.

Starmap commit `7961c8281` registers the additional diagnostic regression.
Its verifier suites pass 93 tests. Sol and Opus report zero findings for the producer branch.
The [repository review proof](../../plans/proof/starport-production-catalog/csp9/repository-review.json) records the exact source revisions and qualification limits.
Final acceptance passes all twelve CSP9 subcases.

Sol and Opus report zero consumer findings across three review chunks.
Published-pair checks, native CI, and merge remain required.


## September 19 operation-specific authorization clocks

Starport commit `81aa42a74ce28126ae19fb7f6127c3e9823e8080` removes storage-based clock selection from gateway authorization.
Local and shared stores use monotonic permission deadlines with a 60-second maximum.
Starmap external authority receipts retain their qualified-time contract.
The owner selected a two-second normal revocation-propagation target. Fleet qualification remains open.

The review reproduced receipt resurrection after clock recovery. Copies now share permanent invalidation after observed expiry or clock failure.
Corrected authorization race checks pass 103 events with four service-dependent skips.
Real Valkey/PostgreSQL checks pass ten events without skips. Readiness and startup application tests also pass.

The initial broad command failed an old test that checked an expired receipt at an earlier simulated time.
Its allocation assertion now runs before expiry. Independent clock cases use fresh receipts.

The operation-clock proof records exact test events and remaining checks.
Native suspend/resume, authority-specific diagnostics, full A46, review, and merge remain unverified.


## September 19 authorization recovery diagnostics

Starport commit `0b529f7` adds policy-authority status to admin info and preserves verified local-operator diagnostic reads during policy failure.
Inference and mutations still refuse. The two diagnostic reads no longer require inference budgets.
Revision polling now includes read latency within its fixed cadence. The two-second delayed-read component test passes.

Final server and authorization race checks pass 709 events with eight optional skips. The corrected application recovery test passes.
Native suspend/resume, real fleet propagation, full A46, review, and merge remain open.


## September 19 authorization source-record bounds

Starport commit `e56f49f9` bounds policy records before storage transfer and decoding.
KV adapters check size before copying or returning payloads. SQL projections suppress oversized records with byte-length checks.
Writes enforce the same 64 KiB record limit. Administrative policy writes report HTTP 413 when they exceed it.

The review rejected substring-based scalar bounds because SQLite truncates text at an embedded NUL.
Complete-value checks now preserve identifiers or refuse them. The scalar regression covers both outcomes.
Affected package race checks pass 884 events. Final controller checks pass 213 events, and final SQL checks pass 161 events.
Capacity, native timing, fleet enforcement, review, and merge remain open.

## September 19 authorization integration qualification

The local Starport candidate is `31feeca`, on `codex/starport-authorization-memory`.
Its paired workspace uses Starmap `152148130`. Neither this candidate nor the pair has release qualification.

The [capacity proof](../../plans/proof/starport-production-catalog/csp10.2/capacity.json) covers entry limits, encoded byte limits, concurrent loads, decoded heap, and shutdown.
The [fleet proof](../../plans/proof/starport-production-catalog/csp10.2/live-fleet-timing.json) covers separate-process observation and restart refusal with real Valkey and PostgreSQL.
The [partition proof](../../plans/proof/starport-production-catalog/csp10.2/network-partition.json) covers connection loss and withdrawal enforcement by the reachable authority.
The [expiry proof](../../plans/proof/starport-production-catalog/csp10.2/disconnected-expiry.json) covers the original 60-second lifetime when the proxy discards authority payloads.

Repository qualification found three integration defects. Architecture rules omitted the new revision and record-bound packages.
Setup rollback expected four records and refused the initial authorization marker. Usage fixtures expired after their fixed date exceeded retention.

The corrected architecture rules preserve dependency direction. Rollback now requires the initial valid marker and refuses missing, corrupt, or advanced revision state.
Usage fixtures use a current date without changing expected totals.

The [qualification record](../../plans/proof/starport-production-catalog/csp10.2/repository-qualification-progress.json) preserves failures and reruns.
Local checks pass: full Go tests, vet, lint, build, SDK smoke, and CGO-disabled first-run startup.
Focused repair checks pass 163 race-test events with 25 explicit skips. Nine A46 subcases have local evidence.

Opus 5 completed two review passes with zero findings at the configured P0 threshold. Linux and Windows pure-Go cross-builds also pass.
The published-module check still fails because the committed Starmap pin lacks required APIs.
Native suspend, clock discontinuities, multi-host qualification, full A46, and merge remain open.


## September 19 cache storage contract failures

The [cache proof](../../plans/proof/starport-production-catalog/csp12.1/cache-contract.md) records seven failing checks against Starport `31feeca` with real Badger.
Both cache implementations extend expiry after Get, GetMulti, and Warm refills. The default response cache also writes through the authoritative KV handle.
The test overlay leaves the reviewed implementation unchanged. The run has eight failed test events, including one parent, and no skips.

CSP12.1 owns the repair. Its cache interface must return a value with lifetime evidence for that same record version.
A separate GetTTL call cannot establish this under concurrent replacement. Transfer delay must consume the local deadline.
Cache-only service configuration, bounded fills, stream bounds, and discovery checks remain part of the task.


## September 19 default cache isolation

Starport `ba68306` separates the cache manager from the durable KV interface.
Default response and extraction caches now use process memory in application composition.
Extraction has separate cleanup and remains independent of the response-cache switch.
The [local isolation proof](../../plans/proof/starport-production-catalog/csp12.1/local-isolation.json) records 910 passing race-test events and seven explicit skips.
Vet, lint, the document-parser guard, and a pure-Go binary build pass.

Dedicated shared caching and original refill expiry remain open. CSP12.1 also owns bounded fills, stream limits, and discovery qualification.
The legacy layered and hybrid constructors remain outside application composition. Their recorded refill failures still require repair or replacement.


## September 19 refill lifetime repair

Starport `d651147` repairs the six recorded layered-cache and hybrid-cache expiry failures.
Badger and Valkey now return bounded payloads with atomic lifetime evidence. Refills retain a deadline anchored before the read.
Each local entry checks that deadline independently of cache queue timing. Successful backing writes invalidate local entries.

The [lifetime proof](../../plans/proof/starport-production-catalog/csp12.1/lifetime.json) records 254 passing race-test events and one manual restart skip.
All expiry and lifetime tests execute. Lint and vet pass.
Dedicated service configuration, optional workers, stream limits, and full cache qualification remain open.

## September 19 optional cache execution and shared service

Starport `eee04f9` moves response byte writes into bounded workers and exposes pressure counters.
Starport `a53776d` bounds stream retention while preserving delivery and cleanup.
The [stream proof](../../plans/proof/starport-production-catalog/csp12.1/stream-bounds.json) records 327 passing race-test events without skips.
Response encoding remains synchronous. These component checks do not establish full gateway latency or retained heap.

Starport `6fc0a2c` adds an explicit shared-cache endpoint, namespace, and separate connection owner.
The [shared-service proof](../../plans/proof/starport-production-catalog/csp12.1/shared-service.json) records real credentials, database selection, reconnect, expiry, and composition checks.
Scratch development rejects shared-cache settings. The admin response exposes connection availability without URI values.
The durable Valkey adapter remains unchanged and still requires the CSP12 URI and effective-option repair.

Custom-CA support, namespace ownership, model and extraction fills, capacity, latency, and discovery acceptance remain open.
No cache task completion or publication follows from these local commits.


## September 19 shared cache trust and coherence

Starport `51767ba` adds explicit CA roots, canonical file reporting, namespace probes, and diagnostic codes.
The [trust proof](../../plans/proof/starport-production-catalog/csp12.1/trust-and-access.json) records 70 final cache race events and one manual restart skip.
Nine targeted service events pass. Tests cover real Valkey behind TLS and preserve another namespace's record after denied reads and writes.

Two coherence repairs remain in CSP12.1. The independent cache namespace must derive from canonical deployment identity.
The layered, hybrid, and durable-KV cache adapters have no production callers and should leave the runtime source.
Preserve their historical evidence and verify the actual application paths before full cache acceptance.
Model and extraction fills and full performance, discovery, and production qualification remain open.


### Shared cache deployment identity repair

Starport `4e277db` removes the independent cache namespace setting.
A real Valkey regression proved that different canonical deployment IDs could previously share a cached response.
The cache now derives its encoded prefix from the canonical deployment ID and reports that prefix for ACL provisioning.
The [deployment identity proof](../../plans/proof/starport-production-catalog/csp12.1/deployment-identity.md) records 117 passing race events and the original failure.
Other shared stores and full cache qualification remain open.


### Cache serialization cost and endpoint reuse

Starport `d289fe3` repairs warm endpoint requests that rebuilt results after reading serialized cache records.
It also skips extraction inputs above 512 KiB before encoding, including all identity, text, and offering strings in the bound.
The caller still receives the document result. The cache reports the skipped fill.
The [encoding proof](../../plans/proof/starport-production-catalog/csp12.1/encoding-and-endpoints.md) preserves regressions and component measurements.

Oversized model serialization still costs about 2.6 ms before the queue drops the record. This remains an unresolved performance finding.


### Model encoding and typed discovery repair

Starport `7aae8da` bounds model encoding and removes the generic map round trip on discovery cache hits.
The earlier oversized model finding now has a passing allocation control and a lower-cost component measurement.
Typed decoding preserves large integer values. Named scalar types, JSON compatibility, pointer cycles, and queue capacity have regression coverage.
The [model encoding proof](../../plans/proof/starport-production-catalog/csp12.1/model-encoding.md) owns the results and their limits.
Aggregate heap, supported custom serialization methods, and full-request latency still need qualification.


## September 25 paid-operation admission audit

The [paid-operation matrix](PAID_OPERATION_MATRIX.md) covers eleven served operation names and their internal, streaming, and background callers.
The audit pins Starport source to `ca2d2057d60c07ad0bd6668023af8ae8647534c1`.
CSP12.2 owns implementation and A47 qualification. No matrix entry claims completed reservation support.

The source audit found a required accounting gap in `internal/jobs/accounting.go`.
`Service.settle` marks the job accounted before calling `RecordJob` and discards that call's error.
A later settlement skips the accounted job. Required budget settlement therefore needs its own recoverable transition.
The current job-slot reservation only bounds outstanding jobs. It does not reserve spend or token capacity.

`TestAFailedJobDrawsNoCost` and `TestACancelledJobDrawsNoCost` assert a false chargeable flag from terminal state alone.
That flag cannot prove no provider charge under the accepted reservation contract.
CSP12.2 must replace this assumption with per-attempt usage evidence or retained uncertain capacity.
Optional reporting failure cannot release capacity or close required reconciliation.

Video submission, poll, cancel, and content currently share `videos-generations`.
Admission needs the call purpose and billing evidence to distinguish new work from observation or retrieval.
Semantic-cache embeddings and guardrail moderation call the gateway internally and need independent child reservations.
The [audit record](../../plans/proof/starport-production-catalog/csp12.2/operation-inventory.json) binds these findings to exact source hashes.


### Production dispatch confirms the budget overrun

The [dispatch failure proof](../../plans/proof/starport-production-catalog/csp12.2/provider-dispatch-before.json) strengthens the earlier middleware-only evidence at Starport `ca2d2057`.
Two concurrent requests pass HTTP authentication, budget middleware, routing, execution, and the production OpenAI connector.
Both reach a barrier-controlled loopback provider before either response completes.
Both return HTTP 200. The real Badger usage repository records 1,202 tokens against the key's 1,000-token daily limit.
Each request permits 600 output tokens. No paid provider request occurs.

The race-enabled probe fails its provider-dispatch bound in 38.19 seconds without a data-race report.
The overlay changes only the test fixture. Application code and both source worktrees remain unchanged.
This proves the current local token-budget failure. It does not qualify reservation correctness, shared storage, other scopes, other operations, or recovery.
CSP12.2 must repair the production admission boundary and retain this failure evidence.


### Storage admission probe, September 26

The [storage contract proof](../../plans/proof/starport-production-catalog/csp12.2/storage-contract/verification.json) compares real Badger and Valkey at Starport `ca2d2057`.
The race-enabled overlay leaves product source unchanged.
Badger rejects a wrong expected value and a write that becomes stale before commit.
Valkey's `Transaction.CompareAndSwap` accepts both writes and overwrites the stored value.
Its implementation discards the expected argument.

No production caller currently uses `BeginTransaction`. The remaining caller is the repository test harness.

The separate `KVStore.CompareAndSwapBatch` passes the component probe on both backends.
Two concurrent candidates compete for the same account, key, and team records.
Exactly one succeeds. A stale team value rejects all three mutations without partial changes.
This probe uses one process and one Valkey node. Cluster, failover, crash recovery, and complete budget admission remain unqualified.

CSP12.2 must use a qualified atomic batch operation for its reservation ledger.
The current Valkey transaction API cannot provide that contract.
The usage repository also cannot serve as that ledger: `Put` updates each scope counter separately and repeated calls accumulate usage again.
Required settlement must retain its own idempotent state and recovery evidence.

### Deployment credential revocation, September 26

The [issued-material proof](../../plans/proof/starport-production-catalog/csp9/issued-material/verification.json) records a dispatch gap at Starport `527aac7c5`.
The resolver removed revoked material from its cache. Previously returned material remained valid while a request waited for a connection.
Regression tests reproduce this failure with explicit revocation and denied, invalid, or removed source material.
Both HTTP/1.1 and HTTP/2 tests sent the waiting request after revocation.

Local repair `b769aa5` binds deployment material to the existing in-memory revocation fence.
Terminal refresh failures, removal, explicit revocation, and rotation invalidate issued handles.
An unchanged refresh preserves each issued deadline. Transient failures retain material only within its existing validity.
The admitted stream still finishes. Waiting requests fail before dispatch.

The credential and connector suites pass 485 race test events, with one unrelated Valkey repository skip.
A separate real-Valkey run passes all 143 provider, authentication, and keyring test events without skips.
The warm credential lookup reports zero allocations in three runs. This does not measure complete gateway overhead.

The nine-pass review claimed that the standard `uuid` package does not exist. Go 1.27.1 and the compiled tests disprove that finding.
Final checks and the pre-PR gate remain required after the published Starmap module update.


### CSP11 native process checkpoint: September 26, 2026

Consumer `26d67ccf7` passes one concurrent-process recovery scenario.
It preserves partial acquisition inputs after leader termination and directory loss.
It also restores a restarted follower through durable polling without notifications.
The test explicitly expires the dead leader lease to avoid a 90-second wait.
This proves catalog recovery, not HTTP serving or request latency.

Fifteen additional native test events pass across thirteen leaf cases.
They cover expiry at the final publication and acceptance write, backend replacement,
and shared application startup with explicit recovery approval.
Startup refuses missing approval. The previous accepted head remains after expiry.
The existing shared-storage CI job now includes these contracts and process recovery.

The [process checkpoint](../../plans/proof/starport-production-catalog/csp11/process-recovery-2026-09-26/verification.json) records exact commands and limits.
Published-module qualification, acceptance registration, consumer review, and merges remain open.
The extra goago run used the Starmap tool through the temporary workspace.
Starport does not declare that tool or its policy. Preserve its six reports as historical diagnostics, outside the Starport gate roster.


### CSP11 published dependency qualification: September 26, 2026

Starport `09ed740cb` pins published Starmap `a2d66bd25`.
All eleven fleet acceptance subcases pass without a local Go workspace.
The architecture rerun passes twelve conditions, including the full Go suite.
The remaining required repository commands pass. Both pre-PR reviews report no findings.

[Starmap #185](https://github.com/agentstation/starmap/pull/185) must merge before [Starport #385](https://github.com/agentstation/starport/pull/385).
Both PRs still need exact-head CI and protected merges.
The [published-pair checkpoint](../../plans/proof/starport-production-catalog/csp11/published-pair-2026-09-26/verification.json) retains commands, review results, and the initial workspace-boundary failure.
CSP13 owns upgrade compatibility and complete state migration. CSP15 owns immutable payload retention and backend qualification.


### Deployment isolation evidence correction: September 26, 2026

The earlier CSP11 namespace tests prove catalog isolation only.
A real-Valkey probe against Starport `09ed740cb` gives two configurations distinct deployment IDs.
The second configuration can read a gateway API-key record created through the first configuration.
`identity:v1:` and `credentials:v1:` still identify concept namespaces without a deployment prefix.

The [scope audit](../../plans/proof/starport-production-catalog/csp11/isolation-scope-2026-09-26/verification.json) preserves the failing test and source.
CSP12 owns all gateway namespace enforcement and explicit fresh-fleet initialization. CSP13 owns populated-state recovery.
The acceptance map retains every original A41 requirement and adds two catalog-specific cases.
The original broad labels must not turn catalog evidence into a deployment-wide qualification claim.


### Fleet recipe preparation: September 26, 2026

The configuration loader accepts Valkey with local SQLite and filesystem blobs.
It also accepts Valkey with PostgreSQL and filesystem blobs.
The [recipe audit](../../plans/proof/starport-production-catalog/csp12/recipe-audit-2026-09-26/verification.json) records both failures and unpublished configuration guards.
Those guards reject incomplete fleet recipes before storage access.
Focused checks pass 16 named events across 12 leaf cases. Full qualification remains open.

Existing budget checks already refuse unknown required usage and team policy.
The native baseline passes 20 named events across 15 leaf cases, including real Valkey, with no skips.
Full retry and reservation coverage remains open. CSP12 must preserve these existing refusals.

The durable Valkey adapter still strips URI prefixes instead of applying the complete validated connection contract.
Its client options omit several declared connection settings. The optional cache has a separate validated endpoint implementation.
CSP12 must apply effective settings and preserve the separate cache service and credential boundary.


## September 26 clock qualification decision

D40 makes physical OS suspend testing optional platform evidence. CSP22 owns that follow-up, which remains UNVERIFIED.
The 60-second authorization limit, observed withdrawal enforcement, and stricter Starmap authority receipts remain mandatory.
Native adapter checks and deterministic expiry, clock-failure, and withdrawal tests remain required.

The [clock audit](../../plans/proof/starport-production-catalog/csp10.2/clock-contract-audit-2026-09-26/verification.json) passes all 25 required tests with no failures or skips.
D40 resolves its former pending policy decision. Historical results remain unchanged.
The [optional procedure](../../plans/proof/starport-production-catalog/csp10.2/optional-suspend-qualification.md) defines the additional hardware test.
These clock results do not establish complete gateway latency or multi-host propagation performance.

The [proof](../../plans/proof/starport-production-catalog/csp10.2/merged-qualification-2026-09-26/verification.json) passes all ten A46 subcases. The toolchain is Go 1.27.1.
It records 192 named passing test events, with no failures or skips. CSP10.2 is complete at Starport merge `f5f066ddb`.

## September 26 fleet upgrade decision

D41 retains the fleet baseline independently of binary releases.
The existing replay digest includes the packaged baseline, but recovery records omit its complete bytes.
That combination can prevent every upgraded replica from owning refresh.
The [transition contract](../../plans/proof/starport-production-catalog/csp11/upgrade-contract-2026-09-26/CONTRACT.md) assigns seventeen acceptance conditions across CSP11, CSP16, and CSP16.1.

Configured catalog updates remain automatic. Acquisition-policy apply and embedded-only promotion remain explicit.
The owner decision is complete. At the September 26 audit, implementation, native qualification, review, and both merges remained open.

The [September 27 qualification](../../plans/proof/starport-production-catalog/csp11/merged-pair-2026-09-27/verification.json) records the completed CSP11 baseline repair.
Starmap #185 merged at `2b98ab260`. Starport #385 merged at `6bd7fcf4`.
Coordinated policy apply and explicit baseline promotion remain open under CSP16 and CSP16.1.

### Configuration ownership after D41

The current CSP16 contract already assigns shared SQL configuration revisions and transactional audit to one configuration owner.
CSP16.1 assigns expected revisions, idempotency, and saved-versus-applied status to its operator API.
A separate CSP11 policy-apply protocol would duplicate those responsibilities.
D41 permits the binary-upgrade repair without that duplicate: replay uses the retained baseline while incompatible policy still refuses ownership.

CSP16 must bind the approved SQL revision to native Valkey publication and lease fencing before enabling policy writes.
CSP16.1 must implement explicit baseline promotion through that operation contract.
The transition contract retains all seventeen acceptance conditions. The baseline repair does not implement policy transitions.


## September 27 fleet bootstrap repair

Final consumer review reproduced two defects in `76a5687c`.
Input-only publications displaced distinct rollback generations from accepted history.
Complete catalog-key loss inside a live Valkey process could appear to be an unused deployment.
Both real-store regressions failed before repair.

Consumer `aab7dd7c` retains distinct rollback generations and consumes independent SQL permission before first publication.
Only explicit fresh initialization grants that permission. Existing migrated approval rows default to denied.
An interrupted first publication after consumption requires controlled recovery.

CSP13 owns that procedure, populated deployment adoption, and recovery after Valkey restart or failover.
The operator documentation states that those commands remain unavailable in this intermediate build.

The [repair proof](../../plans/proof/starport-production-catalog/csp11/retained-baseline-2026-09-26/bootstrap-review-repair/verification.json) records 13 focused race results and 75 split broad results.
The initial broad command timed out after 74 passes. Its remaining test passed separately with unchanged assertions and timeout.

All eleven required CSP11 subcases, 34 consumer commands, 99 verifier tests, and full producer checks pass.
Producer `0f45bbae2` registers the new acceptance checks. Consumer qualification uses published producer module `b46c077a7`, whose Go source remains unchanged.

Producer review passes. Consumer review confirms three known production adoption/recovery gaps.
Their existing CSP12, CSP13, and CSP15 requirements remain release blockers. The consumer review is not clean.
Both PRs contain the reviewed commits. Exact-head CI and protected merges remain open.

## September 27 incomplete usage settlement

Starport `a189b8dc` treated missing or null prompt and completion counts as zero.
A partial report could satisfy the sum check and release budget capacity without complete monetary evidence.
The [production regression](../../plans/proof/starport-production-catalog/csp12.2/usage-presence-2026-09-27/verification.json) fails for three partial reports before repair.

Consumer `0ea27f36` retains usage presence through decoding, copying, and serialization.
Required settlement rejects incomplete totals. Complete explicit zeros remain valid.
Final focused race checks pass 35 results. Broader component and repository evidence remains in the same proof.

The audit also found estimated Vertex embedding counts and zero Ollama embedding counts without measurement.
CSP12.2 must preserve these as unknown during required settlement. Current strict-budget embedding dispatch remains unsupported.


## September 27 embedding admission

The prior production path refused embedding calls with required budgets because no quote existed.
Six public-route regression subcases returned HTTP 503 before implementation.
The [embedding proof](../../plans/proof/starport-production-catalog/csp12.2/embedding-billing-2026-09-27/verification.json) preserves those failures and the repaired behavior.

Schema 12 retains complete embedding billing declarations independently of prices.
The OpenAI serving record now declares the verified input basis and 8,192-token input limit.
Its legacy zero per-token placeholder already resolves as missing beside a nonzero per-million rate. No price-presence repair was necessary.

Starport reserves per-item limits and settles complete measured input counts.
Missing or inconsistent counts remain uncertain, including native absent usage and Vertex estimates.
The semantic-cache adapter previously omitted the caller's routing configuration. Child calls now retain that configuration and separate reservation identity.

All eleven production race results pass. The full consumer suite passes 4,403 results and skips 133.
The producer catalog package passes 1,335 race results. Final focused producer checks pass 23 results.
Consumer package race checks pass 1,048 results and skip 38. Forty-five pure-Go results pass.
These checks use the unpublished producer through the candidate workspace.
Full A47, shared-backend operation coverage, native CI, exact dependency qualification, and required reviews remain open.

The Vertex adapter still assumes a scalar string input. A non-string input can panic at its type assertion.
CSP12.2 must repair and qualify its input shapes before claiming complete embedding coverage.
Legacy display estimates also need explicit provenance before they can represent measured usage outside strict settlement.

## September 27 embedding usage and cache repair

Vertex accepted only scalar text through an unchecked type assertion and returned only the first vector.
It estimated token usage by splitting words. Public responses and optional accounting could present unknown counts as measured zero.
The [usage proof](../../plans/proof/starport-production-catalog/csp12.2/vertex-usage-2026-09-27/verification.json) retains the failed provider, wire, accounting, and legacy-cache regressions.

Consumer `0231ec7a` preserves complete provider measurements across these boundaries.
Legacy cached vectors remain usable, but their usage remains unknown without recorded provenance.
Malformed vector output retains complete paid usage for required settlement.
Optional embedding costs use the declared billing contract and exact arithmetic.

The initial package race run failed the semantic streaming cache case.
Its setup assumed that every optional fill succeeded. The repaired setup waits for actual production submissions to become readable.
Hit and provider-call assertions remain unchanged. Ten matrix runs pass 110 results.

The final package race run passes 900 results. The repository run passes 4,440 results and skips 133.

All evidence uses the candidate producer through `GOWORK`.
Vertex monetary declarations, remaining operation contracts, recovery, A47, review, native CI, and merges remain open.

## September 27 recognition admission and runtime retention

Producer `5790fb04e` retains complete recognition charges in schema 13.
Consumer `8b1dc0eb` reserves selected-route charges and settles raw provider usage before response processing.
The production fixture uses the Gemini connector, HTTP gateway, Badger, and SQLite.
It verifies measured and unknown usage, incomplete documents, later chat failure, budget refusal, and extraction-cache reuse.

The [recognition proof](../../plans/proof/starport-production-catalog/csp12.2/recognition-billing-2026-09-27/verification.json) preserves failed regressions and qualification limits.
The previous operation path refused required recognition budgets because it supplied no quote.
Document parsing also wrapped gateway budget refusals as provider failures.
Both failures now preserve the required admission behavior.

With chat-response caching disabled, parsing previously had no retained runtime generation.
Extraction-cache keys were incomplete. Two identical requests made four paid attempts instead of three.
The repair retains the generation through parsing and chat, including stream closure.
The production cache regression and lease-lifetime cases pass.

Optional reports previously treated unknown tokens as free usage and omitted cached-token counts as measured zero.
The repair preserves both unknown states. Complete totals remain visible when missing cache detail prevents exact pricing.
Final affected-package race checks pass 738 results and skip 22. Lint and vet pass.
The earlier repository run passes 4,485 results and skips 133. It precedes the final provenance repair.

Fixed-page evidence has component coverage but no qualified shipped provider connector.
The consumer still uses the unpublished producer through `GOWORK`.
A47, remaining operations, asynchronous recovery, exact dependency qualification, review, native CI, and merges remain open.

## September 27 fixed-page recognition

The page-billing projection initially lacked a shipped page-based provider transport.
Starmap `9d81c4293` adds the Mistral OCR protocol and an exact offering with standard per-page pricing.
Starport `1253668c` enforces explicit page selection and settles provider-reported page usage.
A required token budget refuses this contract because page billing does not establish measured token bounds.

The review found optional reports that priced native document pages without provider usage.
The repair retains measured processed pages separately. Missing usage stays unpriced, and measured charges survive incomplete extraction.
Three old fixtures now declare their measured pages. Their original cost assertions remain unchanged.

The protocol roster includes OCR, and the catalog census verifies twelve recognition offerings.
A missing provider page ceiling requires the protocol that enforces explicit page selection.

The [qualification proof](../../plans/proof/starport-production-catalog/csp12.2/page-recognition-2026-09-27/verification.json) preserves failed regressions and final results.
Final repository checks pass 4,513 results with 133 skips. Focused race checks pass 27 results.
Producer race checks pass 1,350 results. Both commits remain unpublished.
The exact dependency pin, remaining operation contracts, durable recovery, A47, review, native CI, and merges remain open.


## September 27 moderation and guardrail admission

Starmap `5cef96251` declares complete moderation request billing in schema 14.
Starport `9e54be6a` admits and settles direct and guardrail moderation from the selected catalog contract.
The [qualification proof](../../plans/proof/starport-production-catalog/csp12.2/moderation-billing-2026-09-27/verification.json) retains failures, source hashes, commands, and raw test events.

The initial production regression returned HTTP 503 for free moderation under a spend budget.
The operation lacked a quote. The repair requires an explicit complete contract and request price.
Free pricing does not establish token consumption. Required token budgets still refuse before dispatch.

The guardrail adapter dropped the caller's routing restrictions when it created its child request.
The repair preserves those restrictions. Production tests confirm that forbidden models reach no provider.
The pipeline also discarded underlying errors. It now preserves typed admission causes and their retryable HTTP status.

The old optional-report test expected unknown cost because the provider reports no tokens.
The catalog now supplies an explicit request price. The revised test verifies known zero cost and unknown tokens independently.
Provider failures retain uncertain reservations. Separate child attempts retain the parent's budget rules and identity.

Producer race checks pass 1,354 results. Consumer repository checks pass 4,531 results and skip 133.
Focused race checks pass 42 results. The final application race check passes nine results.
Pure-Go checks pass 34 results. Lint, vet, and goago pass.

These checks use an unpublished producer through `GOWORK`. The remaining CSP12.2 requirements remain open.


## September 27 rerank budget admission

Starmap `abd3475b3` declares complete synchronous token billing for Voyage rerank-2.5 and rerank-2.5-lite.
Starport `17564bb4` reserves every submitted pair, including repeated query tokens.
The [component proof](../../plans/proof/starport-production-catalog/csp12.2/rerank-billing-2026-09-27/verification.json) records source hashes, commands, failures, and raw test events.

The initial production test returned HTTP 503 because rerank lacked an admission quote.
The implementation now reserves spend and token capacity before provider dispatch.
Production tests use Badger, SQLite, the gateway HTTP handler, and a loopback provider.
They verify insufficient-capacity refusal, measured settlement, unknown usage, and charges after an invalid ranking result.

The previous decoder rounded provider counts. It also lost the distinction between absent usage and explicit zero.
Two additional regressions reproduced fractional rounding and negative underflow.
The repair validates bounded decimal literals exactly. Both public response codecs preserve measurement presence.
The catalog projection shares input-token arithmetic with embeddings and independently validates each operation's billing declaration.

The final consumer suite passes 4,575 results and skips 133. Producer race checks pass 1,358 results.
Focused race checks pass 181 results before the final decimal repair. Final decoder and wire race checks pass 41 results.
Final decoder and wire pure-Go checks pass 41 results. Lint, vet, and goago pass.

The provider-ownership guard passes nineteen conditions. The rerank structural guard passes twenty-two conditions.

The [Voyage API reference](https://docs.voyageai.com/reference/reranker-api) supplies the request and pair limits.
Its [pricing documentation](https://docs.voyageai.com/docs/pricing) supplies token rates and repeated-query accounting.
The source review date is September 27, 2026. The contract excludes account credits and native batch discounts.

The [Cohere pricing FAQ](https://cohere.com/pricing) counts billing chunks as documents within search units.
The [Cohere API reference](https://docs.cohere.com/v2/reference/rerank) describes document truncation.
These facts do not yet establish the complete supported billing bound. Cohere strict admission remains required and incomplete.
The local workspace dependency also remains unpublished. No result establishes final CSP12.2 or released-pair qualification.


## September 27 speech budget admission

Starmap `3635aa986` declares character-priced speech for TTS-1 and TTS-1-HD.
The previous catalog marked their output as text. The initial production probe returned HTTP 400 before provider dispatch.
The corrected modality produces speech endpoints and removes unsupported chat endpoints.
Starport `7326c120` reserves catalog-defined character charges through the shared admission path.

The [component proof](../../plans/proof/starport-production-catalog/csp12.2/speech-billing-2026-09-27/verification.json) records twelve production cases and their parent result.
Cases cover Unicode, combining marks, whitespace, the maximum input, excess input, capacity refusal, absent budgets, token uncertainty, and response failures.
Empty completed audio retains its charge. An interrupted download retains uncertain capacity.
Usage capture preserves input characters, unknown tokens, measured zero, and invalid measurements.

The audit found a shallow character-price copy in reconciliation. Its regression test failed before the copy repair.

Producer catalog race checks pass 1,361 results. Reconciler race checks pass 689 results.
Consumer race checks pass 63 results and skip four. Pure-Go checks pass 50 results and skip four.
The complete consumer suite passes 4,598 results and skips 133. Skips remain unqualified.

The [TTS-1 model page](https://developers.openai.com/api/docs/models/tts-1) supplies its price and modalities.
The [TTS-1-HD model page](https://developers.openai.com/api/docs/models/tts-1-hd) supplies its character price.
The [speech API](https://developers.openai.com/api/reference/cli/resources/audio/subresources/speech/methods/create) states the 4,096-character input limit.
The source review date is September 27, 2026.

The code-point interpretation follows public character pricing and JSON string semantics. It is an inference without provider invoice verification.
No live paid call occurred. The unpublished workspace dependency and remaining CSP12.2 contracts still require qualification.

## September 27 image billing admission

Starmap `fb7f8ae21` adds schema 17 with image-count and pixel-iteration contracts.
Starport `2a2e8a5b` reserves declared image charges before provider dispatch.
The production cases use Badger, SQLite, and a local HTTP provider through the actual gateway and connector.
An empty image entry retains the complete reservation as uncertain. The regression failed before this repair.

The [image proof](../../plans/proof/starport-production-catalog/csp12.2/image-billing-2026-09-27/verification.json) records fifteen production cases and their parent result.
Final consumer race checks pass 45 results. Final catalog race checks pass 1,355 results.
Reconciler checks pass 690 results. Provider checks pass 103 results and skip three opt-in cases.

The full consumer suite passes 4,629 results and skips 133. Pure-Go checks pass 28 results.
The proof identifies source timing, failed attempts, source hashes, and remaining qualification limits.

DeepInfra public acquisition, fixture refresh, and wire drift checks pass. Its fixture contains 187 records and 26 image-unit prices.
The broader live-provider gates fail for missing credentials and stale fixtures outside DeepInfra.
No paid inference or provider invoice verification occurred. Those failures and all skips remain recorded.

Image edits, token-priced images, other speech units, transcription, video, recognition variants, and search-unit admission remain incomplete.
Asynchronous recovery, shared-backend qualification, A47, the published dependency, review, native CI, and merges remain required.

The [DeepInfra image API](https://docs.deepinfra.com/apis/image-generation) documents count, size, and compatibility-only quality and style parameters.
The [Schnell API page](https://deepinfra.com/black-forest-labs/FLUX-1-schnell/api) prices pixels and iterations.
The [FLUX-1.1-pro API page](https://deepinfra.com/black-forest-labs/FLUX-1.1-pro/api) states a flat image price.
The source review date is September 27, 2026.

The new fixture test initially compared exact floating-point representations. Existing acquisition normalization removes representational noise.
The corrected assertion permits two adjacent floating-point steps and still detects unit conversion.
The provider-decoding function exceeded the lint complexity limit. Billing-schema validation now has its own function with unchanged version refusals.


## September 27 accounting retry foundations

Starport `3fb9c131` repairs three reproduced defects before asynchronous settlement integration.
Usage replay previously increased counters again. Job replacement could erase a newer accounting stamp during a same-state update.
A lost asset-publication acknowledgement could also delete bytes that the committed job referenced.
Each regression failed before its repair. Job regressions reproduced on memory, Badger, and Valkey.

The [component proof](../../plans/proof/starport-production-catalog/csp12.2/accounting-retries-2026-09-27/verification.json) records the source commit, commands, test names, and raw output.
Optional usage now commits its record and aggregate changes in one native conditional batch.
Exact replay preserves totals. Conflicting contents, expired absent receipts, counter corruption, and overflow refuse.
The tests cover concurrent distinct events, repeated events, lost acknowledgements, expiration precision, and Badger close/reopen.

Job replacement now binds the caller's observed record. Concurrent updates must read the accepted record before another change.
A definite asset-publication conflict discards the candidate. An uncertain result preserves its bytes and attempts a confirming read.
An unresolved result can leave unreferenced bytes. Bounded recovery and collection remain required.

The final usage and job race suite passes 211 results without skips. Earlier caller race checks pass 872 results and skip one.
The complete suite passes 4,793 results and skips 71. Eight packages skip independently.
Lint and affected-package vet pass. The tests use Go 1.27.1, real Badger, and a task-owned Valkey container.

Required budget settlement remains separate from optional analytics. The job terminal stamp still precedes reporting and slot release.
Durable submission, pinned valuation, settlement retries, idempotent slot release, and complete recovery enumeration remain open.
Shared-process failure, failover, the PostgreSQL witness, and released-pair qualification remain unverified. This component does not complete CSP12.2.


## September 27 durable submission repair

Consumer `c270de27` repairs provider dispatch before durable job creation.
The two regression contracts produced eight failing results across memory, Badger, and Valkey before the repair.
The [component proof](../../plans/proof/starport-production-catalog/csp12.2/async-submission-2026-09-27/verification.json) preserves failed and passing runs, exact source hashes, commands, and skipped checks.

The actual HTTP provider now observes the durable attempt before receiving work.
Starport persists acceptance before success and rechecks permission after its initial durable write.
A bounded acceptance write survives client cancellation. A confirming read resolves a lost commit acknowledgement.
An ambiguous reply stops retries and retains the job and outstanding slot.
Both protocol prefixes expose the gateway job ID and lookup location without exposing the provider handle.

Final race checks pass 39 results. Pure-Go checks pass 25 results.
The full suite passes 4,825 results and skips 71. Eight packages skip independently.
Lint, affected-package vet, six dependency conditions, and eighteen asynchronous-job conditions pass.
The initial lint failure led to a separate submission-validation method, without weaker validation.

Required video budgets still refuse before dispatch. Current video pricing and terminal-state cost assumptions remain unqualified.
The terminal accounting stamp still precedes reporting and slot release.
Ambiguous job creation can retain an unresolved slot. Durable slot claims and settlement recovery remain required.
Real storage tests do not qualify process loss, failover, or the PostgreSQL recovery witness.

This component does not complete CSP12.2.


### Shared batch and video counter repair

Consumer `b7194135` repairs an unbounded batch that skipped reservation but still released a slot.
That release reduced the counter for an active video.
The regression produced four failing results across memory, Badger, and Valkey.
Unbounded batches now reserve their own slots before dispatch. Completion preserves the active video's count.

The [submission proof](../../plans/proof/starport-production-catalog/csp12.2/async-submission-2026-09-27/verification.json) includes this follow-up and its source hashes.
The complete jobs and limits race suites pass 128 results without skips.
The additional batch and production-video run passes 20 results without skips. Lint and affected-package vet pass.
Durable claim identity, idempotent release, and settlement recovery remain open.


## September 27 durable outstanding-work claims

Consumer `f5788320` replaces anonymous job counters with durable claims shared by videos and batches.
Repeated release previously decremented another job's capacity. The regression failed on memory, Badger, and Valkey.
A native conditional batch now commits the claim, count, and account marker together.

The [claim proof](../../plans/proof/starport-production-catalog/csp12.2/slot-claims-2026-09-27/verification.json) retains the failing regressions and source hashes.
It also records repaired read races during concurrent initialization and release.
Final jobs and claim race checks pass 156 results. Pure-Go checks pass 77 results.
Both runs have zero skips. The broader application race run exceeded its four-minute limit in existing speech tests.

Video slot release retries after optional accounting and interrupted acknowledgements.
Batch cancellation retains its slot until admitted lines finish. A later read can recover an interrupted release.
Batch compare-and-swap now binds the complete observed record, so stale changes cannot erase ownership progress.
Legacy counters and old record schemas require migration. The new format cannot claim a complete upgrade path before CSP13.

Claims without confirmed job records still require recovery. The video sweep still has incomplete enumeration, and batch recovery still depends on access.
Durable required settlement, pinned valuation, replay horizons, process loss, failover, and the PostgreSQL witness remain unqualified.
This component does not complete CSP12.2.


## September 27 recovery enumeration

Consumer `3d14fb65` repairs another recovery defect. Repeated sweeps previously omitted one of 1,001 retained records on Badger and Valkey.
Native cursor scans now reach those records. Finished batches can release slots without client reads.
Corrupt records no longer prevent valid records from recovering. Interrupted invocations retain their unfinished page work.

The [recovery proof](../../plans/proof/starport-production-catalog/csp12.2/recovery-scans-2026-09-27/verification.json) preserves the failing regression and exact source timing.
The component race run passes 379 results and skips one backend-replacement test.
Final focused race checks pass 29 results without skips. They include production composition, durable release, and zero provider dispatches.
Pure-Go checks pass 25 results without skips. The full suite passes 4,896 results and skips 71. Eight packages skip.

Lint and affected-package vet pass. Native platform, process-loss, and failover qualification remain required.
Orphan claims and required spending settlement remain incomplete. CSP12.2 stays in progress.


## September 27 pending claim recovery

Consumer `a8137780` repairs publication after claim release. The regression failed on memory, Badger, and Valkey before the change.
Job creation and claim attachment now commit together. Recovery closes old unattached claims and prevents delayed publication against them.
The application runs this recovery through its existing job-maintenance loop.

The [attachment proof](../../plans/proof/starport-production-catalog/csp12.2/claim-attachment-2026-09-27/verification.json) retains failed builds and the repaired fixture that used separate claim and record stores.
Final jobs and claim race suites pass 200 results without skips. Pure-Go checks pass 60 results without skips.
The production ownership checks pass within a separate 59-result race run. They inspect attachment before provider dispatch.

The full suite passes 4,918 results and skips 71. Eight packages skip. Lint and affected-package vet pass.

Claim, counter, and history version 3 requires CSP13 migration. Job and batch schema versions remain 2.
Attached uncertain submissions still require provider reconciliation. Required settlement and remaining native qualification keep CSP12.2 in progress.


## September 27 cancellation and video contract audit

Consumer `e0b4b237` repairs cancellation that previously discarded the provider state and released capacity.
The regression failed on memory, Badger, and Valkey. The connector also synthesized cancellation from an HTTP deletion acknowledgement.
The repair preserves reported outcomes and checks the provider identifier on polls and cancellation replies.

The [component proof](../../plans/proof/starport-production-catalog/csp12.2/cancellation-outcomes-2026-09-27/verification.json) records 585 package race results and 81 focused race results, including both production API families.
Pure-Go checks pass 46 results. The full suite passes 4,986 results and skips 71. Eight packages skip.
Lint, affected-package vet, and both architecture checks pass. The operator guide retains 48 prose diagnostics outside the edited section.

Provider research changes the next implementation action:

- DeepInfra publishes USD 0.075 per second for Wan2.2-T2V-A14B. Its documented native route returns video output directly. [DeepInfra API](https://deepinfra.com/Wan-AI/Wan2.2-T2V-A14B/api)
- OpenRouter declares duration, model pricing SKUs, and terminal usage cost. The current generic request and accounting code do not establish that contract. [OpenRouter video guide](https://openrouter.ai/docs/guides/overview/multimodal/video-generation)
- OpenAI lists September 24, 2026 as the Videos API and Sora 2 removal date. Historical qualification cannot establish current availability. [OpenAI deprecations](https://developers.openai.com/api/docs/deprecations)

The current Starmap video price field says per video. Starport also prices completed jobs from the current catalog instead of the submission valuation.
Both defects remain under CSP12.2. Provider-specific route selection and billing qualification must precede strict-budget video support.
The local timeout path still releases a slot without confirmed provider completion. That recovery defect also remains under CSP12.2.
No live video inference or invoice check occurred. This component does not complete the task.


## September 27 polling recovery

Consumer `1143dfb0` repairs local timeout that previously failed a job and released capacity without provider completion evidence.
The regression failed on memory, Badger, and Valkey. Nine failing results include the parent tests.
The [polling proof](../../plans/proof/starport-production-catalog/csp12.2/polling-recovery-2026-09-27/verification.json) binds the source and raw verification logs.

A paused job retains its last provider state, claim, and unaccounted status.
Both API families publish `polling_status: "paused"` and an account-scoped reconciliation extension.
The explicit check never resubmits generation or extends automatic polling. Errors retain capacity, and confirmed terminal responses permit outstanding-slot release.
The console reports paused work and exposes **Check provider**. Cancellation feedback now reports the actual provider outcome.

The jobs race suite passes 190 results. Focused race checks pass 54 results, including both production HTTP families.
Pure-Go checks pass 32 results. Those runs have no skips.
The full Go suite passes 5,004 results and skips 71. Eight packages skip.

All 464 console tests pass across 74 files. Console lint, build, type checks, Go lint, and affected-package vet pass.
The operator guide retains 48 pre-existing prose diagnostics outside the changed section.

The current Jobs page refreshes listings, which do not poll providers. Explicit reconciliation supplies manual recovery for accepted handles.
Background provider recovery and concrete replay horizons remain open. Required settlement must retain the submission valuation and authoritative billing evidence.

The historical AMJ-V15 guard still assumes terminal-state charging. Its passing result does not qualify billing.
Provider-specific video contracts, ambiguous submissions, process loss, failover, migration, and A47 remain unqualified.
This component does not complete CSP12.2.


### Live video pricing inventory

The public OpenRouter video endpoint returns 29 models and 37 distinct SKU keys in the September 27 capture.
The [public response](../../plans/proof/starport-production-catalog/csp12.2/video-contract-audit-2026-09-27/verification.json) requires no credential and requests model metadata only.
Its names include seconds, video tokens, input images, minimum charges, and megapixel-seconds.
Resolution, audio, input mode, continuation, and creativity can change price selection.
Some names explicitly use cents. The documentation's simple per-second example does not define all live keys.

A scalar per-second replacement cannot represent this inventory.
CSP12.2 must verify quantity, currency scaling, selectors, additive charges, and minimum semantics before declaring a complete video quote.
Unknown SKU semantics must remain unknown. A matching name alone cannot establish safe reservation or settlement.
The audit does not qualify any new provider contract.

## September 27 media duration units

The provider adapter previously mapped `input_seconds` to `audio_input` and `output_seconds` to a per-item generation field.
That mapping lost the quantity unit. The original acquisition regression fails before the correction.
Schema 18 preserves `input_second` and `output_second` through validation, copies, reconciliation, and serialization.
The current fixture verifies duration conversion for thirteen DeepInfra models. Twelve embedded records receive unit corrections without numeric price changes.

Eleven match current acquisition. The retained FastVideo record follows historical commit `0414a3cd1` and remains unverified for current availability.

Consumer `06982eb2` refuses optional video valuation when duration quantities remain unknown, including data that also contains a per-video price.
The mixed-rate regression previously returned a partial cost. Strict video budget admission remains unqualified and must refuse before dispatch.

Broader checks found incomplete OCR catalog metadata from an earlier component.
The correction adds a separate identity-review supplement and explicit unknown Boolean capabilities. The historical identity map remains unchanged.
The media census now includes the two added speech offerings and the new OCR offering.

Mistral's current model page identifies `mistral-ocr-4-0` and its fixed-page prices. [Model documentation](https://docs.mistral.ai/models/ocr-4-0)
The inspected limits page specifies file-upload limits but no OCR page ceiling. [Known limitations](https://docs.mistral.ai/resources/known-limitations)
The catalog leaves that page ceiling unknown. Fixed-page admission still requires a decoded positive page count and enforces a known ceiling when one exists.
Token-billed recognition retains its bound checks. This correction does not add annotations or provider-native batch billing.

The [duration proof](../../plans/proof/starport-production-catalog/csp12.2/media-duration-units-2026-09-27/verification.json) owns the source revisions, failed attempts, check counts, and remaining limits.
Global live-provider qualification still fails outside DeepInfra. Missing credentials and stale fixtures remain recorded.
Complete video billing, pinned asynchronous valuation, required settlement, and full task qualification remain open under CSP12.2.

## September 27 retained settlement recovery

The ledger retained measured usage after a settlement failure, but application startup did not retry it.
The startup regression reproduced an unchanged uncertain reservation before the worker existed.
The new worker uses the original storage authority and starts only with the application runtime.
Shutdown waits for it before storage closes. Each pass limits work and elapsed time.

Native scans retain their continuation and unread entries between passes. Corrupt records do not prevent unrelated settlement.
Badger and Valkey tests cover concurrent workers, lost acknowledgements, missing usage, and more than one thousand records.
A Badger reopen test recovers retained evidence. Process-loss qualification remains open.

The pure-Go check exposed a concurrent settlement defect.
A worker could read an old attempt followed by newly settled balances and report unavailable state.
The repair retries only when the attempt changed between reads. Unchanged inconsistent balances still refuse settlement.
A deterministic regression reproduces this race on both native backends.

The independent approval test uses SQLite with Valkey. A closed or changed approval prevents old-worker settlement.
It does not qualify the primary PostgreSQL fleet recipe or failover.
Full-history traversal adds background reads. Capacity qualification must measure recovery delay and request interference before production acceptance.

The [recovery proof](../../plans/proof/starport-production-catalog/csp12.2/retained-settlement-recovery-2026-09-27/verification.json) records exact results and source hashes.
Missing provider evidence remains unresolved. Complete asynchronous billing, interrupted batches, and full CSP12.2 qualification remain open.

## September 27 required job settlement boundary

The job service previously marked accounting complete without reading its required reservation.
The regression reproduces that defect on memory, Badger, and Valkey storage.
The new boundary requires durable settlement evidence before the optional mark. Job completion alone cannot settle a spending reservation.

Reservation attempts now bind one job identity. Concurrent bindings select one winner, and exact retries preserve that selection after a lost response.
The application verifies account, key, offering, generation, and operation before binding or confirming settlement.
Required settlement retries separately from concurrency-slot release. A prior optional mark cannot bypass the required check during recovery.

The shared-authority test uses SQLite and Valkey. It rejects closed approval and prevents an old owner from adopting a new recovery epoch.
This result does not qualify PostgreSQL, process loss, failover, or provider billing.
Attempt payload version 2 requires the CSP13 migration procedure before use with populated older state.

The [job settlement proof](../../plans/proof/starport-production-catalog/csp12.2/job-settlement-boundary-2026-09-27/verification.json) preserves the regression, results, and limits.
Optional reports still price from the current catalog and mark before delivery. Pinned measured reporting and retry remain required under CSP12.2.


## September 27 job reporting and DeepInfra protocol correction

The prior job accounting mark preceded optional delivery. A storage failure therefore prevented every later retry.
Terminal notification also waited for required settlement. The new regression checks both failures on memory, Badger, and Valkey.
The repair acknowledges optional reporting after delivery and claims the notification attempt independently.
The real usage repository preserves one aggregate contribution through concurrent retries after a missing job acknowledgement.

Notifications remain best-effort. A persisted attempt claim does not prove recipient delivery.

Pinned measured reporting remains incomplete. The current reporting adapter still reads the current catalog.
A price change can therefore conflict with an earlier usage receipt during recovery. CSP12.2 must remove that dependency before publication.

DeepInfra's current [OpenAPI](https://api.deepinfra.com/openapi.json) declares asynchronous video endpoints alongside native inference.
The earlier native-route observation did not establish a provider-wide transport rule.
The [live probe](../../plans/proof/starport-production-catalog/csp12.2/deepinfra-video-probe-2026-09-27/verification.json) records a model-specific HTTP 400 response for asynchronous Wan2.2-T2V-A14B submission.
The response requires native inference. The native endpoint then rejects two seconds and requires five.
These replies contradict the provider-wide video endpoint assumption and the advertised two-second option.

Model-specific transport selection, input constraints, complete billing, and crash recovery remain required.
The corrected native request produced a five-second video. DeepInfra reported USD 0.375 and five output seconds.
This direct provider test does not qualify Starport dispatch, final invoice reconciliation, or other video offerings.


## September 27 native video schema and usage retention

The model schema fixes Wan2.2-T2V-A14B at five seconds, 720p, and landscape output.
It permits a seed and provides no streaming schema. The provider response defaults an omitted status to succeeded.
Its `cost` field explicitly describes an estimate. The earlier USD 0.375 observation is not final invoice evidence.
The [video contract proof](../../plans/proof/starport-production-catalog/csp12.2/video-contract-2026-09-27/verification.json) records the schema and local checks.

Producer `eb03c466c` adds exact-model endpoint overrides and output-duration billing under catalog schema 19.

The selected native endpoint replaces the provider-wide OpenAI video route for this model alone.
The existing DeepInfra OpenAI URLs remain unchanged. Operator base-URL overrides now name the common `/v1` root, without `/openai`.
The regenerated payload retains earlier source observations and adds a custom-update observation for these changes.

The new consumer valuation pins the declared rate and preserves measured zero and overage.
The native adapter retains measured duration when an asset or explicit provider state fails validation.
It does not treat a native request ID as an asynchronous job handle. Production registration and durable job execution remain incomplete.

A new real-Badger regression reproduces lost usage after a failed acceptance write.
The router repair settles valid measured usage before returning that persistence failure. Missing usage retains the full reservation.
Both outcomes stop retries. The repair does not claim that the failed job write succeeded.


## September 27 native gateway jobs and pinned reports

Consumer `2e3c7733` connects native inference to production HTTP submission and durable gateway jobs.
Each replica bounds concurrent workers and execution time. Shutdown stops workers before their dependencies close.
A receipt retains measured usage and inline output before terminal publication. Recovery does not repeat inference.
The submitted price, asset bound, and retention window remain stable after configuration changes.

The production fixture found a credential-grant defect that component tests missed.
Grant compilation omitted exact-model endpoint overrides, so the native request never left Starport.
The repair includes the approved model path while rejecting other models and unsupported polling destinations.
A corrected regression proves the earlier refusal. The first regression fixture selected the wrong credential profile and does not prove that defect.

Optional reports now read retained valuation and measured duration. They never consult current catalog prices.
Failed or cancelled jobs can carry measured charges. Missing usage remains unknown, including for completed jobs.
Receipt corruption checks reject changed identities, invalid lengths, and altered asset bytes before completion.
Job schema 4 requires CSP13 migration for older populated state.

The [native job proof](../../plans/proof/starport-production-catalog/csp12.2/native-job-flow-2026-09-27/verification.json) records 1,898 race results with one container-recipe skip.
Application checks pass 14 results. Pure-Go checks pass 81 results. Nine final recovery/deadline results pass.
These counts include parent tests and subtests. They do not count independent acceptance cases.

No further paid generation ran. External asset downloads, uncertain-response recovery, PostgreSQL, process loss, failover, capacity, and full task qualification remain open.


## September 27 external assets and budget-independent retrieval

Consumer `fa42a9c7` adds explicit asset origins, bounded credential-free downloads, retained content digests, and safe retrieval status.
The first regression showed that external-only output never expired. The repair ends recovery at the submitted retention deadline.
A production fixture then found HTTP 402 on a completed job after its charge exhausted the budget.
Budget prechecks now apply to the 19 routes that start paid work. Both API families still enforce authentication and account ownership.

Concurrent recovery exposed a publication conflict between asset writes and diagnostic updates on memory, Badger, and Valkey.
Publication now preserves concurrent record updates. It cannot replace the first accepted asset bytes with a changed download.

A separate test fixture reused a fixed job identifier. Another cancellation assertion stopped at the storage boundary.
Both fixture errors remain in the proof with their corrections. They do not establish product defects.

The [external asset proof](../../plans/proof/starport-production-catalog/csp12.2/external-assets-2026-09-27/verification.json) records 1,852 broad race results and three opt-in skips.
Seventeen application results, 111 pure-Go results, and 20 final HTTP results pass.
Seven repository scripts, Go policy checks, targeted vet, and changed prose pass.

The source manifest binds 24 committed files. These results do not qualify missing-response recovery or the full paid-operation matrix.
No additional paid generation ran. PostgreSQL, process loss, failover, capacity, review, native CI, and paired merges remain required.


## CSP12.2 administrator recovery and late evidence: September 27, 2026

Consumer `7551e8d4` implements the approved administrator recovery contract.
Authenticated administrators can resolve uncertain native responses using actual usage or explicit no-charge evidence.
The job record retains the immutable identity, actor, reason, evidence reference, disposition, and decision time before releasing capacity.
Ordinary job responses expose safe status. They do not expose private audit evidence.

Review found that a late charged response could contradict a manually accepted no-charge decision.
The repair preserves both records and atomically blocks the original budget windows before publishing the conflict.
It grants no refund and changes no accepted charge. Matching late evidence does not block normal operation.
An audited correction procedure remains open. The first decision cannot be overwritten through the current endpoint.

The application checks exposed stale asynchronous fixtures after native offering selection changed.
Those fixtures now select the declared generic asynchronous offering and its paths. They do not prove live asynchronous provider support.
The polling fixture seeds an old record without weakening immutable production identity checks.
A separate failed run used an incorrect PostgreSQL hostname. Final checks use the isolated local service.

The [administrator recovery proof](../../plans/proof/starport-production-catalog/csp12.2/administrator-reconciliation-2026-09-27/verification.json) records 1,622 broad race results, 44 application results, and 45 pure-Go results.
Two optional broad checks skipped. Go policy, targeted vet, four repository scripts, and changed prose pass.
The proof preserves earlier failures and binds 36 committed files. Counts include parent tests and subtests.

These results cover memory, Badger, Valkey, and SQLite/PostgreSQL ownership checks in the selected suites.
They do not qualify full process loss, failover, capacity, A47, native CI, or the remaining operation matrix.
No additional paid generation ran. CSP12.2 remains in progress pending complete qualification, review, dependency publication, and paired merges.


## CSP12.2 interrupted batch ownership: September 27, 2026

The current batch runner retains line progress and result streams in memory.
A second worker changes durable cancellation state but cannot reach the first worker's local cancellation function.
The [batch probe](../../plans/proof/starport-production-catalog/csp12.2/batch-recovery-contract-2026-09-27/verification.json) fails on memory, Badger, and Valkey.
Each backend dispatches all three lines after cancellation, where only the already admitted first line should run.
Four test results fail, including the parent test. No production repair accompanies this probe.

`BatchService.Sweep` releases finished claims but cannot recover interrupted runs.
A durable line contract must own claim, dispatch identity, result references, cancellation, and restart state.
The [repair contract](../../plans/proof/starport-production-catalog/csp12.2/batch-recovery-contract-2026-09-27/CONTRACT.md) defines the required boundaries and evidence.
CSP12.2 owns this repair. The owner decision about continuing untouched lines after interruption remains pending.


## CSP12.2 atomic batch claims: September 27, 2026

Consumer `0683b733` repairs cancellation across workers through atomic batch-state and line-claim writes.
The original failing probe now passes on memory, Badger, and Valkey without changing its expected dispatch count.
Concurrent requests permit one claim winner. Failed or lost write acknowledgments permit no provider dispatch.
The stored request identity survives repository reopen and cannot authorize another execution.

The [claim proof](../../plans/proof/starport-production-catalog/csp12.2/batch-line-claims-2026-09-27/verification.json) records 540 package race results with one optional overhead skip.
Production batch checks pass 18 results. Pure-Go checks pass 27 results, and the final race fault checks pass 18.
Three repository scripts, targeted vet, Go policy, and changed prose pass.
The first script attempt omitted the paired workspace and failed against the old producer dependency. Its corrected invocation passes.

Batch schema 3 and line schema 1 preserve durable claims, not completed result recovery.
Interrupted-run policy, durable results, process loss, fleet qualification, A47, and paired merges remain open.
No additional paid generation ran.


## CSP12.2 completed batch output and byte-accounting failures: September 27, 2026

Two new probes use consumer `0683b733` without production changes.
The [process-loss proof](../../plans/proof/starport-production-catalog/csp12.2/batch-output-recovery-2026-09-27/verification.json) kills a real child worker after its first line returns and its second line starts.
Badger retains both claims, but the file service has no readable result for the completed first line.
The aggregate output remains incomplete until the batch closes its pipe. Durable claims alone cannot preserve paid output.

The second probe retires one already-deleting file from two workers using the real storage meter.
An unrelated eight-byte file remains readable, but the meter reports zero instead of eight bytes.
The failure reproduces on memory, Badger, and Valkey. Four test results fail, including the parent test.
The initial fixture timed out during a competing state transition. The final fixture isolates retirement and fails on every backend.

File removal deletes its record, then releases its byte charge without a durable claim identity.
Two successful idempotent deletions can therefore produce two decrements. Checkpoint cleanup must not use that behavior.
The [repair contract](../../plans/proof/starport-production-catalog/csp12.2/batch-output-recovery-2026-09-27/CONTRACT.md) assigns prepared output, retained results, and byte claims to their owning packages.
CSP12.2 requires this repair before durable batch recovery. CSP13 retains migration ownership.

Both probes remain failing evidence. No production repair or paid provider call accompanies this proof.
The restart-policy decision about untouched lines remains pending. Storage-accounting and output recovery can proceed independently.


## CSP12.2 durable file quota repair: September 27, 2026

Consumer `e2e85607` replaces anonymous increments and decrements with durable file claims.
The original concurrent-retirement assertion now passes on memory, Badger, and Valkey.
Lost acknowledgments during attachment, settlement, and release preserve unrelated capacity.
A stale ready write cannot revive deleting metadata. Prepared-claim recovery reaches later pages and fences delayed attachment.

The [storage proof](../../plans/proof/starport-production-catalog/csp12.2/storage-claims-2026-09-27/verification.json) records 633 broad race results with one optional skip.
Final storage race and pure-Go checks each pass 101 results. Application checks pass three results.
Vet, goago, three repository gates, and scoped prose checks pass.
The broader race run predates final accounting validation. Final targeted checks cover that change.

File schema 2 and byte-accounting schema 2 require CSP13 migration qualification.
Durable batch output and ordinary file-sweep pagination remain open. The commit remains local and does not complete CSP12.2.


## CSP12.2 retained batch results and retirement race: September 27, 2026

Consumer `d93432d0` retains completed line bytes before the worker admits another line.
The original process-loss assertion now passes with Badger and filesystem storage.
The [output proof](../../plans/proof/starport-production-catalog/csp12.2/durable-results-2026-09-27/verification.json) records 623 broad race results with one optional skip.
File checks pass 72 results, production checks pass 16, and pure-Go checks pass seven.
Final recovery checks pass nine results. Pagination and identity checks pass five.

A separate probe pauses publication while another service expires the file.
The paused writer later creates a blob after the file record and its quota charge are gone.
Four results fail across memory, Badger, and Valkey. The probe remains required evidence.
The [backend retirement repair](../../plans/proof/starport-production-catalog/csp12.2/durable-results-2026-09-27/RETIREMENT_CONTRACT.md) remains within CSP12.2.
The commit is local and does not qualify full batch recovery or shared-object storage.


## File retirement repair: September 27, 2026

Consumer `4a0de8d5` repairs the delayed-publication failure recorded at `d93432d0`.
The [proof](../../plans/proof/starport-production-catalog/csp12.2/retirement-2026-09-27/verification.json) records 59 final blob race results and 85 final file race results, with no skips.
Batch integration passes 16 results, pure-Go checks pass eight, and application composition passes two.
The original failing probe remains historical evidence.

Durable cleanup exposed a Valkey pagination timeout. Each pass now handles at most 256 records.
The final test removes all 1,030 expired records, preserves the live record, and rejects every retired identity.
An earlier run's two failed results remain in the proof.

The video owner still calls mutable blob writes and deletion. Its expiry race needs a separate audit.
The constructor still makes no bucket probe. Conditional-write readiness remains unqualified.
This local commit does not complete CSP12.2 or authorize a claim of production readiness.


## Video publication repair: September 27, 2026

Consumer `e0aae1c7` repairs two native defects and one provider-asset ownership defect.
Delayed writes could recreate expired content. Repeated callbacks could replace the first billing receipt.
A provider asset could lose its only reference after a successful write lost its acknowledgment.
The [proof](../../plans/proof/starport-production-catalog/csp12.2/video-publication-2026-09-27/verification.json) preserves five failed native results and the provider-asset failure.

Final checks pass 578 broad race results with one optional skip, 29 HTTP results, and 23 production results.
Pure-Go checks pass 23 results. Actual process-loss checks pass four results, and the prepared-bound check passes one.
Earlier schema, fixture, and compile failures remain evidence.

All job blobs now use immutable publication. Pending provider assets retain durable ownership.
Conditional-write readiness, restore, physical cleanup, and the full task acceptance criteria remain unqualified.

## Conditional publication readiness: September 27, 2026

Consumer `98bbd43d` checks byte-publication capability before file allocation and video dispatch. Five baseline readiness results failed before this repair. A separate real Valkey scan regression exceeded its deadline before the scan-work repair.

The [proof](../../plans/proof/starport-production-catalog/csp12.2/publication-readiness-2026-09-27/verification.json) retains exact commands, results, failure history, and source hashes. Runtime probes preserve unrelated gateway readiness. Warm checks use memory without heap allocation.

Aggregate reconstruction, interrupted-run recovery, audited correction, A47, capacity, review, native CI, and paired merges remain required.

## Batch aggregate repair: September 27, 2026

Consumer `7ba266a9` repairs unstable aggregate identity, unchecked cleanup, and deletion retries after lost acknowledgments.
The broad tests also found that recovery finished incomplete failed batches too early. Such batches now retain their claims.
The architecture check found a storage dependency in the limits vocabulary. The meter now belongs to `internal/limits/storedbytes`.

The [proof](../../plans/proof/starport-production-catalog/csp12.2/batch-aggregates-2026-09-27/verification.json) records 655 package race results with 88 optional skips.
Production checks pass two results. Router checks pass 23 results. Pure-Go checks pass 39 results with eight skips.
Two child-process interruption cases preserve aggregate identity and storage charges. Earlier failures remain evidence.

The router test reached terminal status before checkpoint retirement finished. It now waits for the durable cleanup marker before removing temporary storage.
Application shutdown still lacks a batch-worker drain contract. CSP12.2 owns that repair before publication.
Shared backends, interrupted-run policy, audited correction, full A47, capacity, review, native CI, and paired merges remain open.

## Batch shutdown repair: September 27, 2026

The baseline probe showed that application Close returned success during active checkpoint retirement.
Consumer `fa321e95` now drains batch workers before closing their dependencies.
It tracks submission writes, admitted calls, result persistence, and final cleanup.

The [proof](../../plans/proof/starport-production-catalog/csp12.2/batch-shutdown-2026-09-27/verification.json) records 91 batch race results with 15 optional skips.
Lifecycle checks pass seven results. Pure-Go checks pass five results. Go vet and goago pass.
A deadline preserves dependencies for a later Close attempt. The HTTP controller returns 503 for new work after shutdown starts.

The restart-policy question, audited correction, shared qualification, full A47, capacity, review, native CI, and paired merges remain open.

## CSP12.2 correction ledger: September 27, 2026

The dispute window stored one Boolean for all conflicting attempts. The new baseline test proves that two disputes had no separate retained count.
Consumer `3e1729c7` adds exact counts and atomic corrections with immutable receipts.

The broad limits run also found a prior compile error in `stored_bytes_test.go`.
The clone assertion incorrectly referenced `storedbytes.StoredBytes`. The repair restores `limits.StoredBytes` and preserves the assertion.
Earlier evidence does not qualify that test until the new passing run.

The [proof](../../plans/proof/starport-production-catalog/csp12.2/correction-ledger-2026-09-27/verification.json) records 236 limits race results, one PostgreSQL skip, 41 pure-Go results, and four production results.
Actual process interruption covers both sides of the correction commit. Real Valkey checks cover retry and competing decisions.

Classification: in-scope CSP12.2 budget and recovery repair.
Job audit integration, administrator routes, usage adjustments, and full task qualification remain open.

## CSP12.2 usage adjustments: September 27, 2026

Consumer `27983f87` adds explicit adjustment storage while preserving immutable `Put` behavior.
The baseline rejects changed billing under an existing usage identity. The new operation changes original-window totals without another request count.
Final checks pass 134 race results, 60 pure-Go results, and four existing production results without skips.
The [proof](../../plans/proof/starport-production-catalog/csp12.2/usage-adjustments-2026-09-27/verification.json) preserves the initial test compile failure and intermediate results.

Classification: in-scope CSP12.2 reporting and correction integration.
The job service still lacks correction intent, history, administrator routes, and report acknowledgement.
Consumer `0db2fbcd` replaces the two-write late-provider sequence with one atomic job and reservation transaction.
The [follow-up proof](../../plans/proof/starport-production-catalog/csp12.2/atomic-dispute-2026-09-27/verification.json) records stale-state, concurrent-write, and lost-acknowledgement checks.
The correction operation must still compare its inspected job state before clearing a dispute.

Consumer `00dfff81` adds the correction side of the atomic publication contract.
The baseline test rejects separate correction and job writes because a stale job can leave its dispute cleared.
The [proof](../../plans/proof/starport-production-catalog/csp12.2/correction-publication-2026-09-27/verification.json) passes 271 shared race results with one PostgreSQL skip.

Final targeted race checks pass 24 results. Pure-Go checks pass 21 results.
The job service does not yet call this correction operation. This evidence does not qualify an administrator correction API.

### Audited correction integration: September 27, 2026

Consumer `0e3e3aba` connects administrator correction to durable job history and required budget settlement.
A review found that pending correction status could hide a new provider dispute. Required settlement now compares billing evidence directly.
The production test verifies retained restriction, refusal before another provider call, and explicit operator resolution.

A second probe found repeated delivery attempts after original reporting expired. The repair retains a separate expiry timestamp without claiming delivery.
The real reporter test verifies that expiry cannot recreate usage or counters.

The [proof](../../plans/proof/starport-production-catalog/csp12.2/job-corrections-2026-09-27/verification.json) preserves failed runs and exact source boundaries.
Final race checks pass 458 job results with one MinIO skip, sixteen adapter results, and four production results.
Pure-Go checks pass 54 results with sixteen Valkey skips. Full CSP12.2 qualification remains open.

### Shared batch aggregate qualification: September 27, 2026

The [shared aggregate proof](../../plans/proof/starport-production-catalog/csp12.2/shared-aggregates-2026-09-27/verification.json) passes at consumer `07578301`.
An older restart test raced its still-active worker. It now drains that worker through `Close` before recovery.
Original output and byte-total assertions remain unchanged. The task gate still lacks 23 registered checks.

### Admission qualification: September 27, 2026

Producer `ab86b3ca2` and consumer `1d9b328f` qualify fourteen CSP12.2 checks. Nine required checks remain unverified.
The [proof](../../plans/proof/starport-production-catalog/csp12.2/admission-qualification-2026-09-27/verification.json) records 134 race results, fifteen pure-Go results, and 99 verifier tests without skips.
The [coverage map](../../plans/proof/starport-production-catalog/csp12.2/admission-qualification-2026-09-27/coverage-map.json) names each remaining requirement.

Separate gateway processes share five applicable meters through real Valkey and PostgreSQL.
Process loss after dispatch retains uncertain capacity. A replacement gateway cannot spend that capacity or repeat the provider call.
This evidence does not qualify storage replacement or background recovery workers.

Production tests also retain recognition charges after caller cancellation and charge embeddings on semantic-cache hits.
Fallback tests retain earlier uncertain capacity. UTC tests cover exact window boundaries and leap February.
The remaining matrix, storage failover, backend operation counts, retention horizons, capacity, native CI, and paired merges remain required.
The September 28 decision below resolves the then-pending restart policy.

### Reporting failures and budget operations: September 27, 2026

The [qualification proof](../../plans/proof/starport-production-catalog/csp12.2/reporting-operations-2026-09-27/verification.json) covers failed usage writes, rejected HTTP exports, and both faults together.
Complete measured usage remains charged. Missing usage retains its full reservation.
Every case refuses another provider dispatch when capacity is insufficient.

A warm request with five applicable meters uses 22 native Valkey commands and six PostgreSQL approval queries.
Nineteen calls precede provider dispatch. Nine calls settle measured usage.
The test verifies real shared storage and production HTTP execution.
These counts exclude unrelated storage work, connection setup, conflicts, and recovery. They do not establish latency or capacity.

CSP12.2 must evaluate bounded grouped reads without weakening independent approval or uncertain reservation retention.

### Grouped budget reads: September 27, 2026

The [qualification proof](../../plans/proof/starport-production-catalog/csp12.2/grouped-reads-2026-09-27/verification.json) reduces a warm five-meter budget operation from 28 backend calls to fifteen.
Ten calls precede provider dispatch. Five calls settle usage. All six independent SQL approval queries remain.
This measurement does not qualify elapsed latency, capacity, or allocations.

Storage returns an ordered snapshot with a maximum of sixteen unique keys and one MiB of possible payload.
The caller bounds each record before copy or transfer. Errors return no partial batch.
Badger uses one read transaction. Valkey uses one incarnation-checked EVAL.

Each conditional-write retry gets a new snapshot. No snapshot or budget balance carries permission into another request.

Native tests cover grouped-read consistency and refusal under a replacement backend.
Reservation tests preserve concurrent settlement, history continuity, expiry refusal, and conditional-write conflict recovery.
The full paid-operation matrix, production storage failover, retention horizons, capacity, native CI, and paired merges remain required.

### Responses and audio: September 28, 2026

Production HTTP tests found two defects in the declared audio path.
The embedded Whisper records identified a text model, so routing rejected transcription and translation before admission.
After catalog repair, provider authentication replaced the multipart content type with JSON and prevented file decoding.

Producer `5ee053080` corrects both Whisper records and regenerates the bound payload and endpoint projection.
Consumer `d231abea` preserves encoder content types and tests multipart bytes through the provider connector.

The [proof](../../plans/proof/starport-production-catalog/csp12.2/responses-audio-2026-09-28/verification.json) retains both failures and their regression tests.
Eighteen production HTTP results, 435 connector results, five catalog checks, and 22 pure-Go results pass without skips.
Responses reserves once per attempt. Uncertain usage retains capacity. Audio requests with required budgets refuse unknown billing bounds before dispatch.
Confirmed absent budgets permit transcription and translation through valid multipart requests.

The full paid-operation matrix and seven remaining CSP12.2 checks remain unverified. This increment uses no Docker service or paid provider.

Consumer `7f5a4756` adds production batch evidence for chat, Responses, and embeddings.
Ten race results and ten pure-Go results pass. Distinct line identities bind reservations to the caller and both meters.

Measured usage settles ordinary provider charges. Missing usage retains capacity and prevents another dispatch.
Insufficient capacity records per-line HTTP 402 errors. Retry and restart qualification remain open.

## Preset order and unsupported budget modes: September 28, 2026

Production batch tests found that preset projection omitted the primary model from the complete routing list.
Consumer `d5f05496` preserves that model and tests provider order, attempt identity, uncertain reservations, and fallback refusal.

The configuration loader silently ignored a requested admission mode.
It now accepts `STARPORT_BUDGET_ADMISSION_MODE=atomic` and refuses other modes before startup.
Standalone and shared configuration tests use the same validation. No local quota lease implementation exists.

Producer `91e9ee1e7` registers the operation matrix and quota-lease boundary.
Both adapters pass in the [proof](../../plans/proof/starport-production-catalog/csp12.2/followup-modes-2026-09-28/verification.json).
Five task checks remain unverified. The sixteen earlier passes remain historical until the full task gate runs again.

The final configuration and proxy suites pass 642 race results with four backend skips.
Focused application tests pass 33 race results. Pure-Go checks pass 47 results without skips.
The proof preserves fixture failures, source identities, skipped checks, and current limitations.
Docker remains stopped. No paid provider request or publication occurred.

## Replacement recovery prerequisite: September 28, 2026

The [production replacement probe](../../plans/proof/starport-production-catalog/csp12.2/shared-recovery-2026-09-28/verification.json) uses PostgreSQL and two independent Valkey processes.
Concurrent settlement and gateway process-loss subtests pass. Replacement startup fails after explicit budget authority approval.
The error reports consumed catalog bootstrap permission and requires recovery.

The failing production restart assertion remains mandatory. The current evidence does not establish a runtime defect or a complete catalog snapshot.
Inspect the original publication and copied inventory before selecting the repair.
CSP12.2 owns the accounting probe. CSP13 owns complete restore and catalog adoption.
Neither task can treat budget approval as sufficient catalog recovery.

Fourteen media and settlement tests map to 88 passing results, including real Valkey approval checks.
The complete task gate, native CI, and paired publication remain unverified.

## Catalog adoption gap confirmed: September 28, 2026

The corrected backend replacement fixture publishes and accepts its catalog before copying the stopped deployment.
It explicitly disables acquisition and preserves canonical settings from the loader.
The replacement gateway still refuses startup after budget approval, with `invalid fleet acceptance record`.
The copied publication and acceptance retain the old recovery identity.

`FleetStore.CurrentHead` requires the approved backend and epoch.
`readAcceptance` enforces the same identity. `ApproveAuthority` only adopts the budget authority.
No catalog adoption operation currently connects these contracts. Resetting bootstrap permission would not repair this mismatch.

The [recovery contract](../../plans/proof/starport-production-catalog/csp12.2/catalog-recovery-prerequisite-2026-09-28/CONTRACT.md) moves the necessary prerequisite into CSP12.2.
CSP13 retains the complete migration and operator tooling scope.
The original and corrected failures remain evidence. Successful backend recovery remains unverified.

## Controlled catalog adoption implemented: September 28, 2026

Producer commit `35264ea32` and consumer commit `f238cc27` add explicit catalog adoption under a closed recovery gate.
The [current proof](../../plans/proof/starport-production-catalog/csp12.2/catalog-adoption-2026-09-28/verification.json) closes the production replacement startup failure.
The test retains five uncertain reservations and refuses another request until explicit reconciliation permits it.
The catalog keeps its original publication evidence and receives a separate recovery receipt.

Native tests reject missing or corrupt retained data, conflicting operation evidence, and unresolved reader claims.
They recover pending uploads and retry after lost catalog-selection or native-approval responses.
The approved retry preserves a later ordinary publication. An obsolete retry cannot reopen a newer closed epoch.
Authority tests confirm that adoption grants no inference permission and does not undo a known withdrawal.

The changes remain unpublished. Remaining adoption cases, native CI, full task verification, review, and paired merges remain required.


### Batch restart and correction horizon: September 28, 2026

Consumer `1fecf04` implements the 90-day correction horizon from original settlement.
Consumer `501cb0d` resumes only proven-unstarted batch lines under current authorization and normal budget admission.
Completed results survive restart. Uncertain attempts never repeat automatically.
Accepted exact correction retries remain idempotent after expiry. Unresolved reservations never expire through retention cleanup.

The [correction proof](../../plans/proof/starport-production-catalog/csp12.2/correction-horizon-2026-09-28/verification.json) records reservation and integration checks.
The [restart proof](../../plans/proof/starport-production-catalog/csp12.2/batch-resume-2026-09-28/verification.json) covers process loss, withdrawn permission, native storage, and production recovery.
The [retention proof](../../plans/proof/starport-production-catalog/csp12.2/retention-acceptance-2026-09-28/verification.json) verifies that missing or expired evidence cannot refund consumption.
Complete task qualification, native CI, required review, and paired merges remain open.


### Backup capture boundary: September 28, 2026

Consumer `a01d8c44b` connects canonical file selection and native store snapshots to operator capture commands.
It rejects changed loaded configuration and verifies bundles without opening live stores.
The [proof](../../plans/proof/starport-production-catalog/csp13/inventory-capture-2026-09-28/verification.json) records the checks and retained fixture failures.

Badger rejects read-only opens on Windows.
Windows capture therefore uses an exclusive native open with application maintenance disabled.
It refuses a missing database manifest before that open.
Cross-compilation passes, but native Windows capture remains UNVERIFIED.

Capture does not establish independent recovery history or validate every historical encrypted credential.
The SQL close command does not stop processes. Operators must separately stop and fence writers.
Reference validation, full restore, required review, native CI, and merge remain open.


### Backup reference validation: September 28, 2026

Consumer `339883cfa` adds owner-specific backup checks.
Artifact digests and a selected-key challenge do not prove usable application state.
A regression fixture captures intact bundles with missing file bytes or a credential encrypted under another key.
The byte-only verifier accepts those bundles. The reference verifier refuses them.

Batch results can retain a digest before their output file commits.
Validation preserves that interrupted state. Missing execution claims cause refusal.
The full owner suites pass without optional storage skips.
SQL references, independent later history, full restore, native qualification, review, and merge remain open.

The [reference proof](../../plans/proof/starport-production-catalog/csp13/reference-validation-2026-09-28/verification.json) records 47 focused race passes, 47 pure-Go passes, and 939 full owner-suite race passes.
All three cohorts have no failures or skips.


### SQL identity reference validation: September 28, 2026

Consumer `31616bb68` validates captured SQL identities.

The previous reference verifier accepted a SQL user row with invalid record JSON.
The retained failing regression now passes through owner-specific SQL validation.
The shared private-image reader also serves relational import, with all nine backend pairs still passing.

An initial template validator reused the smaller authorization-record bound.
The corrected validator accepts valid larger templates within the 64 MiB portable-record bound.
Gateway-key and budget references, independent later history, full restore, native qualification, review, and merge remain open.

The [SQL reference proof](../../plans/proof/starport-production-catalog/csp13/identity-references-2026-09-28/verification.json) records 35 focused race passes, 35 pure-Go passes, and 318 broader passes.
All three cohorts have no failures or skips.


### Gateway-key and budget references: September 28, 2026

Consumer `e02671ac9` validates captured key and accounting references.

The previous verifier accepted a captured gateway key after the fixture removed its hash index.
The selected-verifier regression preserves that failure, and the owner check now refuses the backup.
Deleted initial keys and teams can leave legitimate retained records. Verification preserves these states and reports their meaning.
Consumed team grants without matching history remain unknown rather than fresh capacity.

The reservation test imported the recovery coordinator. The new coordinator dependency exposed a test import cycle.
The real-storage integration test now lives with the coordinator and retains its six meter assertions.
Aggregate accounting checks, complete correction ancestry, independent later history, and full restore remain open.

The [reference proof](../../plans/proof/starport-production-catalog/csp13/security-references-2026-09-28/verification.json) records 623 race passes and 72 pure-Go passes.
Both cohorts have no failures or skips.


### Accounting consistency: September 28, 2026

Consumer `bca519227` validates captured balances and correction chains.

The previous verifier accepted changed window totals, missing attempts, changed correction heads, and orphan correction receipts.
Six failing regression subtests preserve those gaps.
The implementation now checks accounting conservation through a private on-disk index.

Review found that ordered absolute correction timestamps would reject state that the runtime permits.
The repair uses immutable state bindings and retains each correction deadline.
The backward-time regression fails before that repair and passes afterward.
Cross-store execution links, independent history, and full restore remain open.

The [accounting proof](../../plans/proof/starport-production-catalog/csp13/accounting-consistency-2026-09-28/verification.json) records test counts, retained failures, and final qualification scope.


### Job accounting links: September 28, 2026

Consumer `1e69e4f16` validates captured job and correction links.

Four baseline regression cases reproduce accepted job/reservation mismatches.
They cover generation, reservation identity, absent reverse binding, and valuation.
The verifier now checks those links and retained correction histories.

Review confirmed two valid states that inspection must preserve.
Job creation binds the reservation first. Permitted deletion can also leave a reservation without its job.
Budget evidence can exist without duplication in a job.

Missing jobs remain diagnostics. Conflicting identities or retained evidence cause refusal.
Independent later history and full restore remain open.

The [job accounting proof](../../plans/proof/starport-production-catalog/csp13/job-accounting-links-2026-09-28/verification.json) records exact results and retained fixture failures.


### Restricted SQL restore: September 28, 2026

Consumer `f5d5558ff` retains transactional SQL import receipts.

The original relational importer refused an exact retry after a successful import.
The new wrapper's baseline test reproduces that target-not-fresh result.
The receipt now resolves a lost acknowledgement without repeating recovery restrictions.

All three SQL backends pass retry, concurrent ownership, and rollback tests.
The coordinator also checks closed gates after process reopen and refuses changed target permissions.
Backup capture refuses a pending SQL import, so partial recovery cannot become a new accepted backup.
Complete bundle preparation and independent recovery history remain open.

The [SQL restore proof](../../plans/proof/starport-production-catalog/csp13/restricted-sql-restore-2026-09-28/verification.json) records exact results and qualification limits.


### Restricted bundle preparation: September 28, 2026

Consumer `2a4e9216d` connects the retained storage adapters.

The original one-shot filesystem blob importer cannot resume after a lost acknowledgement.
The new exact-retry wrapper verifies the retained import receipt and every object.
The old one-shot API retains its existing-path refusal.

Local and shared bundle recipes now pass complete preparation and retry checks.
Injected failures before and after blob import and after KV import retain barriers and resume without repeating SQL restrictions.
Changed SQL gates prevent completion. Changed staged files cause refusal on retry.
Native identity checks detect path aliases into protected source trees.

The implementation remains a library operation. Canonical file placement, independent history, and operator recovery commands remain open.

The [bundle preparation proof](../../plans/proof/starport-production-catalog/csp13/bundle-prepare-2026-09-28/verification.json) records exact results and qualification limits.


### Operator restore preparation: September 28, 2026

Consumer `7237a692c` exposes restricted preparation.

The missing command test fails before the CLI registers `backup prepare`.
The command now reaches native application preparation and retains restricted state across exact retries.
Local and shared recipes verify SQL, KV, blob barriers, and inactive selected files.
Wrong source digests, keys, deployment IDs, and overlapping targets cause refusal before target creation.
Changed fencing evidence cannot reuse a retained import.

Native schema preparation covers SQLite, PostgreSQL, and MySQL.
Final package results contain 171 race passes and 67 pure-Go passes.
The evidence preserves initial compile and fixture failures and identifies the final storage-only reruns.
Independent-history and activation commands remain open.

The [operator preparation proof](../../plans/proof/starport-production-catalog/csp13/restore-command-2026-09-28/verification.json) records exact results and qualification limits.

The final review found that SQL NULL metadata could escape an inequality count.
The regression proves that the preflight then changed the schema before refusing.
Explicit NULL checks now refuse without changing the target schema.


### Restore inventory validation: September 28, 2026

Consumer `3c48670d5` validates product inventory before target access.

Eight malformed product inventories passed preparation despite valid outer bundle hashes.
The regression fixture changes inventory metadata and recomputes outer hashes to isolate that gap.
New validation refuses these cases before target creation. A ninth case checks unknown JSON members.

The final focused suites record 75 race passes and 75 pure-Go passes without failures or skips.
The broader configuration and CLI suite records 529 passes and one optional container-image skip.
Twenty configuration tests pass under race detection and pure Go after the final constant rename.
The report identifies required file procedures. Actual file publication and activation remain open.

The [inventory proof](../../plans/proof/starport-production-catalog/csp13/file-plan-2026-09-28/verification.json) records exact results and remaining requirements.


### Canonical inference-policy publication: September 28, 2026

Consumer `b4b83664` adds `backup publish-files --role inference-credential-policy`.
The command verifies or resumes restricted preparation before publishing a canonical policy tree.
Current target configuration selects the destination. The credential owner validates the captured deployment and replica identity.
Unsupported roles, conflicting targets, pending journals, and different replica identities cause refusal.
Exact retries verify the complete existing tree without replacing it.

The operation preserves accepted provider choices and the legacy default.
KV, SQL, and blob barriers remain closed. Administrator credentials and other file roles retain their separate owner procedures.
Independent later history and activation remain required. This local checkpoint does not qualify complete recovery.

The [owner publication proof](../../plans/proof/starport-production-catalog/csp13/owner-publication-command-2026-09-28/verification.json) records source, tests, and remaining requirements.

### Acquisition-policy recovery and Windows copy tests: September 28, 2026

Starmap now exposes a read-only credential-policy inspector. Starport uses it for the separate acquisition-policy publication procedure.
The inspector validates the retained default, accepted provider records, native access, ownership, and pending publication state.
Inspection does not create a missing default or consult provider credentials.
Both policy restore paths preserve closed admission barriers in tests with Valkey, PostgreSQL, and object storage.

The Windows amd64 runtime job for Starmap #194 failed while its test tried to rename an open directory.
Windows returned a sharing violation before the test could replace that directory.
The corrected Windows test requires that protection, verifies complete output, and checks handle release after copying.
It then replaces the directory and requires the original handle owner to refuse a new copy.
POSIX retains its replacement-during-copy refusal test. Cross-compilation does not qualify native execution.

The [acquisition-policy proof](../../plans/proof/starport-production-catalog/csp13/credential-policy-publication-2026-09-28/verification.json) preserves the failed native evidence and subsequent checks.
Full recovery, activation, and final native qualification remain open.

The broader architecture suite exposed two stale import contracts from earlier CSP13 work.
Recovery now composes domain-owner checks over captured state. Batch recovery reads file records through the file owner.
The contract now names those owners explicitly. It still rejects application, provider, and request orchestration imports at these boundaries.
The production validators and their record checks remain unchanged.

### Local administrator recovery: September 28, 2026

The existing rotation command printed the new administrator secret. Its new `--no-secret` option reports only metadata.
Existing default output remains unchanged. Both text and JSON metadata modes rotate the stored credential.
Tests verify that neither secret enters output and that old signed sessions fail against the replacement token.

A native local-storage test captures a valid old token, prepares the backup, and rotates the target credential through the CLI.
The test then retries preparation and verifies that the fresh target token and original source token remain unchanged.
KV, SQL, and blob barriers stay closed. The independent recovery witness also stays closed.
Full recovery and activation remain open.

The [local-access proof](../../plans/proof/starport-production-catalog/csp13/local-access-recovery-2026-09-28/verification.json) also records the actual Starport #390 merge and its integration.

### Completed baseline export recovery: September 28, 2026

The existing exporter writes the installed generation and can recover local journals.
It could not validate a retained export without writes or without requiring the current binary's generation.
The new Starmap inspector separates that read-only check from export and recovery.
It uses the catalog generation decoder rather than duplicating schema and membership rules in Starport.

Starport now has a candidate baseline file-publication path through the catalog owner.
Its integration tests use a real embedded export, Badger, SQLite, and filesystem blob storage.
They cover publication at the configured destination, exact retry, conflicting targets, corrupt payloads, and unfinished stages.
The successful path keeps captured journal files inactive and every storage barrier closed.
The installed exporter can subsequently verify the same generation with fresh journal ownership.

This checkpoint does not recover runtime identity, replay floors, or path-bound journals.
Independent later history, admission, and full recovery remain unfinished.

Publish the producer API before final consumer qualification. Use the published module without a local replacement.

### Retained runtime publication: September 28, 2026

The retained runtime contains more than its owner record. It also holds its instance seed, catalog inputs, pending publication references, and upstream replay evidence.
The new Starmap inspector validates these through their existing owners without opening a runtime, reading credentials, or acquiring source data.
Review found that an unsupported discovery schema previously returned empty state. The regression test verifies refusal. The source reader now refuses it and preserves the saved file.

Starport's candidate runtime publication checks the configured owner and scheduler identity before installing the private tree.
Local integration tests use real Badger, SQLite, and file storage. They cover preserved identity, exact retry, invalid seeds, different replicas, and changed scheduler overrides.
The tests verify that all storage barriers remain closed after publication or validation failure.
The different-replica case refuses before target stores exist.

The inspector preserves a structurally valid pending semantic publication without applying it.
Inspection alone refuses path-bound migration and native publication journals. The completed-history procedure below handles matching completed migration records.
Other owner recovery, accepted-catalog consistency, independent later history, and controlled activation remain unfinished.
Producer publication and final qualification against the published consumer module must precede completion claims.

The [runtime evidence proof](../../plans/proof/starport-production-catalog/csp13/runtime-evidence-recovery-2026-09-28/verification.json) records the current checks and remaining qualification.

### Completed migration history during restore: September 28, 2026

The previous runtime publication path rejected all migration records, including records from a completed move.
Copying those records unchanged would leave the replacement tied to paths on the former host.
The new owner check identifies matching completed history without reopening those paths.
It validates identity and seed binding before Starport excludes only the two approved historical records from the active tree.

Tests complete a real runtime move, capture its state, and restore the remaining tree with admission closed.
They preserve the historical records and exact retry behavior. They refuse incomplete history, conflicting history, and a pending move hidden beside completed history.
A Starmap test validates captured records after both former directories move out of reach.
Separate cases retain POSIX, Windows drive, and Windows share path strings without interpreting them as local paths.

The first integration fixture used the full embedded catalog throughout each migration phase.
It was still consuming CPU when stopped. The replacement fixture uses a small catalog through the same real migration, storage, backup, and restore paths.
Dedicated capacity gates retain full-catalog coverage. The stopped run remains historical evidence and does not count as passing qualification.

Starmap #195 merged as `8020da753f2809d32cd4377e94abb7ecdea56b07` after all 32 checks passed.
The tested and merged trees match. The current producer branch includes that merge without changing its tested tree.
An earlier #196 macOS job passed its test steps but failed evidence upload with `ENOTFOUND`.
Its failed status and missing evidence remain recorded. The new head requires complete native qualification.

The [completed migration restore proof](../../plans/proof/starport-production-catalog/csp13/completed-migration-restore-2026-09-28/verification.json) records checks, the actual prerequisite merge, and remaining qualification.

### Native runtime publication restore (September 28, 2026)

Starmap `05501c597` validates bounded canonical journals and their runtime-owned destinations. Starport `100cd6c2b` composes this check with the verified backup inventory.

Captured staging bytes must match the journal size and digest. The captured writer record must be present and empty.
Keep validated journals and staging files in the verified backup and inactive preparation. Report `verified-staging` without an active destination.

Never promote staging bytes or use historical native identities as current cleanup authority. Validate all destination records through their owners before publication.
Changed or unowned staging evidence causes refusal before target preparation. Exact retries retain the same selection, and admission barriers remain closed.
Other journal owners, interrupted moves, independent history, controlled activation, and full native qualification remain open.

The [native publication restore proof](../../plans/proof/starport-production-catalog/csp13/native-publication-restore-2026-09-28/verification.json) records source commits, final pinned tests, review, and remaining work.

### Credential publication recovery (September 29, 2026)

Local Starmap `5aefe198a` owns shared native publication inspection and journaled acquisition-policy writes. Each caller still defines its permitted destinations and staging prefixes.
Restore preserves accepted acquisition and inference policy. Validated journals and matching staged bytes stay in the verified backup and inactive preparation.
Complete legacy acquisition stages require the captured deployment, instance, and policy family. Partial, changed, or foreign stages cause refusal before preparation creates target stores.

Exact retries retain the same selection. Publication does not accept a policy migration or authorize credential use. All admission barriers remain closed.

The consumer implementation passes development and real-storage checks against a local module replacement. Published-module qualification and native checks remain open.
The [credential publication proof](../../plans/proof/starport-production-catalog/csp13/credential-publication-journals-2026-09-29/verification.json) records source hashes, test counts, review scope, and remaining work.

### Baseline publication recovery (September 29, 2026)

Starmap `9ab700ad9` validates the captured baseline and journal inventory. Starport `f8f69220a` uses that published module with workspace overrides disabled.
Canonical journals and an empty writer record identify matching stage files by name, size, and digest. Complete journal updates and interrupted collection retain their evidence.
Completed exports require separate owner validation. Historical native identities never authorize cleanup, and staging bytes never become an active catalog.

Keep staged files as `verified-staging` and journal records as `verified-history`, with no active destination. Exact retries preserve selection and every admission barrier.
Unknown, malformed, or changed stages cause refusal before target preparation. Interrupted path migration, independent history, controlled activation, and full qualification remain open.

The [baseline publication proof](../../plans/proof/starport-production-catalog/csp13/baseline-publication-journals-2026-09-29/verification.json) records the actual #196 merge, new #197, source commits, and final pinned qualification.

### Captured catalog consistency (September 29, 2026)

Starport `d96a11fff` checks catalog references during backup capture, verification, and restore preparation. It reads a verified private KV image before target creation.
Standalone checks bind catalog pointers, chunk descriptors, payloads, and acceptance history. Fleet checks also bind deployment identity, publication receipts, recovery inputs, and adoption records.

Incomplete fleet uploads retain matching bytes as inactive evidence. Durable catalog records cannot expire. Captured leases preserve their recorded expiration and grant no new permission.
These checks establish captured-state consistency. Independent later history, deployment replay, controlled activation, and interrupted path recovery remain separate requirements.

The [catalog reference proof](../../plans/proof/starport-production-catalog/csp13/catalog-reference-validation-2026-09-29/verification.json) records fail-before results, source commits, the actual #197 merge, and native qualification.

### Recovery CI coverage (September 29, 2026)

The previous workflow did not run the new backup tests with shared storage fixtures.
Starport `1d5ea68ab` adds the storage, catalog, and application tests to the required recovery job.
It supplies pinned PostgreSQL, MySQL, and two Valkey instances alongside the existing object store.

Local checks pass all nine SQL transfer pairs. The [readiness proof](../../plans/proof/starport-production-catalog/csp13/recovery-ci-readiness-2026-09-29/verification.json) retains the exact counts and skipped replacement tests, which passed separately.
Native CI and full deployment recovery remain unverified for this consumer branch.

The first full Go suite reached its ten-minute package limit. Aggregate local and release commands now match the existing native 30-minute allowance.
Individual contract deadlines remain unchanged.

The final Go suite passes 5,974 results with 369 skips.
Starport `3f5aa9b4c` removes the duplicate full-suite invocation from the architecture command. Eleven focused contracts pass.
Separate full-suite requirements remain in local checks, native CI, and release verification.

The operator guide previously claimed runtime migration never opened SQL. Badger migration leaves SQL closed, but shared migration requires the existing PostgreSQL recovery witness.
The guide now states that distinction. Incomplete migration backup records still cause refusal before runtime publication.

Draft [Starport #391](https://github.com/agentstation/starport/pull/391) publishes the reviewed head.
The required review reports no findings across four parts at the configured P0-only threshold. Native CI remains in progress.

### Windows recovery publication (September 29, 2026)

Native Windows ARM tests at Starport `3f5aa9b4c` produced 72 failed results.
SQLite backup publication retained an open staging-directory handle during rename.
Go opens that root without delete sharing on Windows. Native rename therefore failed.
The same handle lifetime existed in blob restore and both recovery-file publishers.

Repair `6b36dfdf5` closes each staging root before publication and retains identity checks and exclusive destination publication.
Cleanup still removes only the original staging directory.
Existing publication, concurrent-writer, process-loss, and replacement tests cover these paths.

Local storage suites pass 333 results in race mode and 333 in pure-Go mode, each with 48 service-dependent skips.
Application recovery passes 73 race results with three service-dependent skips.
The workflow now runs storage recovery before long runtime suites. All 76 packages still run once.
The [repair proof](../../plans/proof/starport-production-catalog/csp13/windows-directory-publication-2026-09-29/verification.json) retains the failed native evidence and local checks.
Fixed-source native qualification remains required before merge.

The next Windows storage run passed 332 results and failed one parent-replacement test.
The test assumed that an open parent could move during validation. Windows correctly refused the rename with `ERROR_SHARING_VIOLATION`.
Repair `d9b502da2` preserves the POSIX replacement case and verifies Windows handle protection, unchanged bytes, and handle release after the operation.
No production code changed in this repair. The Windows case remains required.

The [parent contract proof](../../plans/proof/starport-production-catalog/csp13/windows-parent-contract-2026-09-29/verification.json) records 33 local race passes, Windows ARM compilation, and the fresh review.
PR #391 publishes the reviewed repair. Its native checks remain open.

### Canonical journal fixtures and namespace capture (September 29, 2026)

Windows ARM at `d9b502da2` passed 1,134 results and failed six application results. Git converted two canonical journal fixtures to CRLF.
The validators correctly refused those bytes. Published `90ad16c2d` pins both fixtures to LF without changing production code or assertions.

Forced CRLF checkout now preserves their exact repository bytes. Seven focused application race results pass.
The [checkout proof](../../plans/proof/starport-production-catalog/csp13/windows-journal-checkout-2026-09-29/verification.json) retains native failures, the repair, and required review evidence.
Replacement native qualification and merge remain open.

Local `fa214f6a5` adds explicit unprefixed Valkey capture. Ordinary startup and capture retain canonical namespaces.
The source exposes enumeration only and binds its observed backend incarnation. Mixed deployment namespaces and import barriers cause refusal.

The manifest records the selected source layout. Capture and verification report counts and validate references.
Preparation imports into the canonical target namespace under startup barriers. Exact retries preserve the result and original source bytes.

The [local proof](../../plans/proof/starport-production-catalog/csp13/unprefixed-capture-2026-09-29/verification.json) records 27 storage and CLI passes and two application passes without skips.
This branch remains unpublished. Required review, full repository checks, independent history, and controlled activation remain open.

### Recovery activation and server TLS (September 29, 2026)

Starmap #198 merged as `4bf9b0b83` after all 32 checks passed. The reviewed tree equals the merged tree.
The [merge proof](../../plans/proof/starport-production-catalog/csp13/starmap198-merge-2026-09-29/verification.json) records unchanged branch protection and the actual commit.

The activation audit found that Starport ignored selected server TLS material. Its HTTP runtime always served plaintext.
Production composition also opened storage before rejecting invalid certificate material. The fail-before tests reproduce both defects.

Local server regression tests now pass. The configuration helper checks native file access, bounds each input, and parses the matching certificate pair.
Application composition must pass that pair to the HTTPS listener before any storage or setup effects.
The complete repair still needs native CI and publication.

Starport #393 native Windows reached the default ten-minute recovery package timeout. The captured run contains no assertion failure before that timeout.
Its repair partitions the complete Windows recovery suite and checks the union of all test owners.
The [activation findings](../../plans/proof/starport-production-catalog/csp13/activation-findings-2026-09-29/verification.json) retain the original failures and current repair ownership.

### Native recovery guards and startup ordering (September 29, 2026)

Local consumer `49d791c7` integrates checked operator inputs, actual HTTPS serving, and a native SQL guard.
The [guard proof](../../plans/proof/starport-production-catalog/csp13/sql-closed-guard-2026-09-29/verification.json) records 94 passing results per mode across SQLite, PostgreSQL, and MySQL.
Independent Badger and Valkey commits survive lost callback replies. Their exact native receipts support retry while admission remains closed.

KV and SQL activation originally bound import claims without the final replay cursor. The new strict procedures check that cursor inside native release.
The [activation proof](../../plans/proof/starport-production-catalog/csp13/position-activation-2026-09-29/verification.json) records 107 passing results per mode and preserved fail-before evidence.
Complete coordinator qualification remains open.

The startup audit found maintenance and inference-policy effects before the selected SQL import barrier check.
After KV release during partial recovery, this order can change canonical state before SQL refuses startup.
CSP13 owns an early read-only recovery preflight. It must preserve normal fresh startup and prevent these effects while recovery remains incomplete.

Closed final-history checks require active import barriers. They cannot reconstruct the original capability after partial barrier release.
Canonical file publication also repeats preparation and cannot serve as the post-release restart verifier.
The activation coordinator needs retained owner receipts and phase-specific verification. Component release alone does not prove complete recovery.

### Native receipts and reviewed recovery components (September 29, 2026)

Starport #393 merged as `b256a14f` with 31 passing checks. Starmap #199 merged as `af57ed3f` with 32 passing checks.
Both merges preserve the reviewed tree and branch protection. They do not complete CSP13.

Strict native activation receipts now retain the approved final replay position. Passive checks do not repeat owner changes or grant permission.
The [native proof](../../plans/proof/starport-production-catalog/csp13/native-activation-inspection-2026-09-29/verification.json) records 117 results per mode.
Bounded catalog reads pass 26 results per mode. They read one record under exact native recovery guards.

The local consumer includes the [startup preflight](../../plans/proof/starport-production-catalog/csp13/startup-preflight-2026-09-29/verification.json).
It reads selected SQL before setup and joins native owners before maintenance or policy writes. Integrated race and pure-Go checks each pass 26 results.
These consistency checks cannot replace the complete retained activation decision.

Starmap [PR #200](https://github.com/agentstation/starmap/pull/200) adds structural retention and passive selected-input checks.
Its [proof](../../plans/proof/starport-production-catalog/csp13/catalog-retention-2026-09-29/verification.json) records complete checks and a clean isolated review. Native CI remains pending.
The consumer uses the published module without a replacement. All twelve changed module files match the reviewed producer bytes.

The topology review found incomplete selected-publication binding when different inputs share a generation ID.
The final inventory identity encoding also lacked a bound after the compiler added archive metadata.
Both repairs belong to the catalog owner before integration. Complete native activation and measured recovery remain open.

### Passive restart and exact final approval (September 29)

Starport `92b3b5f4` integrates the reviewed topology compiler, passive retained history, strict fleet approval, and passive operator-input reopen. The [topology proof](../../plans/proof/starport-production-catalog/csp13/catalog-topology-integration-2026-09-29/verification.json) records 21 integrated results per mode. It preserves the failures that exposed same-generation selection substitution and unbounded final inventory metadata.

The [retained-history proof](../../plans/proof/starport-production-catalog/csp13/retained-activation-history-2026-09-29/verification.json) records 129 results per mode. It validates original package, journal, image, and final graph evidence without native replay. Catalog-mode journal integration remains open.

The [fleet approval proof](../../plans/proof/starport-production-catalog/csp13/imported-authority-position-2026-09-29/verification.json) records five results per mode across real Valkey and three SQL owners. It binds approval to the final SQL position and exact closed boundary. Later withdrawal refuses approval while passive inspection still verifies historical completion.

The [operator-input proof](../../plans/proof/starport-production-catalog/csp13/operator-input-inspection-2026-09-29/verification.json) records 30 results per mode. Passive reopen rejects missing or changed original evidence without publication or repair. Each final check has no failures or skips. These components do not qualify complete recovery. The catalog lane, local completion, canonical-file inspection, application decision, and process-loss matrix remain open.

### Retained component integration and native test timeout (September 29, 2026)

Starport `17ddf17c` integrates passive canonical-file inspection, strict Badger and SQLite completion, and the journaled catalog preparation contract.
Their source checks pass 62, 30, and 27 results per mode. Root review precedes each integration.
The catalog bridge preserves the original run declaration after native barriers disappear. Missing or corrupt original assets still cause refusal.

Starmap `a23de11a` separates current permission from structural retained-directory inspection.
The [permission proof](../../plans/proof/starport-production-catalog/csp13/catalog-permission-inspection-2026-09-29/verification.json) records 69 results per mode and complete repository checks.
Passive validation preserves original authority expiry, checkpoint uncertainty, source policy, and native record identities. It issues no permission and starts no acquisition.

Starmap PR #200 passes 31 native checks and reaches the Windows runtime package timeout.
The active source-reset subtest starts 2.674 seconds before the timeout. Its stack does not prove a thirty-minute individual test hang.
The [failure proof](../../plans/proof/starport-production-catalog/csp13/starmap200-native-timeout-2026-09-29/verification.json) retains the original artifacts.
The repair partitions the complete inventory and preserves required aggregate check names. Native qualification requires the reviewed replacement head.

The combined Starport integration run reaches its six-minute limit after 184 passing results.
Separate contract groups retain the same tests and limit. The recovery-owner race group passes 189 results. Catalog and pure-Go integration remain pending.

A separate local task clears the default Go cache during compilation. An owned cache removes that build interference without changing source assertions.
Complete application activation, process-loss qualification, and deployment recovery objectives remain open.

## Published recovery contracts, September 30, 2026

Starmap PR #200 now publishes `2453d21b72f91dc7597bedc672ad4dc0b234a690`. The final isolated review reports no actionable findings. Canonical option resolution supplies the passive permission inspector without another settings parser. The artifact-pattern regression prevents Linux ARM artifacts from entering the Linux x86 aggregate. Final native Actions remain unverified.

Consumer `8726cc0b` integrates application canonical-file checks from `8997b1ec`. Its source contract passes 45 results per mode. A fresh child process checks sealed original evidence after a real blob release without target writes. Final integrated qualification remains required.

The unchanged full embedded-catalog test passes with published Starmap `975bcb6426ca`: 355.622 package seconds under race detection and 25.186 with pure Go. The earlier combined five-minute run timed out. These durations came from concurrent checks and do not qualify deployment RTO or maximum capacity. The proof retains source and dependency identities.

The source-file selection contract previously omitted file contents and native identity. It now retains both, using Starmap's catalog payload bound. Configuration and token files retain their smaller bound. Selected YAML workspace recovery still needs its owner procedure. Complete coordinated activation and the fresh-process loss matrix remain open.

## Original backup input qualification, September 30, 2026

Starmap #200 merged as `cf45526ef644d56642d61920c4dde7751d9012d2`. All 62 final checks pass. The merge proof preserves the exact reviewed tree and branch protection. Earlier timeout and pending-check snapshots remain historical evidence.

Starmap #201 publishes `86c169a0d93be9d4d2941caa4f6122f7a0b84553`. Portable inspection validates actual producer descriptors, baseline archives, and immutable retained envelopes without opening a runtime. Both new regression probes fail against the prior implementation. Final contract runs pass 50 results each with race detection and pure Go, without failures or skips. Complete repository checks pass.

The isolated pre-PR review is clean at its configured P0 threshold. Native CI remains pending. The [component proof](../../plans/proof/starport-production-catalog/csp13/captured-catalog-publication-2026-09-30/verification.json) preserves intermediate failures and final source hashes.

Starport `631ce329` preserves a prepared SQL boundary with an empty backend identity and validates the future fleet identity separately. Its five results per mode pass. `e53d174f` exposes checked original backup boundaries, KV snapshots, and bounded payload reads. Its 30 results per mode pass. Its initial 256 MiB payload cap needs the producer-owned 512 MiB envelope correction before full integration. These tests do not qualify maximum-capacity transfer.

Starport `77b4eade` projects canonical passive recovery settings. Its five results per mode pass without runtime creation or acquisition.

 `34ecd941` corrects the lint proof after an initial process-lock refusal. The terminal pinned lint run passes. The original failure remains disclosed.

The original-backup compiler and complete application coordinator remain under qualification. Existing topology transfer covers two cross-topology directions. Same-backend complete recovery still needs its own checked path and acceptance evidence. CSP13 remains in progress, with one required subcase passing and fifteen unverified.

## Exact phase write and expiring lease findings, September 30, 2026

Starmap #201 now publishes `79852bf7ddee936c4dee2a741f1c1c5c4d30dadb`. The published Go module contains the portable archive readers and exact pending-write inspection and completion APIs. GitHub verifies the commit signature. Native run `36680574306` qualifies that final head. Workflow concurrency cancelled the preceding run after the update. Its earlier results remain historical evidence.

The complete private-file and public host-file owners pass 183 results each under race detection and pure Go. Actual child processes stop with header, empty, prepared, and published journals. Completion retains exact phase bytes and checks original native evidence. Foreign, changed, unowned, or incomplete stages remain pending after refusal.

A regression proves callback mutation cannot alter the requested bytes. Complete repository checks and the final isolated P0 review pass. The [proof](../../plans/proof/starport-production-catalog/csp13/exact-phase-publication-2026-09-30/verification.json) preserves the complexity failure and final results.

The fleet compiler also exposed expiring control records. Native transfer retains absolute expiry, omits expired records, and refuses expiring values in ordinary persistent comparison. Treating original leases as ordinary retirement preimages would therefore refuse valid recovery. The native owner owns the separate deletion-only repair with original expiry and exact replay receipts. Its full coordinator and real-storage qualification remain open.

## Integrated original backup census, September 30, 2026

Starport `b016190f` integrates the original-backup compiler. `52f9455d` pins the final published producer module. The compiler now covers both same-backend directions and the producer-owned 512 MiB envelope bound. It derives original inventory, reconstruction inputs, and ordered stages from the verified backup. Missing or ambiguous historical inputs refuse activation.

The [compiler proof](../../plans/proof/starport-production-catalog/csp13/original-backup-census-2026-09-30/verification.json) records 37 passing results per mode, without failures or skips. Race qualification separates 32 small-contract results from five actual embedded-backup results. The combined six-minute timeout remains historical evidence. The split preserves every assertion and timeout.

Root independently passes 18 pure-Go results and three boundary results in each mode against the final published module. The first selector did not match every boundary test, so the separate command covers those tests explicitly. The original-source, same-backend, and passive-settings checks preserve their exact selectors and counts.

Complete native activation, process-loss recovery, maximum capacity, and RPO/RTO remain unverified. CSP13 still has one passing required subcase and fifteen unverified subcases.

## Native expiring control retirement, September 30, 2026

Starport `fd3b0ba7` integrates the deletion-only native retirement owner. Original bytes, absolute expiry, claim, incarnation, and replay cursor bind durable receipts. Both Badger and Valkey reject changed values, changed expiry, and unexplained absence. Exact retries preserve later state. Generic persistent comparison remains unchanged.

The [native owner proof](../../plans/proof/starport-production-catalog/csp13/expiring-import-owner-2026-09-30/verification.json) records 42 passing results per mode. Root independently reproduces those counts against actual stores. Close-and-reopen tests preserve the original receipt and closed import barrier. Final application composition and native Linux/Windows qualification remain open.

The application fixture also found a missing stopped-directory preparation API. Producer tests establish owner, seed, and inert layer directories through private functions before materialization. The consumer cannot copy those private codecs or open an acquiring runtime. The producer must expose explicit preparation before the application seals its release decision. Restart inspection remains passive.

## Starmap archive and phase API merge, September 30, 2026

Starmap #201 merged as `c384ccb103fbd64e5d1fba5333f486c669ad74ba`. All 62 checks pass at reviewed head `79852bf7ddee936c4dee2a741f1c1c5c4d30dadb`. The [merge proof](../../plans/proof/starport-production-catalog/csp13/starmap201-merge-2026-09-30/verification.json) records exact tree equality, verified head signature, resolved review threads, and unchanged strict protection. Earlier pending snapshots remain historical evidence.

The stopped-directory preparation API now has a separate unpublished follow-up branch. It builds on the merged producer tree. Its current owner tests pass 23 results per mode, without failures or skips. Full repository checks and pre-PR review remain required before publication.

## Integrated typed expiry stages, September 30, 2026

Starport `ed404cea` integrates compiler delivery `75528147`. `24c52eca` integrates the native receipt presence correction. The [integrated proof](../../plans/proof/starport-production-catalog/csp13/expiring-catalog-stages-2026-09-30/verification.json) records 32 compiler and 43 native results per mode. Root reproduces these counts without failures or skips.

The compiler checks actual persistent Badger import and selection. The native owner checks actual Badger and Valkey retirement, with durable close-and-reopen retries. An explicit presence field prevents nil and empty byte slices from sharing a receipt identity. The original failing regression remains in the proof. These results do not qualify the application's SQL guard, release ordering, process-loss matrix, or recovery objectives.

## Explicit stopped-directory preparation publication, September 30, 2026

[Starmap #202](https://github.com/agentstation/starmap/pull/202) publishes `1846721f445f24a6d2df7f3dab120ef16f3fba5a` on the actual #201 merge. The resolved module contains the new API. It prepares native owner, seed, and empty layer directories under the directory lock. It creates no runtime, source refresh, selection, or permission. Migration markers, conflicting owners, missing retained seeds, and selected recovery materialization refuse preparation.

The [publication proof](../../plans/proof/starport-production-catalog/csp13/stopped-directory-preparation-2026-09-30/verification.json) records 23 results per mode, complete repository checks, generated API documentation, and one clean isolated P0 review. Root also inspected the unchanged private helpers. The proof preserves the missing-API failure and both intermediate documentation-check failures. Native qualification remains pending.

## Prepared catalog boundary correction, September 30, 2026

Starport `ac9bddda` separates final prepared inspection from original backup inspection. The [component proof](../../plans/proof/starport-production-catalog/csp13/prepared-catalog-inspection-2026-09-30/verification.json) records three final pure tests, two full race tests, and the final epoch-guard race test. It preserves the original application failure and the intermediate fixture and lint failures. Native expiry replay under the SQL guard passes six results per mode. The unchanged original fleet inspector passes 16 native pure results.

The [acceptance audit](../../plans/proof/starport-production-catalog/csp13/prepared-catalog-inspection-2026-09-30/acceptance-gap-audit.json) retains all 15 incomplete subcases. Complete CLI activation, post-backup revocation and spend, process fencing, fresh readiness, maximum capacity, and measured recovery objectives remain required. Component success does not complete CSP13.

## Coordinated application and operator delivery, September 30, 2026

Starport `2e573bdc` integrates the application coordinator and adds `backup activate` and `backup activation-status`. The [operator proof](../../plans/proof/starport-production-catalog/csp13/operator-activation-delivery-2026-09-30/verification.json) records 20 CLI results per mode and actual local Badger/SQLite activation, status, and exact retry. All 205 CLI pure results pass. Native platform qualification, post-backup activity, old-writer fencing, fresh readiness, maximum capacity, and actual RPO/RTO remain open.

Starmap [PR #203](https://github.com/agentstation/starmap/pull/203) registers actual WAL backup and migration ownership evidence. Required SQL children must each run and pass exactly once. The [publication proof](../../plans/proof/starport-production-catalog/csp13/sql-acceptance-publication-2026-09-30/verification.json) records complete local checks, 105 runner results, 65 workflow race results, and clean review. Fresh native CI remains pending.

The coverage gate previously hid failing Go test output. A reproducing regression now checks the diagnostic and exact exit status. The final complete gate passes. The original coverage failure remains unexplained and preserved. It does not establish a product defect or a passing initial run.

Two structurally valid highly compressed original capsules exceed the producer's decoded batch limit when packed by encoded size. The provisional paired repair uses producer-owned raw and decoded accounting. Published-module and maximum combined capacity qualification remain open. Component success does not complete CSP13.

## Native application deadline and prepared census, September 30, 2026

Starmap #203 merged as `f7831606686da65c13d1f639a3ac6da94e9de08d` after all 62 CI checks passed. Its reviewed and merged trees match. Branch protection remains unchanged. Full A16 still requires final consumer native and shared SQL evidence.

The [native application audit](../../plans/proof/starport-production-catalog/csp13/application-native-audit-2026-09-30/verification.json) records 32 passing bounded race results and one required full-catalog failure. The full case exceeded its existing seven-minute operation deadline. CPU evidence identifies repeated semantic publication decoding during final prepared inspection. The original test binary was unavailable, so the first sample uses a rebuilt same-source binary with a recorded address adjustment.

Starport `fc3b9074` retains a private complete byte census from the verified original backup and compiled transfer. The [component proof](../../plans/proof/starport-production-catalog/csp13/prepared-census-repair-2026-09-30/verification.json) records three results per mode and unchanged strict source validation. A regression proves the prior prepared inspector accepted an additional valid chunk outside the sealed transfer. The repair rejects that chunk and missing, substituted, or newly expiring original records. The full application measurement retains the same deadline and an actual CPU profile.

Starport `303fdef6` integrates the [observed post-backup DR test](../../plans/proof/starport-production-catalog/csp13/post-backup-activity-2026-09-30/verification.json). One result passes per mode with actual Badger, SQLite, complete backup, independent history, and activation. It preserves 700 nanoUSD spent and 200 nanoUSD held. Withdrawn keys and grants remain denied while an unaffected key works. An uncertain provider dispatch never repeats.

This observed interval does not prove continuous history completeness, shared DR, old-primary fencing, native Linux/Windows behavior, maximum capacity, or deployment RPO/RTO. CSP13 remains in progress.

## Checked-source activation repair, September 30, 2026

The [full embedded profile](../../plans/proof/starport-production-catalog/csp13/full-embedded-vector-failure-2026-09-30/verification.json) failed at Starport `0c2cf5d4` on the published Starmap `2bb99571` module. The test exceeded its unchanged 420-second activation limit. Its exact binary and CPU profile remain private with recorded hashes. The profile shows repeated original reference inspection during canonical checks. It does not establish complete wall-time attribution.

Starport `2a7cd1a1` reuses the privately checked source during one activation. It retains fresh complete artifact and encryption-key checks. The [component proof](../../plans/proof/starport-production-catalog/csp13/checked-source-integration-2026-09-30/verification.json) records 49 canonical results per mode without failures or skips. Public inspectors and restarted processes still inspect all domain references. Complete activation and maximum capacity remain UNVERIFIED.

Root integrated operator-command tests at `11afee4c` after source and evidence review. Their original source passes two native results per mode. A shipping binary proves status and completed exact retry. Actual CLI handlers resume in a fresh test process after a lost blob commit reply. Combined-source native qualification remains required.

The empty-target import and catalog adoption primitives lack a complete populated restart adoption procedure. The [extension contract](../../plans/proof/starport-production-catalog/csp13/populated-adoption-contract-2026-09-30/CONTRACT.md) assigns that procedure to CSP13. Offline expected-state reconstruction and restricted native ownership remain required. Coordinated operator activation and real restart or loss cases also remain required. Empty-target restore after a restart cannot qualify the complete procedure.


## Recovery target decision on 2026-09-30

The owner confirmed D42: measure supported reference deployments first, then agree to numeric RPO and RTO limits before production readiness. The [decision record](../../plans/proof/starport-production-catalog/csp13/recovery-target-decision-2026-09-30/decision.json) defines required measurement context and permission limits. Numeric recovery targets remain UNVERIFIED.
