# Catalog distribution plan audit

Verdict: READY WITH REQUIRED CHANGES

## 1. Executive verdict

The plan is cohesive across Starmap and Starport. The accepted decisions are consistent with each other. CAT2 cannot close on its own acceptance criterion. The named verifier script does not exist yet. CAT3 through CAT8 can start after the plan changes in section 9. None of the findings requires a new owner decision.

Three highest risks:

1. The transport policy has no total bound per transfer. The target combines a 2-minute inactivity timeout, a 64 MiB body cap, and no refresh deadline. A slow-drip source can then hold a refresh worker for an unbounded time. Single-flight stalls every later cycle silently. Owner: CAT4 and CAT5.
2. CAT8 scopes the Starport timing migration as a middleware change. The 2-minute total budget on streaming lives in the execution package. The connector header timeout is 30 seconds. Both contradict the target policy, and both are outside the middleware. Owner: CAT8.
3. The fleet policy relies on jitter where jitter cannot help. Once a fleet spends the per-identity GitHub budget, no phase choice restores it. The plan says large fleets use central Starmap. It sets no request budget and no admission or capacity budget for the central tier. Owner: CAT4 and CAT7.

## 2. Repository baseline

| Repository | Expected | Actual | Difference |
| --- | --- | --- | --- |
| Starmap plan worktree `/Users/jack/src/github.com/agentstation/starmap-catalog-publisher` | `codex/catalog-publisher-six-hour` @ `96f0c3cc` | `codex/catalog-publisher-six-hour` @ `96f0c3cc`, clean | none |
| Starport worktree `/Users/jack/src/github.com/agentstation/starport` | `b522d7dc` with unrelated owner changes | `main` @ `117ad8f5`, clean | `b522d7dc` is 29 commits behind `117ad8f5`. No unrelated owner changes are present. |

Source changes between the plan baseline `42b610a` and `96f0c3cc` touch only `.github/workflows/catalog-generation.yaml` (5 added lines, 2 removed). Starport was not modified during this audit.

The Starport difference has one effect. The plan recorded the CAT8 fail-before conditions against `b522d7dc`. The audit re-verified each cited Starport condition against `117ad8f5`. All conditions still hold at the lines cited below. CAT8 must re-record its fail-before evidence against the commit it starts from.

Module facts: Starmap declares `go 1.25.0` and `toolchain go1.26.6` with no sigstore dependency (`/Users/jack/src/github.com/agentstation/starmap-catalog-publisher/go.mod`). Starport declares `go 1.26`, `toolchain go1.26.5`, and `github.com/agentstation/starmap v0.15.0` (`/Users/jack/src/github.com/agentstation/starport/go.mod`).

Constraints honored: no file edits, no commits, no branches, no pull requests, no secret values read or printed, no owner questions.

## 3. Verified agreements and disagreements

Agreements (plan matches source and primary sources):

- Layered runtime claim. `Update` produces the effective catalog under the single-slot coordinator. It swaps the result as one generation (`/Users/jack/src/github.com/agentstation/starmap-catalog-publisher/update.go:89-122`, `:167-186`). Upstream activation also goes through the coordinator (`update.go:126-165`). Nothing reads the effective catalog back as input. The claim is true for the target design and is not contradicted by the current code.
- Decision 6 and 7. `NewContext` reaches no network. It loads the embedded bootstrap and the optional workspace (`client.go:132-279`). Decision 11 holds today for Starport remote mode. Construction never reaches GitHub (`/Users/jack/src/github.com/agentstation/starport/internal/catalog/remote_runtime.go:72-137`).
- Decision 10. Starport `RemoteRuntime.Accept` verifies checksum, rejects older `GeneratedAt`, and commits the accepted head with an expected-ID compare-and-swap (`remote_runtime.go:259-322`, CAS at `:318`). CAT8 must preserve this. Findings CAT-F6 and CAT-F9 are correct.
- CAT-F13 and CAT-F16. The providers source attempts every provider with a catalog client and marks any issue as degraded (`/Users/jack/src/github.com/agentstation/starmap-catalog-publisher/internal/sources/providers/providers.go:131-136`, `:342-347`, `:387-401`). Starport passes no options to `acquisition.New`, so Starmap's default environment resolver applies (`starport/internal/catalog/runtime.go:55`, `starport/internal/app/app.go:1276-1280`, `starmap-catalog-publisher/acquisition/syncer.go:33`). Acquisition bypasses the Starport loader's `.env` credential lookup (`starport/internal/config/loader.go:170-178`).
- CAT-F11 and CAT-F15. Remote mode validation rejects workspace path, refresh on start, and refresh interval (`starport/internal/config/validation.go:104-133`).
- Publisher measurement. 20 runs, median 222.5 s, p95 284 s, max 285 s (`docs/proof/catalog-publisher/cat2-publisher-runs.json`). The 60-minute job timeout target has 12x headroom over the max.
- GitHub rate limits. GitHub allows 60 unauthenticated requests per hour per IP. It allows 5,000 authenticated requests per hour. A `GITHUB_TOKEN` gets 1,000 per hour per repository. A conditional 304 response is free only when authorized. Source: https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api.
- GitHub retry headers. `retry-after` and `x-ratelimit-reset` are hard not-before values. Source: https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-integrators.
- GitHub schedules. GitHub can delay a scheduled workflow at the start of every hour. Source: https://docs.github.com/en/actions/writing-workflows/choosing-when-your-workflow-runs/events-that-trigger-workflows. Minute 17 avoids the top-of-hour peak. `timeout-minutes` defaults to 360. Source: https://docs.github.com/en/actions/writing-workflows/workflow-syntax-for-github-actions.
- Go facts. `Transport.ResponseHeaderTimeout` excludes body read time. The HTTP/2 transport reads the same field. Sources: `go doc net/http.Transport` and `$(go env GOROOT)/src/net/http/h2_bundle.go:8568` under `GOTOOLCHAIN=go1.26.6`. `Client.Timeout` keeps running through body read. `Server.WriteTimeout` is not per-request, and `ResponseController.SetWriteDeadline` cannot extend a deadline that has already passed (`go doc net/http.ResponseController.SetWriteDeadline`). A custom `DialContext` disables HTTP/2 unless the caller sets `ForceAttemptHTTP2`. The per-chunk reset design in the plan is therefore correct only when the reset happens before the previous deadline passes.

Disagreements (plan text conflicts with itself or with the source):

- `Sync` signature. `cat2-dx.md` returns `(*sync.Result, error)`. `cat2-final-review.md` returns `(AcquisitionReport, error)`. One contract must win before CAT5.
- `Close` join bound. One plan document says the configured shutdown bound. Another says five seconds. `remote/config.go:24` currently defaults to 5 s.
- CAT2 acceptance names `scripts/verify-catalog-distribution.sh` (`docs/plans/catalog-publisher-plan.html:357`). The file does not exist in the worktree. The plan overview names authoring it as the next action (`:92`). The ledger `in_progress` state is correct. CAT2 cannot move to `audited` on this evidence.
- Branch name says six hours. Decision 3 says four hours. The plan records this. No action.
- Retry policy. The plan says backoff resets only after one healthy 60-second liveness window. The subscriber resets `attempt` to zero immediately after a successful open and catch-up (`remote/subscriber.go:439`). This is a current defect for CAT7 to fix. It is an expected fail-before condition. The plan's CAT7 acceptance does not name it.

## 4. Current-vs-target timing matrix

| Operation | Current | Target | Timeout owner | Cancellation owner | Progress signal | Failure outcome |
| --- | --- | --- | --- | --- | --- | --- |
| Manifest or channel fetch (Starmap `remote.Client`) | `http.Client.Timeout` 30 s total, body buffered to 64 MiB (`pkg/catalogs/remote/client.go:92-94`, `:364-378`) | connect 30 s, TLS 30 s, headers 60 s, inactivity 2 m, no total | Starmap transport policy (CAT4) | caller context | bytes read per idle window | typed transport error, source marked unhealthy, retry per not-before |
| Archive or payload download | same 30 s total in Starmap. Starport uses a 2-m per-request context (`starport/internal/catalog/remote_runtime.go:434`) | same as above plus size cap | Starmap (CAT4). Starport removes its own wrapper (CAT8) | caller context | bytes read | as above |
| SSE stream open | `Timeout=0`, no header bound (`pkg/catalogs/remote/stream.go:76-77`). blocks until `Close` | headers 60 s | Starmap subscriber (CAT7) | subscriber context | none until first frame | reconnect with jitter |
| SSE stream idle | liveness 60 s (`remote/config.go:22`, `subscriber.go:537-553`) | client inactivity 2 m | Starmap subscriber | subscriber context | heartbeat 20 s | reconnect with jitter, then fallback poll |
| Provider acquisition per provider | 30 s total client (`internal/transport/client.go:21-28`), Vertex 2-m context (`google/client.go:308`), 16 MiB cap (`clients/provider.go:123`) | headers 60 s, inactivity 2 m, 16 MiB cap | Starmap provider client (CAT5) | sync context | bytes read | provider outcome `failed`, retained observation kept |
| Acquisition cycle | `UpdateContextTimeout` 5 m default (`internal/constants/constants.go:23`, `pkg/sync/options.go:74`). Starport applies 2 m (`starport/internal/catalog/runtime.go:25`, `:103-115`) | no default total. `CATALOG_REFRESH_TIMEOUT=0s` | runtime (CAT5) | caller context | per-provider outcomes | partial generation with outcomes |
| Async refresh run | none. `RefreshCatalog` is synchronous (`starport/internal/app/catalog_operations.go:31-53`). coordinator single slot without join (`update_coordinator.go:15-37`) | run ID, single-flight join, cancellable | runtime (CAT5), app (CAT6), Starport (CAT8) | run owner and admin `DELETE` | run status | run record `failed` or `cancelled` |
| Starmap server catalog payload write | one `Write` under `WriteTimeout` 10 s (`internal/server/handlers/catalog.go:94`, `internal/server/config.go:43-62`) | 2-m write deadline reset before each chunk | Starmap server (CAT6) | request context | per-chunk deadline reset | connection closed, client retries with ETag |
| Starmap SSE frame write | `SetWriteDeadline` 10 s per frame (`internal/server/sse/broadcaster.go:381-393`) | 2 m per frame | Starmap server (CAT6) | request context | per-frame reset | subscriber dropped |
| Starmap admin update | synchronous with 6-m write deadline (`internal/server/handlers/admin.go:33-38`, `:53`) | asynchronous with run ID | Starmap app (CAT6) | run owner | run status | 202 plus run record |
| Starport admin refresh | synchronous, 504 or 408 on timeout (`starport/internal/app/controllers/catalog.go:69-99`) | 202 plus run ID, `GET` and `DELETE` on run | Starport (CAT8) | run owner | run status | run record |
| Starport startup refresh | `context.Background()` bounded only by refresh timeout (`starport/internal/app/app.go:358-366`) | `Open` returns embedded state promptly. connected work is background | Starport (CAT8) | app shutdown | status `bootstrap_only` then `connected` | status reports connected failure |
| Starport scheduled refresh | plain ticker, no phase, no jitter, no single-flight (`starport/internal/app/app.go:1474-1487`) | stable phase over full interval, 15-m startup spread | Starport (CAT8) | app shutdown | run status | skipped when a run is active |
| Starport non-stream inference | global chi `Timeout` 60 s (`starport/internal/server/routes.go:384`, `starport/internal/config/config.go:148`). connector header timeout 30 s (`starport/internal/providers/connectors/types.go:356-369`). execution `MaxElapsed` 2 m (`starport/internal/execution/types.go:144`) | 10-m route budget, first response 5 m | Starport route policy (CAT8) | request context | none | 504 or 408 |
| Starport streaming inference | global 60-s context plus `MaxElapsed` 2 m on the stream context (`starport/internal/execution/stream.go:27`) | no total deadline after commitment. provider and client inactivity 2 m | Starport execution and route policy (CAT8) | client disconnect | per-frame write deadline reset (`starport/internal/app/controllers/chat.go:191` clears it) | stream ends with error frame |
| Publisher job | no `timeout-minutes` (default 360) | 60 m | workflow (CAT3) | GitHub | step logs | run fails. next schedule replaces pending |
| Runtime shutdown | subscriber 5 s (`remote/config.go:24`). server grace 100 ms (`internal/server/config.go`). serve command 30 s (`internal/cmd/serve/command.go:273`). Starport `closeWithTimeout` (`starport/internal/app/app.go:1240-1245`) | `Close` idempotent, joins within one stated bound | runtime (CAT5), app (CAT6), Starport (CAT8) | shutdown context | none | forced exit after bound |

## 5. Slow-network and large-payload audit

Measured evidence (`docs/proof/catalog-publisher/cat2-network-measurements.json`):

| Item | Value |
| --- | --- |
| Archive assets | 312,818 to 393,555 bytes |
| Source JSON cap | 16,777,216 bytes |
| Remote body and compressed artifact cap | 67,108,864 bytes |
| 64 MiB at 1 Mbps | 536.9 s |
| 64 MiB at 256 Kbps | 2,097.2 s |
| 16 MiB at 1 Mbps | 134.2 s |

Findings:

- Current state fails slow networks before the plan. A 30-s `Client.Timeout` on `remote.Client` and on provider fetches aborts any transfer above about 3.7 MB at 1 Mbps. Today's archives (under 400 KB) transfer in about 3 s at 1 Mbps, so the current defect is latent. A 1 Mbps link cannot fill the 16 MiB provider cap under the 30-s timeout (134 s needed). This is an expected fail-before condition owned by CAT4 and CAT5.
- The target inactivity policy is implementable on HTTP/1.1 and HTTP/2. Go has no body inactivity timeout. The implementation must wrap the response body with a reader. The reader resets a timer on each `Read`. It cancels the request context on expiry. `Request.Context` documents that the context governs body read for outgoing requests. Cancel therefore aborts the read on both protocols (`go doc net/http.Request.Context`). Inference: the Starport `catalogRemoteTransport` cancel-on-close pattern (`remote_runtime.go:445-457`) is the right shape to move into Starmap.
- Starmap must set the header timeout on a `Transport` it owns. `remote.NewClient` copies a caller-supplied `http.Client` (`pkg/catalogs/remote/client.go:92-94`). If the caller's transport has no `ResponseHeaderTimeout`, the 60-s header bound is silently absent. CAT4 must either wrap the caller transport or reject a transport that lacks the bound.
- The target does not bound slow-drip. With inactivity 2 m, a server that sends one byte every 119 s keeps a 64 MiB transfer alive indefinitely. Single-flight then skips every later phase tick, and freshness decays with no error until the operator reads status. The policy needs a per-transfer maximum duration derived from the size cap and a floor rate. At a 256 Kbps floor, 64 MiB needs 35 min. A 60-m per-transfer bound preserves the accepted "no default refresh deadline" decision because it bounds one transfer, not the refresh.
- Memory. Every catalog fetch path buffers the full body (`client.go:364-378`, `clients/provider.go:123`). Peak is 64 MiB plus decode per in-flight transfer per replica. Attestation verification in CAT2.1 must run on the buffered bytes or on a temp file. Inference: current sizes need no streaming verification, but the plan should state the peak memory bound per replica. An operator can then review the default cap when archives grow.
- Server side. A consumer at 256 Kbps needs 35 min for a 64 MiB payload. The Starmap catalog payload handler issues one `Write` under a 10-s `WriteTimeout` (`handlers/catalog.go:94`). Today's archives complete in under 10 s above about 320 Kbps. The target per-chunk 2-m reset is correct. It must use `ResponseController.SetWriteDeadline` before each chunk, and `Server.WriteTimeout` must be zero on that server or the global timeout wins.
- Starport's fixed 2-m per-request context (`remote_runtime.go:434`) fails a 64 MiB download at 1 Mbps (9 min). Expected fail-before for CAT8, which removes the wrapper in favor of the Starmap policy.

## 6. Fleet and retry audit

Load relationship: `window_seconds >= replicas / allowed_rps`.

| Fleet | 15-m startup spread (rps) | 1-h poll interval (rps) | 4-h acquisition (rps) |
| --- | --- | --- | --- |
| 100 | 0.111 | 0.028 | 0.007 |
| 10,000 | 11.11 | 2.78 | 0.69 |
| 100,000 | 111.1 | 27.78 | 6.94 |

GitHub limits applied:

| Source identity | Allowed rate | Max direct pollers per hour | Where jitter stops helping |
| --- | --- | --- | --- |
| Unauthenticated per egress IP | 60/h, 304s count | 60 | Above 60 instances behind one NAT. Each instance needs one poll per hour, so window math cannot go below 60 per hour per IP. |
| One token (`_SOURCE_TOKEN`) | 5,000/h primary. 900 points/min and 100 concurrent secondary | about 5,000 (secondary limit allows 15 rps) | Above about 5,000 instances sharing one token, or when the startup spread pushes 10,000 instances through a 15-m window (11 rps against 15 rps secondary with no headroom). |
| `GITHUB_TOKEN` in Actions | 1,000/h per repository | n/a for consumers | n/a |

Conclusions:

- Jitter and phase are sufficient only when the per-identity quota exceeds one request per instance per interval. Above the ceilings in the table, the plan's "large fleets use central Starmap" is mandatory, not advisory. The table values are theoretical ceilings, not safe thresholds. The plan must budget from the observed rate-limit headers and the measured requests per cycle. It must state the status warning that fires near exhaustion.
- The 15-minute startup spread is an acceptable default for direct GitHub consumers within their budget. For a central Starmap tier, 100,000 instances produce 111 rps of manifest requests. After a new publication, they pull about 100,000 x 394 KB in 15 min. That is about 350 Mbps egress from the central tier. Acceptable with ETag caching and immutable payloads behind a CDN. Not acceptable from one Starmap replica without admission control.
- Inference on the spread default. The 15-m default is right for the first release. The central tier needs a documented capacity and admission budget in CAT7.
- SSE reconnect storm. After a central outage, 100,000 subscribers reconnect within the 15-m cap. That is 111 rps of new streams plus 100,000 held connections. The decorrelated jitter formula from the AWS reference (`sleep = min(cap, random(base, sleep * 3))`, https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/) spreads reconnects, but the held-connection count is the binding constraint. CAT7 admission control must bound concurrent SSE connections per replica and return `Retry-After` on refusal.
- The current subscriber uses equal jitter in `[delay/2, delay]` with a 5-s cap (`remote/subscriber.go:706-724`, `remote/config.go:18`). It resets to zero immediately on success (`:439`). Fallback polling runs at a fixed interval with no jitter or phase (`:454-475`). No client package handles `Retry-After` (grep across `pkg/catalogs/remote` and `remote` found none). A 401 or 403 stops the subscriber (`:447-452`). All five differ from the target.
- Subscriber retry consequence. CAT7 owns the expected fail-before. With a 5-s cap, 100,000 subscribers retry the central tier at 20,000 rps during an outage.
- Terminal 401/403. `Start` returns the error (`subscriber.go:245`), so Starport remote mode fails to start on a bad token. The target says wait for credential change or normal phase. CAT7 owns the subscriber change and CAT8 owns the Starport start behavior.
- Scenario coverage. The test matrix (`docs/plans/catalog-publisher-plan.html:288-300`) names 15 scenarios. They are restart, rolling deploy, cloned identity, shared storage, lease loss, partition, NAT rate limit, and provider limit. They continue with central outage, SSE storm, multi-hop, delayed schedule, manual burst, credential rotation, and provider flapping. It does not state which tests need injected clocks, random sources, or transports. Section 10 lists them.
- Scheduled publish. Cron `17 */4 * * *` with `cancel-in-progress: false` (`.github/workflows/catalog-generation.yaml:5`, `:13-15`) is correct. A hung run holds the queue for 360 min until CAT3 adds `timeout-minutes: 60`. GitHub disables schedules after 60 days without repository activity on public repositories. Inference: the publisher repository has continuous activity, so this is a NOTE.
- Shared state. The plan names one distributed lease owner. It defines no lease TTL, renewal, fencing token, or behavior after lease loss. Starport's accepted-head CAS by expected generation ID is the fencing primitive already in place (`remote_runtime.go:318`). CAT8 must commit a refresh result only through that CAS and record the lease epoch in the run. The CAS must reject a commit from a stale lease holder instead of merging it.

## 7. Starmap and Starport cohesion

- Ownership boundaries hold. Starmap owns acquisition credentials, sources, and immutable generations. Starport owns candidate and accepted heads, route acceptance, and inference timing (`/Users/jack/src/github.com/agentstation/starport/CLAUDE.md:16-36`).
- Contract mismatch. Starport pins `starmap v0.15.0`. CAT8 depends on a Starmap version that includes `Open`. The Starmap release is CAT11, after CAT8. The plan does not say how CAT8 consumes the unreleased module. No task owns that step.
- Credentials. Starmap's default resolver reads the process environment (`acquisition/syncer.go:33`). Starport's inference credentials come from the keyring (environment, shared, BYOK). Inference: Starmap never reads the keyring, so BYOK and shared credentials cannot enter acquisition today. CAT8's `WithCatalogCredentialResolver` must read only the deployment lookup (process environment and `.env`) and never the keyring. It must add a test that acquisition does not observe a configured BYOK provider credential.
- Status. Starport freshness metadata reports age, degraded flags, and source observations (`starport/internal/catalog/freshness.go:39-58`). It has no upstream-source health field. The console reads `age_seconds` and `degradation_reasons` (`starport/console/src/components/models/FreshnessBar.tsx:109-162`). CAT8 must add upstream fields without renaming the existing ones, or the console breaks.
- Route acceptance order. Starport activates a runtime through `ControlPlane.ValidateRuntime` then `ReplaceRuntime` (`starport/internal/catalog/control_plane.go:159`, `:186`). `Accept` commits the accepted head with CAS. Inference from function names and the `Accept` body: validation precedes the accepted-head advance today. CAT8 must keep that order. It must add a test that a generation rejected by route validation never becomes the accepted head.
- Environment names. The plan's `CATALOG_*` suffixes replace `REFRESH_ON_START`, `REFRESH_INTERVAL`, `REMOTE_URL`, `REMOTE_API_KEY`, and `REMOTE_ACTIVATION_INTERVAL` (`starport/internal/config/config.go:120-128`). Decision 13 forbids aliases. The plan needs a startup error that names each removed variable an operator still sets.

## 8. Findings

Severity legend: BLOCKER stops CAT2 verification or CAT3 through CAT8 start. REQUIRED must change before the owning task closes. RECOMMENDED improves safety. NOTE records evidence.

CAT-A1 BLOCKER. CAT2 acceptance names a verifier that does not exist.
Evidence: `docs/plans/catalog-publisher-plan.html:357`. `ls scripts/verify-catalog-distribution.sh` fails.
Consequence: CAT2 cannot reach `audited` or `complete`.
Correction: author the red verifier with one condition per accepted decision and one per timing row in section 4, then re-run.
Owner: CAT2.

CAT-A2 REQUIRED. `Sync` return type conflicts between plan documents.
Evidence: `docs/proof/catalog-publisher/cat2-dx.md` versus `cat2-final-review.md`.
Consequence: CAT5 and CAT8 implement different contracts.
Correction: pick one type and state it once in the plan's Go API section.
Owner: CAT2 (plan edit), CAT5 (implementation).

CAT-A3 REQUIRED. No total bound per transfer under the inactivity policy.
Evidence: section 5 slow-drip analysis. `pkg/catalogs/remote/client.go:364-378`.
Consequence: one stalled source blocks refresh indefinitely and single-flight hides it.

Correction: add a per-transfer maximum duration equal to the size cap divided by a floor rate (default 60 m). Add a stall counter to status. State the peak memory bound.
Owner: CAT4 (transport policy), CAT5 (status).

CAT-A4 REQUIRED. Header timeout depends on the caller's transport.
Evidence: `pkg/catalogs/remote/client.go:92-94`. `stream.go:76-77` has no header bound.
Consequence: SSE open and custom-client fetches can hang until `Close`.
Correction: Starmap owns the transport wrapper and applies the 60-s header bound to every catalog request including SSE open.
Owner: CAT4 (HTTP), CAT7 (SSE).

CAT-A5 REQUIRED. Starport streaming carries a 2-m total deadline outside middleware.
Evidence: `starport/internal/execution/stream.go:27`. `types.go:144`. connector header timeout `connectors/types.go:356-369`.
Consequence: CAT8 route-specific timing does not remove the stream deadline, so the target "no total deadline after commitment" fails.

Correction: extend CAT8 scope to the execution stream budget, so the elapsed budget applies only until the first byte. Extend it to the connector header timeout, so inference routes get the 5-m first response.
Owner: CAT8.

CAT-A6 REQUIRED. The plan states no fleet request budget.
Evidence: section 6 tables. GitHub rate-limit documentation.
Consequence: an operator with 200 instances behind one NAT deploys direct GitHub polling and exhausts the budget with no warning.

Correction: budget from the `x-ratelimit-limit`, `x-ratelimit-used`, `x-ratelimit-remaining`, and `x-ratelimit-reset` headers. Record the measured requests per cycle and a reserved headroom. Emit a status warning at 80 percent of the observed limit. Document the GitHub ceilings as examples and central Starmap as the fleet pattern.
Owner: CAT4 (warning), CAT9 (docs).

CAT-A7 REQUIRED. The lease contract is incomplete.
Evidence: plan decision CAT-D13. `starport/internal/catalog/remote_runtime.go:318`.
Consequence: a stale lease holder can commit after failover unless the CAS carries the lease epoch.

Correction: define the TTL, the renewal interval, and the epoch in the run record. Reject a commit from a lost lease. State that the lease applies only to shared storage.
Owner: CAT8.

CAT-A8 REQUIRED. CAT8 cannot import the Starmap `Open` API before CAT11 releases it.
Evidence: `starport/go.mod` pins `v0.15.0`. ledger order CAT8 before CAT11.
Consequence: CAT8 stalls or uses an undocumented `replace`.

Correction: split CAT11 so a Starmap pre-release tag lands before CAT8. Or state that CAT8 pins a pseudo-version from the plan branch. CAT11 then replaces the pin.
Owner: CAT8 and CAT11 ordering.

CAT-A9 REQUIRED. Subscriber retry reset and terminal auth behavior are not in CAT7 acceptance.
Evidence: `remote/subscriber.go:439`, `:447-452`, `:706-724`. `remote/config.go:18`.
Consequence: CAT7 can close with a 5-s cap and immediate reset still in place.
Correction: name the five subscriber deltas in section 6 as CAT7 acceptance conditions.
Owner: CAT7.

CAT-A10 REQUIRED. Acquisition credential resolver rule is not explicit.
Evidence: `starport/internal/catalog/runtime.go:55`. `starport/CLAUDE.md:27`.
Consequence: a future change could pass the inference keyring into acquisition.
Correction: state that the injected resolver reads only the deployment lookup and add the negative BYOK test.
Owner: CAT8.

CAT-A11 REQUIRED. Starport freshness field compatibility.
Evidence: `starport/internal/catalog/freshness.go:39-58`. `console/src/components/models/FreshnessBar.tsx:109-162`. `console/src/lib/api.ts:414-415`, `:839`.
Consequence: renaming `age_seconds` or `degradation_reasons` breaks the console.
Correction: add upstream fields alongside the existing fields and include the console in the CAT8 test set.
Owner: CAT8.

CAT-A12 REQUIRED. Six-hour objective needs a hop budget.
Evidence: section 6. cron and publisher measurements.
Consequence: a direct consumer meets the objective (about 5 h 05 min worst case without retries). Each polling hop adds up to 60 min, so two polling hops exceed 6 h.

Correction: state that the objective applies to direct consumers and SSE-push chains, and that polling chains add one poll interval per hop.
Owner: CAT9 (docs), CAT7 (chain status shows hop age).

CAT-A13 RECOMMENDED. Server write timeout must be zero where per-chunk deadlines apply.
Evidence: `internal/server/config.go:43-62`. `go doc net/http.Server.WriteTimeout`.
Correction: set `WriteTimeout` to zero for the catalog and SSE server and apply per-chunk deadlines through `ResponseController`.
Owner: CAT6.

CAT-A14 RECOMMENDED. Lower default cap for channel manifests and archives.
Evidence: archives under 400 KB. cap 64 MiB.
Correction: keep 64 MiB for expanded payloads and use 16 MiB for archives and manifests, with a single documented override.
Owner: CAT4.

CAT-A15 RECOMMENDED. Removed Starport variables need a named startup error.
Evidence: `starport/internal/config/config.go:120-128`. decision 13.
Owner: CAT8.

CAT-A16 NOTE. Starport startup refresh blocks under `context.Background()` (`app.go:358-366`), the refresh loop has no phase or single-flight (`:1474-1487`), and admin refresh is synchronous (`catalog_operations.go:31-53`). Expected fail-before, owned by CAT8.

CAT-A17 NOTE. Workflow lacks `timeout-minutes` (default 360). Expected fail-before, owned by CAT3.

CAT-A18 NOTE. `Close` join bound wording differs between plan documents. Pick one in the CAT2 plan edit.

CAT-A19 NOTE. Starmap admin update is synchronous with a 6-m write deadline (`handlers/admin.go:33-53`). Expected fail-before, owned by CAT6.

## 9. Required plan changes

1. CAT2: author `scripts/verify-catalog-distribution.sh` before any `audited` transition (CAT-A1).
2. Plan Go API section: one `Sync` signature and one `Close` bound (CAT-A2, CAT-A18).
3. Transport policy section: add the per-transfer maximum duration, the floor rate, and the peak memory bound (CAT-A3). Add the Starmap-owned transport wrapper rule, including SSE open (CAT-A4).
4. CAT8 scope: add the execution stream budget and connector header timeout (CAT-A5). Add the lease contract (CAT-A7). Add the resolver rule (CAT-A10). Add the freshness field compatibility rule (CAT-A11). Add the removed-variable startup error (CAT-A15).
5. CAT7 acceptance: name the five subscriber deltas (CAT-A9).
6. Fleet section: state the header-driven request budget and the rate-limit warning (CAT-A6). State the hop budget for the freshness objective (CAT-A12).
7. Ledger: resolve CAT8 versus CAT11 ordering (CAT-A8). Re-record CAT8 fail-before against the Starport commit CAT8 starts from.

## 10. Required tests

Tests that need an injected clock:

- Stable phase over 1-h and 4-h intervals across restart (`hash mod interval` yields the same slot). Subscriber fallback polling uses `time.Now()` directly (`remote/subscriber.go:465`, `:475`) and must take a clock.
- 15-m startup spread distribution across 10,000 simulated identities.
- Retry not-before from `Retry-After` and `x-ratelimit-reset`, with post-boundary jitter of 5 m or less.
- Backoff reset only after a 60-s healthy window.
- Freshness thresholds at the warn and critical boundaries.
- Lease TTL expiry and renewal.

Tests that need an injected random source:

- Decorrelated jitter bounds 1 s to 15 m over 10,000 samples. The subscriber uses the global `rand.Int64N` (`subscriber.go:723`) and must take a source.
- Startup spread and post-boundary jitter distributions.

Tests that need an injected transport or `httptest` server:

- Header timeout on HTTP/1.1 and HTTP/2 (`httptest.NewUnstartedServer` with `EnableHTTP2`), including SSE open that never sends headers.
- Inactivity timeout with a drip server that sends one byte per second, then stalls.
- Per-transfer maximum duration with a drip server under the inactivity window.
- Size caps at cap plus one byte for manifest, archive, expanded payload, page, and record counts.
- ETag 304 handling with and without `Authorization`.
- Forbidden public fallback: a private source configuration with a transport that fails any request to `api.github.com` or `github.com`.
- Cycle rejection at `_SOURCE_MAX_HOPS`.
- Slow client on the Starmap catalog payload and SSE routes with per-chunk deadline resets.
- Starport streaming beyond 60 s and beyond 2 m completes. A non-stream request beyond the route budget returns the documented status.

Tests that need none of the above:

- Accepted-head CAS rejects a stale expected ID and a commit from a lost lease epoch.
- Route validation failure never advances the accepted head.
- BYOK credential not observed by acquisition.
- Provider outcome table (`skipped_not_configured`, `succeeded`, `failed`) with a retained observation kept on `failed`.
- Single-flight join returns the active run ID. `DELETE` cancels. `GET` reports terminal state.
- Removed environment variable produces a named startup error.

Determinism rule: every timing test must run under `-race` with `-count=3` and no wall-clock sleeps above 100 ms.

## 11. Remaining owner decisions

none

## 12. Final gate

CAT2 verification: NOT READY until CAT-A1 and CAT-A2 land. Both are plan and script work inside CAT2.

CAT3 through CAT8 implementation: READY WITH REQUIRED CHANGES. CAT3 and CAT2.1 can start now. CAT4 and CAT5 can start after the transport policy edit (CAT-A3, CAT-A4). CAT7 can start after its acceptance edit (CAT-A9). CAT8 can start after the scope and ordering edits (CAT-A5, CAT-A7, CAT-A8, CAT-A10, CAT-A11).

The audit modified no repository and read no secret value.
