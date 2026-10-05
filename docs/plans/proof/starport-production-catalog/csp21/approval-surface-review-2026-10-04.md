# CSP21 Decision 21-2 lead review: operator inference origin (2026-10-04)

The implementer delivered `STARPORT_<PROVIDER>_INFERENCE_BASE_URL` in `/private/tmp/starport-destination-override-20261004` on `codex/inference-base-url-approval`. The branch holds `fd38c370` (config and approvals), `12016de9` (docs), and `2c1392bf` (approval provenance) over the fix commit `e841427d`. The lead read the full diff and reran the ordering checks.

## Delivered change

- `internal/config/provider_inference_origin.go` reads the setting through the catalog name rule. It refuses a relative URL, user information, a query, a fragment, and a template variable. It also refuses plain `http` for a public host, a provider without inference, and a parameterized provider. A refused value stops startup, names the setting, and never echoes the value.
- `internal/config/provider_environment.go` reads the setting after material resolution and before the not-configured return. The lookup-order assertion in `provider_environment_test.go` stays unchanged. `validateCredentialAliases` claims the derived name with the role `inference_base_url`.
- `ProviderConfig` gains `InferenceOrigin` with the `redact:"url"` tag. Only `resolveProviderRuntime` sets it, from the validated value. `mergeProviderConfig` clears it when the explicit configuration supplies `BaseURL`.
- `internal/providers/destination_installation.go` adds `DeploymentDestinationApprovals(bundled, settings)`. `InstallationDestinationApprovals` is the nil-settings case. The composition reads `InferenceOrigin` only, so an in-code `BaseURL` approves nothing. Only the `environment` policy compiles with the override. The other three role policies keep the public catalog origin. A private catalog origin gets only the environment override. A parameterized provider ignores a resolved base URL.
- `internal/app/app.go` compiles approvals from `b.config.Providers` when no explicit set exists.
- The lead confirmed the build step order, with `openConcepts` at line 262 and `buildGateway` at line 266.
- `OPERATOR-GUIDE.md` gains one subsection with the accepted forms, the path-append rule, and the two limits.
- `ARCHITECTURE.md` adds two sentences after the override rule.
- `CHANGELOG.md` and `docs/TASKS.md` gain one line each.

## Tests added

- `internal/config`: 11 accepted forms with the origin field set, and 14 refused forms, each with and without material. Also the parameterized and no-inference refusals, and explicit base URL precedence with an empty origin field. Also the alias claim with an embedded-catalog collision run, and the recovery digest change with a redacted value.
- `internal/providers`: digest equality without settings and with an in-code base URL. Also the environment-only bind with shared and BYOK denials, the private origin case, and the parameterized case.
- `internal/app`: `TestLoaderComposedGatewayStreamsThroughApprovedInferenceOrigin` uses the production HTTP connector behind an egress guard. It requires the upstream answer, the `OPENAI_API_KEY` bearer, and the override path at the upstream. A BYOK request binds the catalog URL and never reaches the override.

## Findings

- The brief example `http://127.0.0.1:8089/v1` was wrong. Starport appends the catalog endpoint path, which includes `/v1`, so the rehearsal sets the origin without `/v1`. The operator doc states the rule.
- A catalog refresh re-resolves providers against the in-memory environment, so the value cannot change between startup and refresh. Approvals compile once from the bundled catalog, so a refresh cannot widen an approval.
- The first app race run failed `TestApplicationRefusesUnapprovedCredentialDestination` with 200 instead of 503. The fixture sets an in-code `BaseURL` with no approval. The composition read any resolved `BaseURL` as an approved origin. The lead refused a test change and required provenance. The implementer added the `InferenceOrigin` field, and the composition reads only that field. An in-code `BaseURL` keeps the #384 refusal, and the test stays unchanged.
- An explicit destination policy in the configuration still wins over the setting. A provider that first appears in a later catalog refresh gets no approval and stays refused. The docs state both limits.
- No weakened assertion. No change to the installation digest. No AI attribution in the commit messages.

## Checks (implementer logs in the lead scratch directory `csp21/override-proof/`)

- `gofmt -l` clean. `go vet ./...` clean.
- `go test -race -count=1`: `internal/config` ok in 130 s, `internal/providers` ok in 104 s, `internal/diagnosis` ok in 69 s.
- `go test -race -count=1 -timeout 40m ./internal/app/` ok in 1643 s. 247 top-level tests passed, 33 skipped for the absent fixture environment, 0 failed.
- `TestApplicationRefusesUnapprovedCredentialDestination` passed unchanged with 503. The stopped first run stays in `app-race-verbose-run1-stopped.log`.
- Every implementer log carries a modification time after the provenance commit `2c1392bf` edits.
- `bash scripts/verify-starmap-ownership.sh` 12 passed. `bash scripts/verify-v1-architecture.sh` 11 passed.
- `make lint` 0 issues. `make build` complete.
- Technical writing: `CHANGELOG.md` PASS. `ARCHITECTURE.md` 26 before and after. `OPERATOR-GUIDE.md` 68 before and after. `TASKS.md` 6 before and after.

## Rebase and secret scan

- After PR #416 merged, the lead ran `git rebase --onto origin/main e841427d`. The branch now sits on main `9b2ea10f`.
- The override patch is the same before and after the rebase, except one blob index line in `config.go` from #416.
- On the rebased tree, `go test -race -count=1 ./internal/config/ ./internal/providers/` ok in 130.7 s and 101.2 s. `make build` ok.
- The first pre-PR gate stopped before the model call. TruffleHog reported one unverified URI hit, the test value `https://user:secret@relay.example`.
- The validator refuses on `parsed.User != nil`, so commit `3c02c544` changes the value to `https://operator@relay.example`.
- `TestInferenceBaseURLSettingRefusesUnsafeOrigins/https://operator@relay.example` passes under `-race`. The log is `override-proof/userinfo-subtest.log`.
- On the rebased tree, `go test -count=1 ./internal/app/` ok in 527 s with 0 failures. The log is `override-proof/rebased-app.log`.
- Autoreview `--gate pre-pr --mode auto` on `3c02c544`: Sol 6.1 high, 1 pass over 40,062 bytes, TruffleHog clean.
- The review reported no P0 finding, score 0.96, and stored the attestation.

## Publication

PUBLICATION_RESULT
