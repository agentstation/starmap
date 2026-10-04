# CSP20 Starmap slice implementer report (2026-10-04)

Verbatim report from the `csp20-starmap-implementer` agent (Opus 5.5) at `da7e88539` on branch `csp20-registry`. The lead review is in `lead-review-2026-10-04.md`.

CSP20 Starmap slice is complete: 2 commits on csp20-registry, the README sentence (a)+(b) is true as written so no revision commit was needed, technical-writing-check rc 0, 21 of 21 registry Go tests PASS, and the dry run gives 7 PASS and 3 UNVERIFIED.

## Git state (worktree /private/tmp/starmap-csp20-20261004)

`git log --oneline 4f58972d4..HEAD`:

```
da7e88539 docs: complete the README quick start catalog lifecycle
416bd29a0 catalog-product: register the CSP20 README and install checks
```

`git status --short`: empty (clean, rc 0). No push, no rebase, no PR. No AI attribution or co-author trailer. I did not touch any other worktree.

## Commit 416bd29a0: registry (scripts/catalog-product-checks.json, +243/-0)

I appended 10 entries in acceptance-map `task_checks.CSP20` order. All are `kind: all` except one. The go_test leaves are in repository starport.

- A34.temporary_persistent: cmd/starport TestReadmeFollowsTheFirstUseSequence, TestReadmeTemporaryPathPrecedesPersistentPath; internal/config TestDevelopmentRejectsPersistentEnvironmentSelections; internal/app TestDevKeepsCatalogStateInScratch, TestDevUsesInMemoryBadger
- A34.catalog_before_keys: cmd/starport TestReadmeFollowsTheFirstUseSequence, TestReadmeInspectsTheCatalogBeforeAnyCredential, TestModelsSearchAnswersJSON; internal/server TestDiscoveryRouteWithoutProviderCredentials
- A34.credential_roles: cmd/starport TestReadmeNamesTheThreeCredentialRolesInOrder, TestReadmeKeepsCredentialRolesApart
- A34.recipe_links: cmd/starport TestReadmeLinksTheStorageAndRecipePages; internal/config TestDeclaredRecipePages, TestRecipeLatencyProfiles
- A35.each_advertised_native_install: 3 x {"kind":"native_ci","repository":"starport","platform":"linux|darwin|windows","evidence":"install"}
- A35.clean_catalog_no_keys: cmd/starport TestModelsSearchAndShowWithoutCredentialsOrNetwork + the 3 install leaves
- A35.documented_inference: single go_test internal/app TestDocumentedInferenceRequestStreamsThroughGateway
- A34.persistence_paths_and_cache_roles: cmd/starport TestReadmeStatesThePathAnchors, TestReadmeLinksTheStorageAndRecipePages, TestReadmeTemporaryPathPrecedesPersistentPath; internal/cli TestConfigPathsTextIncludesRuntimeAndStorageLocations; internal/config TestLoaderOwnsIndependentProductRoots
- A34.qualified_performance_claims: cmd/starport TestReadmePerformanceClaimsAreQualified, TestReadmeLatencyClaimsReadEachUnit, TestReadmeOmitsCatalogCounts
- A50.current_artifact_evidence: the 3 install leaves

Counts: 26 go_test leaves, and each resolves to exactly one func in the named Starport package (git grep). There are 21 distinct tests and 12 native_ci leaves (4 entries x 3 platforms). I made no change in `task_component_checks`, because validate_registry permits only CSP5 and CSP7 there.

Verifier: no change. catalog_product_verify.py:295-305 passes the native_ci entry unchanged to `<starport>/scripts/native_catalog.py` verify(). That function (:180-197) sends `evidence: install` to validate_install. This confirms D7.

## Commit da7e88539: README (+8/-1)

- Repair: the dangling line 170 "Source acquisition is a separate explicit operation." is now "Acquisition uses the catalog-acquisition credentials and runs separately from catalog reads."
- Lines 172-177: one new lifecycle paragraph, 6 sentences.

Citations (Starmap file:line at da7e88539):

- L170 "uses the catalog-acquisition credentials and runs separately from catalog reads": acquisition/syncer.go:41-43 (WithCredentialResolver selects "the deployment-owned catalog-acquisition credential resolver"); client.go:110-112 (Client "owns no provider acquisition, scheduling goroutine, or cadence").
- L172 "Library reads are passive: `starmap.New` reads the verified embedded catalog or a caller-supplied store, creates no workspace, and starts no acquisition.": client.go:138-146 (New bounds reads from caller-supplied storage; "Construction never repairs or creates a workspace"); client.go:162-164 (newClient publishes the verified baseline that every client serves before its first durable generation); client.go:110-112 (no acquisition, no scheduling goroutine).
- L173 "Persistent application startup, such as `starmap serve`, exports the embedded baseline under the product paths and then opens the connected runtime.": internal/cli/commands/serve/command.go:168 (app.Runtime); internal/cli/app/catalog_runtime.go:60 (starmap.ExportEmbeddedBaseline(ctx, paths.Baselines.Path)) comes before :67 (composition.Open(ctx)). The export error returns before Open. Test: internal/cli/app/baseline_export_test.go:109 TestRequiredBaselineWriteFailurePreventsRuntimeStartup.
- L174 "The runtime serves the embedded catalog before the first upstream reply and schedules source reads and acquisition by default."
  (a) Serves before the first upstream reply: runtime/runtime.go:155-157, verbatim "Open returns a connected runtime. It serves the verified embedded catalog before the first upstream reply, so Catalog and State never wait for the network. Open starts the source and acquisition schedules and returns." Also client.go:162-164.
  (b) Scheduled by default, with no flag or configuration:
    - runtime/options.go:101-105: defaults() sets `source: DefaultSourcePolicy()` and `acquisition: DefaultAcquisitionPolicy()`.
    - runtime/policy.go:147-153: DefaultSourcePolicy = Kind SourcePublic, PollInterval DefaultSourcePollInterval, StartupPreferSource. policy.go:88: DefaultSourcePollInterval = time.Hour.
    - runtime/policy.go:268-269: DefaultAcquisitionPolicy returns {Enabled: true, Interval: DefaultAcquisitionInterval}. policy.go:252: DefaultAcquisitionInterval = 4 * time.Hour.
    - Serve path keeps the defaults:
      - serve/command.go:168-171 passes only runtime.WithListenAddress.
      - catalog_runtime.go:149-167 (baseCatalogOptions: store, state directory, client options) and :122-137 (extras: directory owner, state directory, workspace path, source URL file, WithSourceAcquirer, caller extras) set no schedule option.
      - internal/catalog/settings/composition.go:64-104 (Options) and :178-184 (Open → runtime.Open(ctx, options...)) add only Base, Config.Options(), permission clock, and authority origin options.
      - pkg/catalogs/config/settings.go:421 Config.Options() returns only the options that set settings produced. :79-80 and :284-285: STARMAP_CATALOG_ACQUISITION_ENABLED applies WithAcquisitionEnabled only when someone sets it.
    - Docs agree: docs/ARCHITECTURE.md:1264-1265 ("The Open call starts the source schedule and the acquisition schedule") and :1520-1525 (to stop every catalog request, set STARMAP_CATALOG_SOURCE=embedded and STARMAP_CATALOG_ACQUISITION_ENABLED=false).
    - Caveat checked: app.CatalogAcquisition (internal/cli/app/catalog_acquisition.go:38) passes WithSourcePollInterval(0) and WithAcquisitionEnabled(false), but only when it opens the runtime first. In serve, app.Runtime opens first at command.go:168. openRuntimeLocked then returns the cached runtime (catalog_runtime.go:40-41), so those options do not apply to serve.
  Result: the sentence stands. No revision commit.
- L175 "Acquisition gets catalog evidence from provider APIs and other sources, and `starmap update` or `acquisition.Syncer` runs it on request.": acquisition/syncer.go:76-77, 87, 117-120 (Sync observes and reconciles sources); internal/cli/commands/update/command.go:12 (Use "update [provider]"); update.go:152, 238 (sm.Sync(ctx, opts...)).
- L176 "Each publication commits one complete immutable generation and swaps it into memory as one atomic step.": update.go:126-129 ("Update serializes candidate construction, generation-store CAS, and atomic in-memory publication"); update.go:172 (Activate); acquisition/syncer.go:118-119; client.go:92-96 (without a writable store, publication stays in memory).
- L177 "The `starmap serve` server subscribes to publication and sends each generation ID and sequence to SSE clients at `/api/v1/updates/stream`.": internal/server/server.go:98 (connectHooks), :112 (sm.OnCatalogPublished), :116-118 (sseBroadcaster.Publish(sse.Publication{GenerationID, Sequence})); hooks.go:57-61; internal/server/router.go:152 (prefix+"/updates/stream" → HandleSSE); README.md:948.

## Checks

- `make technical-writing-check` (rerun now at da7e88539, default PATH): rc 0. "PASS: 1975 file(s), 0 diagnostic(s)". GLOSSARY.md: 87 terms, 0 errors, no missing candidate terms. Note: with PATH=/usr/bin first, the target exits 2 ("Python 3.11 or newer is required for TOML configuration"). The cause is the environment. The README is not the cause.
- Verifier unit tests (scripts/: python -m unittest test_catalog_product_verify test_catalog_component_checks): 151 tests OK in 11.2 s (/usr/bin/python3) and 151 OK in 10.5 s (pyenv 3.12.1).
- Registry Go tests, from the dry-run evidence (go test -race -count=1 -json). Each package run exited 0, and 21 of 21 tests PASS:
  - ./cmd/starport (12, 25.4 s): TestModelsSearchAndShowWithoutCredentialsOrNetwork, TestModelsSearchAnswersJSON, TestReadmeFollowsTheFirstUseSequence, TestReadmeInspectsTheCatalogBeforeAnyCredential, TestReadmeKeepsCredentialRolesApart, TestReadmeLatencyClaimsReadEachUnit, TestReadmeLinksTheStorageAndRecipePages, TestReadmeNamesTheThreeCredentialRolesInOrder, TestReadmeOmitsCatalogCounts, TestReadmePerformanceClaimsAreQualified, TestReadmeStatesThePathAnchors, TestReadmeTemporaryPathPrecedesPersistentPath: all PASS
  - ./internal/server (1, 19.5 s): TestDiscoveryRouteWithoutProviderCredentials PASS
  - ./internal/app (3, 38.6 s): TestDevKeepsCatalogStateInScratch, TestDevUsesInMemoryBadger, TestDocumentedInferenceRequestStreamsThroughGateway: PASS
  - ./internal/config (4, 5.0 s): TestDeclaredRecipePages, TestDevelopmentRejectsPersistentEnvironmentSelections, TestLoaderOwnsIndependentProductRoots, TestRecipeLatencyProfiles: PASS
  - ./internal/cli (1, 5.4 s): TestConfigPathsTextIncludesRuntimeAndStorageLocations PASS

## Dry run

Command: `catalog_product_verify.py --starport-root /private/tmp/starport-csp20-20261004 --task CSP20 --json`. pgrep showed no Go test process at the start. Exit 1, gate_status FAIL, "1 passed, 49 failed". The whole-gate failure is expected.

- PASS (7): A34.temporary_persistent (5/5 leaves), A34.catalog_before_keys (4), A34.credential_roles (2), A34.recipe_links (3), A35.documented_inference (1), A34.persistence_paths_and_cache_roles (5), A34.qualified_performance_claims (3)
- UNVERIFIED (3): A35.each_advertised_native_install (3 install leaves), A35.clean_catalog_no_keys (offline test PASS + 3 install leaves UNVERIFIED), A50.current_artifact_evidence (3)
- Reason for each install leaf: "[Errno 2] No such file or directory: '/private/tmp/starport-csp20-20261004/docs/proof/catalog-native/capture.json'". That worktree has no capture.

Evidence: scratchpad/csp20-starmap-dryrun/ (report.json, stderr.log, exit.txt, go-evidence/ with 5 files).

## Limits and uncertainty

- I did not run the dry run against the merged Starport main that holds the format 4 capture of run 37222309299. The 3 install subcases stay UNVERIFIED until that run completes.
- No gate binds the Starmap README by digest. CAT-V59 greps only for the runbook link, which did not change.
- I did not run `make verify`, docker, or any provider network call. No credentials were used.

## Follow-up: limits the lead had not recorded

1. PATH order breaks the writing check. With PATH=/usr/bin first, make technical-writing-check exits 2: "Python 3.11 or newer is required for TOML configuration". It passes with the default PATH (pyenv python).
2. Install evidence: the 3 install subcases stay UNVERIFIED unless --starport-root points at the checkout with the format 4 capture (run 37222309299). clean_catalog_no_keys also depends on those install leaves.
3. Schedule caveat: app.CatalogAcquisition (internal/cli/app/catalog_acquisition.go:38) opens the runtime with WithSourcePollInterval(0) and WithAcquisitionEnabled(false). This applies only if it opens the runtime first. serve opens app.Runtime first, and openRuntimeLocked returns the cached runtime (catalog_runtime.go:40-41), so serve keeps the scheduled defaults. A future command that calls CatalogAcquisition before Runtime would get no schedules. No test binds this order.
4. No gate binds the Starmap README lifecycle paragraph. No registry leaf or digest covers README.md:166-177, and CAT-V59 checks only the runbook link.
5. Noise: pyenv 3.12.1 prints blake2b/blake2s hashlib errors on each start. These errors cause no failure.
