# CSP21 regression fix: loader-built destination approvals (2026-10-04)

Decision 21-1 approved one reviewed Starport fix PR before any CSP21 rehearsal. The lead implemented it in `/private/tmp/starport-approvals-fix-20261004` on `codex/destination-approvals-noinit`.

## Defect

- `internal/config/config.go` declared `InferenceDestinationApprovals *credentials.DestinationApprovals` with only `json:"-"`.
- go-envconfig v1.4.3 `ProcessWith` materializes a nil pointer to a struct unless the field carries `noinit`. The loader therefore produced an empty approval set.
- `app.buildGateway` reads a non-nil set as a supplied policy and skips the installation defaults. An empty set denies every destination.
- The router bind in `internal/router/credential_policy.go:185` refused each request. The candidate answered 503 with the cause `inference credential destination is not approved`.
- PR #384 (`f5f066dd`) introduced the field. Every build after #384 carries the defect, including the `1.2.2-next` candidate from run 37222309299. The v1.2.0 release does not.
- The test fixture `newPerformanceFixtureWithRuntime` replaces the whole decoded configuration, so no test saw the materialized set.

## Fix

- The field now carries `env:",noinit" json:"-"`. Nil selects the installation defaults. An explicit empty set still denies all destinations.
- `internal/config/destination_approvals_loader_test.go` adds `TestLoaderKeepsInstallationDefaultApprovals`. It loads through `Load` and `LoadDevelopment` and requires a nil field and the installation-defaults recovery digest.
- `internal/app/destination_approvals_default_test.go` adds `TestLoaderComposedGatewayStreamsOnInstallationDefaults`. It composes the gateway through `LoadDevelopment` with an environment credential and no explicit policy. It streams one chat request through the router with the mock connector and requires 200, `text/event-stream`, and a final `data: [DONE]`.
- `CHANGELOG.md` gains an Unreleased fix line. `docs/TASKS.md` records the finding.

## Fail-before evidence

Without the tag, both loader subtests fail with "loader materialized an empty destination approval set". The composed gateway test fails with the 503 and the log cause above. The log is in the lead scratch directory at `csp21/fix-proof/fail-before.log`.

## Checks

- `gofmt -l` clean. `go vet ./internal/config/ ./internal/app/` rc 0.
- `go test -race -count=1 ./internal/config/` ok in 103 s.
- `go test -race -count=1 -timeout 40m ./internal/app/` ok in 1593 s. 246 top-level tests passed, 33 skipped for the absent fixture environment, 0 failed. The verbose log is in the lead scratch directory at `csp21/fix-proof/app-race-verbose.log`.
- `bash scripts/verify-starmap-ownership.sh` rc 0. `bash scripts/verify-v1-architecture.sh` rc 0.
- `make lint` 0 issues. `make build` rc 0.
- Technical writing: `CHANGELOG.md` PASS. `docs/TASKS.md` keeps its 6 base diagnostics.
- Autoreview `--gate pre-pr --mode auto`: clean, reviewer `codex gpt-6.1-sol high`, "patch is correct (0.96)".

## Publication

The branch rebased onto Starport main `d22f1e60` as `277179e3`. Draft PR agentstation/starport#416 opened at that head. PR #416 merged at `9b2ea10f` on 2026-10-05T00:21:27Z with 47 of 47 checks green at head `277179e3`. The merge proof in `csp21/starport416-merge-2026-10-04/` records the reviewed tree equal to the merge tree, strict protection, and zero unresolved threads.
