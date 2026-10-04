I stopped before the tooling. No route sends provider `openai` to a local fixture without a product change, which is the brief's D2 stop condition. While testing that, I also found a regression: the candidate refuses the normal `OPENAI_API_KEY` route in `starport dev` before it dials, and v1.2.0 does not. That blocks the README first request, so it blocks the CSP24 real capture too.

Only deliverable 1 is committed. Deliverables 2–4 are not done: no tooling, no manifest, no rehearsal record, no CI or Makefile wiring. No product code changed. Nothing is pushed.

## Commits

`git log --oneline origin/main..HEAD`:
```
550352d5 proof: record the format 4 native catalog capture of run 37222309299
```
- This commit holds only `docs/proof/catalog-native/`: 67 files, 398,261 insertions.
- I copied it with `cp -R` from `/Users/jack/src/github.com/agentstation/starport/docs/proof/catalog-native` and left the original in place.
- `diff -r` shows the copy is identical. 67 files were staged, so `.gitignore` dropped none.
- `capture.json` has `format: 4`, run `37222309299`, and head `aeae766491c248c8585d4f97a006ae3a7e7e40fb`.

One file is untracked and uncommitted: `/private/tmp/starport-csp21-20261004/scripts/readme-demo/fixture_upstream.py`. It is the D2 fixture server, written to the brief. It reads its token from `STARPORT_DEMO_FIXTURE_TOKEN` and never writes that value to output or to its request log.

## Base-URL mechanism: none exists for `openai`

Code evidence:
- **No config key.** `ProviderConfig.BaseURL` (`internal/config/config.go:222`) is filled only by `projectResolvedProvider` (`internal/config/provider_environment.go:385-400`). That function runs only when the catalog `Inference.BaseURL` is a `{VARIABLE}`. No code fills `Config.Providers` from a file. The only writer is `EnableProvider` (`config.go:275`). The config file is dotenv (`loader.go:325`), so it has no `providers.openai.base_url` key.
- **No endpoint binding.** The pinned Starmap `v0.16.6-0.20261003000954-595e3c7ba959` gives `openai` a fixed `base_url: https://api.openai.com` and no `endpoint_bindings`. Only `alibaba`, `azure-openai`, `google-vertex`, and `ollama` have a bindable base URL.
- **Credential destination approval also blocks it.** `InferenceDestinationApprovals` is never assigned outside tests. The app uses `InstallationDestinationApprovals(bundled)` (`internal/app/app.go:722-731`). That function approves only the bundled catalog's public HTTPS origin and skips private and parameterized origins (`internal/providers/destination_installation.go:23`, `:46-60`).
- **A proxy does not help.** The client clones `http.DefaultTransport` (`internal/providers/connectors/http_client.go:20`), so it honors `HTTPS_PROXY`. But a proxy only tunnels TLS to `api.openai.com`. Replacing that answer would need a trusted interception certificate in the macOS keychain, which I did not do.

Experiment against the candidate binary (`route_probe.py` in the scratchpad `csp21/` directory):
- **Setup:** throwaway token, fixture on a free port, temporary HOME, and `starport dev --no-open` on a free port.
- **No egress:** a local proxy recorded and refused every outbound connection. A second run used a `sandbox-exec` profile that denies non-localhost outbound connections.
- **Variants:**
  - no override
  - candidate keys in the scratch `config.env`
  - the same keys in the process environment
- **Keys tried:** `STARPORT_PROVIDERS_OPENAI_BASE_URL`, `STARPORT_OPENAI_BASE_URL`, `OPENAI_BASE_URL`, `STARPORT_PROVIDERS_OPENAI_BASEURL`.

Results:
- **Fixture:** it received 0 requests in every variant.
- **`config.env` variant:** `starport config validate` exits 1 with "configuration sources could not be read".
- **Proxy log:** it recorded only status-page CONNECTs and no CONNECT to `api.openai.com`, so the throwaway token never left the machine.

## Candidate regression in `starport dev`

With no override at all, the candidate returns HTTP 503 "Provider credential material is no longer valid." The server log cause is `inference credential destination is not approved`, which is `ErrDestinationUnapproved` (`internal/credentials/destination.go:18`). It fails before any dial, with or without the proxy.

| binary | sandbox, no proxy, same probe | result |
|---|---|---|
| v1.2.0, binary `f6a0a9f4…` | 502 "The provider did not respond." | approval passes; the dial to `api.openai.com` is blocked by the sandbox |
| 1.2.2-next, run 37222309299, binary `dc8c9cc1…` | 503 | `inference credential destination is not approved`, before any dial |

So the candidate (aeae7664, whose product tree matches main `e17ea25a`) likely cannot serve the README's real-key first request. I did not confirm that with a real key. I did not find the root cause.

Commits since v1.2.0 that touch this area: `cb6f03a2` (#373), `f5f066dd` (#384), `a1a08df8` (#387), `03b09b21` (#391), `046d86bc` (#392), `c2013cd9` (#394). PR #415 (still OPEN when I checked) does not touch it.

## Candidate identity

- Run `37222309299`, PR 414, head `aeae766491c248c8585d4f97a006ae3a7e7e40fb`.
- `starport --version` reports `starport version 1.2.2-next`.
- Archive `starport_1.2.2-next_darwin_arm64.tar.gz`, SHA-256 `926e51d45c9f60369811645cd845c20bcb6b87d025b2768a6d75c531d184f67f`. This matches `checksums.txt`.
- Binary SHA-256 `dc8c9cc10d785634f91959f39c9aa5f4ca127f37affd49747d5695b60ced3362`.

## Commands and results

- `gh pr view 415 … --json state,mergeCommit`: exit 0, OPEN, no merge commit.
- `gh run download 37222309299 … --name starport-release-snapshot`: exit 0.
- `gh release download v1.2.0 … darwin_arm64 + checksums.txt`: exit 0. Its checksum matches. I used it only as the baseline.
- `cp -R`, `diff -r` (identical), `git add`, `git commit`: all exit 0. 67 files committed.
- `route_probe.py` with variants none, `config_env`, and `process_env`: exit 0, fixture 0 requests, 503 in each.
- The same probe in the no-egress sandbox, candidate: 503, destination not approved.
- The same probe in the no-egress sandbox, v1.2.0: 502, dial blocked by the sandbox.
- `sandbox-exec … curl https://api.openai.com`: exit 7, which confirms the sandbox blocks egress.

These were not run because deliverables 2–4 stopped:
- the readme-demo verifier, its mutation proofs, and its unit tests
- `verify-doc-links.sh`, `verify-readme-quickstart.sh`, `verify-starmap-ownership.sh`, `verify-v1-architecture.sh`, and `actionlint`
- the technical-writing lint, since I added no Markdown
- autoreview

## Cleanup

I deleted every downloaded archive. Two extracted binaries stay in the scratchpad `csp21/` directory (215 MB) for a rerun after your decision. No Docker container was touched.

## Decision for you

I recommend fixing the destination-approval regression first, in a separate product PR. CSP24 needs that fix anyway.

The fixture then still needs one of these:
- **A product change:** a development-only, explicitly approved base-URL override for a provider.
- **A design change:** use the `ollama` provider, whose base URL is the `{base_url}` endpoint binding. That changes the model and makes `provider_path: ollama` instead of the schema's `openai`. It also needs a workspace model entry. Installation approvals skip private origins, so I have not verified that this route works.

Two manifest points also need your call:
- The binary embeds `docs/site/`, and the archive ships `README.md`, `SECURITY.md`, `LICENSE`, and `.env.example`. Those are release-snapshot inputs, so the `product_paths` list must include them.
- I would leave `.github/workflows/ci.yml` out of `product_paths`. Otherwise PR A's own CI wiring would invalidate its rehearsal.
## Addendum (2026-10-04, after the lead's four questions)

The text above is the original report, verbatim. This addendum adds new runs. No product code in the worktree changed.

### New run: egress allowed

- `route_probe_egress.py` (scratchpad `csp21/`) is the same probe, but its proxy tunnels each CONNECT unchanged and records the target.
- Each run generates a new throwaway token with `secrets.token_urlsafe(32)`. The two binaries got the same kind of token, not the same value.

| binary | egress | result | CONNECT to `api.openai.com` |
|---|---|---|---|
| candidate `dc8c9cc1…` | allowed | 503, cause `inference credential destination is not approved` | none |
| v1.2.0 `f6a0a9f4…` | allowed | 401 "Provider authentication failed." OpenAI rejected the throwaway token. | one |

- Both binaries made CONNECTs only to provider status pages before the request.
- The v1.2.0 run sent a throwaway token to OpenAI. The OpenAI error put a masked fragment of it in the server log. The probe deleted that run directory.

### Refusal site from a scratch instrumented build

- I copied `cmd`, `internal`, `skills`, `config`, `go.mod`, and `go.sum` to the scratchpad with `git archive`. I wrapped each `ErrDestinationUnapproved` return with a site marker and built that copy. I ran it under the no-egress sandbox.
- The cause chain was `[site credential_policy.go:185] [site destination_approvals.go:58 provider=openai role=environment profile=api-key no grant or policy for provider/role/profile]`.
- The refusal path has three steps:
  1. `internal/router/credential_policy.go:185` calls `DestinationApprovals.Bind`.
  2. `Bind` finds no grant and no policy for that key at `internal/credentials/destination_approvals.go:58`.
  3. `connectors.NormalizeFailure` maps the error to the 503 text at `internal/providers/connectors/failure_adapter.go:51-52`.
- `TestInstallationApprovalsBindCredentialRolesToBundledOrigin` (`internal/providers/destination_installation_test.go:14`) proves that the installation approvals hold that policy. The approval set in `starport dev` therefore does not come from `InstallationDestinationApprovals`, or it lacks the `openai` policy.
- I also added stderr lines to `InstallationDestinationApprovals` in the copy. They did not appear in the dev log. I could not confirm that the log filter captured them.
- The permission system then denied a further tracing command. I stopped there and deleted the instrumented copy and binary.

### Other facts

- `internal/app/performance_test.go:165-166` sets `cfg.Providers[openai].BaseURL` to a test server. Lines 177-187 set explicit `InferenceDestinationApprovals`. So the runtime honors `ProviderConfig.BaseURL`, but no configuration source sets it. That test path also does not use the default approvals, so no test covers the default `starport dev` path.
- I found no credential verification request before the first inference. `OperatorMaterialReady` (`internal/providers/state/store.go:715`) reads an in-memory projection and makes no request.
