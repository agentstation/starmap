# CSP21 lead review and regression finding (2026-10-04)

The Starport implementer stopped at design decision D2. Its report is in `starport-implementer-report-stop-1-2026-10-04.md`. The lead traced the stop condition to a product regression on Starport main `e17ea25a`. The probe evidence is in `lead-probe-2026-10-04/`.

## Regression: every loader-built configuration denies all inference destinations

The candidate binary from run 37222309299 answers each chat completion request with HTTP 503. The body says `Provider credential material is no longer valid.` The server log records the cause `inference credential destination is not approved`. The v1.2.0 binary on the same route answers 200. The implementer observed this first, and the lead reproduced it in process.

Root cause:

- `internal/config/config.go:32` declares `InferenceDestinationApprovals *credentials.DestinationApprovals` with only a `json:"-"` tag.
- Its comment states the contract: nil selects the pinned installation defaults, and an explicit empty set denies all destinations.
- `internal/config/loader.go:378` decodes the configuration with `envconfig.ProcessWith` from `go-envconfig v1.4.3`.
- That decoder materializes each nil pointer to a struct and keeps the empty value unless the field carries `noinit`. See `envconfig.go:452-506`.
- The loader therefore returns a non-nil, empty approvals set with zero policies and zero grants.
- `internal/app/app.go:722` reads non-nil as a supplied deployment policy and skips `InstallationDestinationApprovals(Bundled())`.
- `internal/credentials/destination_approvals.go:49` then refuses every `Bind` with `ErrDestinationUnapproved`.
- `internal/router/credential_policy.go` maps that refusal to the 503 response.

The field entered main in `f5f066dd` (#384). No non-test code writes it. `Load` and `LoadDevelopment` share the same decode, so the defect covers `starport dev` and the production path. Every inference request with an environment credential fails on each build after #384, which includes `1.2.2-next`.

The recovery digest in `internal/config/recovery_selection.go:129` reads the same field. `RecoverySelectionSHA256` returns the `installation-defaults` digest for nil and an empty digest list for the empty set. A loader-built configuration therefore records a different recovery selection digest than the literal configuration in the tests. The fix must keep both digests equal for a deployment without an explicit policy.

## Evidence

| Observation | Source |
| --- | --- |
| Candidate binary `dc8c9cc1…` answers 503 under the no-egress sandbox | `lead-probe-2026-10-04/result.json`, `server.redacted.log` |
| `LoadDevelopment` returns `dev_approvals_nil=false` | `lead-probe-2026-10-04/dev-probe-trace.log` |
| `validProductionConfig` literal returns `literal_approvals_nil=true` | same log |
| `buildGateway` keeps the loader set with `policies=0 grants=0` | same log, line `ZZPROBE approvals.Bind refused` |
| A direct `Bind` against `InstallationDestinationApprovals(Bundled())` passes before and after `Run` | same log, lines `bind err=<nil>` |
| The composed router returns `[request] status=503` | same log |
| Probe source | `lead-probe-2026-10-04/zz_probe_dev_test.go.txt` |

The probe used a throwaway token with no provider value. The lead reverted every trace edit and deleted the probe files from the worktree before this record.

## Why the tests missed it

- `validProductionConfig` in `internal/app/app_test.go:362` is a struct literal. Its nil field selects the installation defaults.
- The loader-based fixtures that stream a request set an explicit approval policy with `approve=true`.
- The `approve=false` fixtures expect a 503, which masks the same 503 from the regression.
- No test composes the application from `NewLoader().Load` or `LoadDevelopment` and streams one request on the default policy.
- The CSP20 test `TestDocumentedInferenceRequestStreamsThroughGateway` streams through the literal configuration, not the loader.
- The CSP20 `candidate-install` job runs `starport --version` and the catalog commands. It sends no inference request.

## Checks ruled out

- A profile mismatch after `Run`: the runtime, bundled, and current catalog profiles for `openai` are identical.
- A second router construction site: `router.New` has one caller at `internal/app/app.go:751`.
- A request-time `AuthorizeDestination` refusal: the trace never reaches it.
- A catalog generation difference: both paths report the bundled generation.

## Pending decisions (recorded for the owner)

### Decision 21-1: regression fix delivery

The lead proposes one reviewed Starport PR from `origin/main` with:

- an `env:",noinit"` tag on `InferenceDestinationApprovals`, which keeps nil for an unset field and preserves the explicit deny-all contract
- a regression test that composes through `NewLoader().WithEnvironment(...)` on both `Load` and `LoadDevelopment`, streams one request to a fake upstream, and requires 200 and `[DONE]`
- a test that the loader-built recovery selection digest equals the literal digest
- a `docs/TASKS.md` entry and a CHANGELOG line that name the defect and the affected builds

The fix touches a deployment policy boundary, so the autoreview pause rule applies. The PR goes before the CSP21 rehearsal, because rule D8 invalidates any rehearsal candidate that precedes a product change. The candidate from run 37222309299 is therefore not usable for the rehearsal record.

### Decision 21-2: CSP21 fixture upstream route

No configuration key routes the `openai` provider to a local fixture without a product change. The implementer verified this on the candidate binary and v1.2.0 and stopped per D2. Options:

1. Add an explicit `providers.<id>.base_url` override that the configuration authority approves as a deployment policy, with tests and a documented operator contract.
2. Use a provider with a private-origin endpoint binding such as `ollama`, and record the demonstration with that provider path instead of `openai`.
3. Record the rehearsal with the real `openai` upstream. This needs a provider credential and a paid request, which the current authorization excludes.
4. Rehearse without the `answer` scene and mark R01 and R02 UNVERIFIED until CSP24.

The lead recommends option 1 inside the regression fix PR or as its direct follow-up. Option 2 changes the demonstrated provider path and weakens the README binding. Option 4 leaves the CSP21 acceptance open.

## Starmap slice state

The Starmap worktree `/private/tmp/starmap-csp21-20261004` holds two commits on `csp21-registry` over `da7e88539`. It needs a rebase onto `b6c30e0a7`. R01 and R02 can pass only after the Starport slice lands a record, so the Starmap PR waits for Decision 21-2.

## Owner decisions (2026-10-04)

- Decision 21-1: the owner approved one reviewed fix PR from main before any rehearsal. The lead implements it in `/private/tmp/starport-approvals-fix-20261004` on `codex/destination-approvals-noinit`.
- Decision 21-2: the owner selected the approval surface. A second Starport PR adds the operator-facing destination approval configuration with tests and an operator doc line.
- The owner asked the lead to explain D2 in plain terms before the second decision. The lead explained it in chat and keeps this record as the durable copy.

## Decision 21-2 design (2026-10-04)

The lead read the configuration, policy compiler, installation policy, registry binding, and recovery digest code. The design brief is in the lead scratch directory at `csp21/approval-surface-brief.md`. Its content:

- One setting per provider, `STARPORT_<PROVIDER>_INFERENCE_BASE_URL`, read beside the catalog-derived credential references. The value is the approval. No second flag exists.
- The loader validates the URL and fails startup with the provider name on a refused form. The loader accepts plain `http` only for loopback, link-local, private, or `localhost` hosts.
- Providers with catalog endpoint bindings keep their binding variables. The setting refuses them, and the operator doc states the limit.
- `providers.DeploymentDestinationApprovals(bundled, settings)` compiles the override for the `environment` and `shared` roles. Account BYOK and anonymous material keep the catalog origin. The registry applies operator overrides only to operator material, so the roles match.
- The gateway calls that function when no explicit approval set exists. The recovery digest changes through the provider settings.
- Required tests cover the loader, the policy composition, and the digest. One loader-composed test streams through a real HTTP connector to an `httptest` upstream.

The implementer `csp21-override-implementer` works in `/private/tmp/starport-destination-override-20261004` on `codex/inference-base-url-approval` from the fix commit `e841427d`.

### Scope refinement (2026-10-04)

The implementer stopped before any code change and reported a conflict. The brief compiled the `shared` policy with the replacement origin. The router passes the operator override only for environment material (`internal/router/execution_adapter.go:320`, `internal/registry/generation.go:443`). A shared credential still calls the catalog URL. Under the brief, every shared credential on an overridden provider fails the destination check.

The lead verified both lines and chose the environment-only scope. Only the `environment` policy compiles with the override. The `shared`, `byok`, and `anonymous` policies keep the public catalog origin. The router stays unchanged. The docs state the environment-only scope as a limit beside the parameterized-provider limit. The implementer also claims the derived name in `validateCredentialAliases` and reads it after material resolution, so the existing lookup-order assertion stays.

## Fix publication (2026-10-04)

The fix branch rebased onto `d22f1e60` as `277179e3`. Autoreview was clean with reviewer `codex gpt-6.1-sol high` and "patch is correct (0.96)". Draft PR agentstation/starport#416 is open, and a background poll watches its CI. The record `csp21/regression-fix-2026-10-04.md` holds the defect, the fix, and the checks.
