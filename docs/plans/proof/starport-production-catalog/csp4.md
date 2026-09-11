# CSP4 authority and retained startup

## Current runtime integration

The plan [delivery checklist](../../starport-production-catalog-plan.html#csp4-deliveries) separates merged work from the remaining CSP4 deliveries.
Starmap PRs #146, #148, #149, and #150 merged. Twenty-eight campaign PRs merged across nine completed tasks.
CSP4 remains active because published dependency qualification, complete delivery verification, and merges remain open.

### Portable query correction

The [correction proof](csp4/windows-portable-entry-2026-09-11/verification.json) preserves the full verifier failure at `d1bb47e1`.
Its complete race stage passes, but production lint reports seventeen unused portable RPC declarations.
Native workflow `34609335965` fails on the same declarations after all six runtime jobs pass.
PR #151 remains unmerged. The agent disabled its automatic merge before the correction.

Commit `2b4aa1fe` keeps the existing QueryStatus entry in portable code and selects Windows authentication through a platform factory.
Unsupported hosts close the supplied stream before sending RPC bytes. Portable wire tests remain enabled.
Both supported toolchains pass 68 race events. Three platform lint checks, four Windows cross-builds, and static checks pass.

Both required reviewers report zero findings. All six native jobs pass in workflow `34620963592`.
They record 11,704 passing test events and no failures. Four explicit skips remain in the ordinary suites.

Separate privileged checks pass the two Linux configuration cases. The two Windows skips are optional diagnostics. Both mandatory clock preflights pass 23 events.

The CI merge checkout has the same tree as reviewed `2b4aa1fe`.
The agent enabled automatic merge after every check except the repository Verification Gate passed. That gate still blocks the merge.

Combined source `6419ea85` includes the correction. Full repository verification runs in session `55757`.
Its log is `.tmp/csp4-origin-integrated/portable-entry-verifier.log`. The combined runtime and Starport still need delivery checks, native CI, and merges.

### Consumer checks and published dependency

Starport `80f8064e` adds four authority acceptance cases. Its [proof](csp4/consumer-acceptance-2026-09-11/verification.json) records 45 passing race events across five repetitions.
The cases cover excluded embedded membership, cold refusal, retained Badger startup, and unsupported-schema permission handling.
An unchanged permission allows the retained catalog. A new permission requirement blocks it before replacement activation.
A different authority or policy also refuses retained inference.

The tests use real Badger candidate, accepted, and lease stores with a deterministic source and qualified clock.
They do not qualify native clocks or live provider routing.
The first schema fixture confused a new publication with a permission withdrawal. The corrected table covers both cases and preserves that initial failure.

Commit `1da7a26` makes the architecture version expression match the existing release verifier.
All twelve architecture checks pass, including the full Go suite. Module replacement rejection remains unchanged.
Starport resolves published Starmap `v0.16.6-0.20260911063726-852a548c4959` with `GOWORK=off`.
The final stable released-pair qualification remains required.

### Task verifier

The [registry proof](csp4/consumer-registry-2026-09-11/verification.json) records eight passing CSP4 subcases with no skipped named tests.
Four consumer mappings use ten named checks across catalog, application, router, cache, and HTTP boundaries.
The initial run skipped the absent signed fixture. Preparation verified its published size and checksum.
The prepared run passed four public subcases and reported four missing consumer mappings. The registered run passes all eight.

This task gate records component checks. Full A09, A10, and A21 qualification remains with CSP10.
The later clock candidate connects production composition. Its published dependency, remaining delivery checks, required review, publication, and merge remain open.

The [integrated task proof](csp4/integrated-task-2026-09-11/verification.json) repeats the gate against Starmap `d1bb47e1` and Starport `9669b61d`.
All eight selected subcases pass with 54 race events and no skips. These checks use the temporary module workspace.
The report leaves 49 primary cases unverified because this invocation selects only CSP4. It completes A07 within that selection.

The full task race command passes 986 events at `d1bb47e1` with `GOWORK=off` and `Go 1.26.6`.
Runtime contributes 901 events, remote 45, and artifact 40. No event fails or skips.
Both explicit CSP4 commands pass at that source. Its repository verification later failed production lint.
Required review and implementation merges remain open.

### Remaining native and origin delivery

All six native runtime jobs pass at `75ac9d07` in PR #151.
The [production proof](csp4/windows-rpc-2026-09-11/verification.json) retains all six artifacts and exact test counts.
Both Windows suites pass 1,925 events, and both Windows access preflights pass 23 events.
Each platform also passes two Git acquisition events and four publication events.
The proof identifies the separate Linux administrator check and opt-in Windows diagnostic skips.
The repository Verification Gate failed production lint in workflow [34609335965](https://github.com/agentstation/starmap/actions/runs/34609335965).

Origin `5066856d` scopes alias history to the selected authority and policy.
The [transition proof](csp4/authority-transition-2026-09-11/verification.json) records 109 passing race events per supported toolchain, static checks, and 1,514-file prose verification.
The cases cover authority and policy changes with retained or fresh runtime directories, failed publication, recovery, replacement activation, and retained restart.
Prior approval cannot authorize the changed context. Same-authority updates retain the alias-history checks.

The initial transition test reproduces an alias-history conflict across authorities.
Fresh runtime state also exposed startup alias validation against the embedded baseline. The correction retains diagnostics without granting permission.
The first implementation required restart recovery after its deliberately failed store write. The final test follows that existing journal contract.

Combined source `d1bb47e1` merges reviewed native parent `75ac9d07` without conflicts.
It contains clock lifecycle, origin settings, follower restart, and subscriber transitions in one delivery.
Full `make verify` failed production lint in session `8874` against this source.
The current log is `.tmp/csp4-origin-integrated/verifier.log` in the origin-settings worktree.
Task checks, required review, publication, native CI, and merge remain required.

CSP11 owns full shared input recovery, equivalent acquisition capability, and atomic lease/head fencing.
Starport still needs a compatible published dependency and its final delivery checks.
The [earlier checkpoint](csp4/integration-history-before-transition-2026-09-11.md) preserves the previous current record.

### Real Valkey integration follow-up

The [Valkey proof](csp4/valkey-2026-09-11/verification.json) records 35 passing race events at Starport `9669b61d`, with no skips.
It covers real storage, pub/sub, the KVStore contract, and application startup with the existing test connector factories.
The container used the repository-configured Valkey 7 image at a recorded digest. It reported version `7.2.14`.

The service bound an ephemeral loopback port and mounted no host data volume. The test runner removed it after completion.
The earlier package run keeps its historical skip. This follow-up supplies the previously absent real-service evidence.
Fleet recovery, PostgreSQL, native clock bounds, and paid inference remain outside this check.

### Starport production clock composition

Local `9669b61d` connects Starport to the canonical Starmap clock profile.
The [clock proof](csp4/starport-clock-2026-09-11/verification.json) records 429 package race events, including all 26 new clock events.
One optional Valkey integration case skips because its test URL is absent.
Six Windows cross-builds, vet, lint, ownership checks, dependency checks, and documentation links pass.

The loader reads both product prefixes through the canonical clock parser.
Process environment values precede files. Within each source, the Starport name precedes its Starmap alias, including explicit empty values.
The runtime owns native monitor startup and shutdown. Invalid clock profiles fail before catalog storage construction.

A canceled construction context does not close the runtime. Its owner must call Close.

The tests use an untracked workspace with Starmap `d1bb47e1`.
The committed published dependency predates the required clock API. Publish the combined runtime and update that pin before final qualification.
Native lifecycle tests prove observation attempts and shutdown, not the declared host error bounds.

The new prose passes with zero diagnostics. The full operator guide retains its 48 unrelated baseline diagnostics.
Required review, delivery gates, native Starport CI, and merge remain open.
The [previous current record](csp4/integration-history-before-starport-clock-2026-09-11.md) preserves earlier state.

## Native clock checkpoint history

Native clock `5418420c` is now [PR #151](https://github.com/agentstation/starmap/pull/151).
Its [proof](csp4/windows-observer-2026-09-11/verification.json) records all 41 local verifier stages passing after the dependency inventory correction.
Sol at xhigh and Opus at high report zero findings. The secret scan passes.

Native workflow `34580591626` passes all four Linux/macOS jobs. Both Windows jobs fail the same two tests. Starport has no open PRs.
The failures name the direct `kernel32.dll` lookup and absent `W32TIME` pipe. A local correction uses the API set and `W32TIME_ALT` with caller identification.

Correction `0dfa9a90` passes 187 permission race events per toolchain, eight Windows cross-builds, static checks, and complete prose.
Its [binding proof](csp4/windows-binding-2026-09-11/verification.json) records zero findings from Sol and Opus and a passing secret scan.
PR #151 contains the correction.

Native workflow `34583505814` passes both Windows counter tests and all Linux/macOS jobs.
Both Windows status calls now return RPC access denied. Authentication and caller-privilege diagnosis remains open.

Local `437dc13c` adds Windows SSPI authentication with packet privacy and native privacy tests.
The [authentication proof](csp4/windows-authentication-2026-09-11/verification.json) records 194 portable race events per toolchain and eight Windows cross-builds.
Static checks, documentation checks, and the 1,495-file prose check pass. Sol and Opus report zero findings, and the secret scan passes.

PR #151 contains `437dc13c`. Workflow `34588801426` passes native SSPI tests on both Windows architectures.
Both W32Time reads fail at the combined principal check. The separate `w32tm` queries succeed.

Diagnostic `e4a80fa6` separates RPC status, missing reply, empty name, and excessive name failures. It preserves all rejection conditions.
Its [proof](csp4/windows-principal-2026-09-11/verification.json) records 194 passing race events per toolchain and four Windows cross-builds.
Static checks and the 1,497-file prose check pass.

Sol reports zero findings.
Opus flags the known native failure as a merge blocker. The proof records its assessment.
The required fixture remains enabled. PR #151 contains the diagnostic candidate, and native workflow `34591105915` runs before any merge.

## Earlier runtime integration checkpoints

The recorded combined worktree is `/Users/jack/src/github.com/agentstation/starmap-catalog-authority-snapshot`, branch `codex/catalog-authority-snapshot`, at clean `e3943c97`.
It adds atomic catalog authority snapshots and includes reviewed consumer and public-catalog changes.
The [snapshot proof](csp4/authority-snapshot-2026-09-11/verification.json) records six scoped passing cases per toolchain, complete static checks, and combined verification.

All 41 verifier stages pass across the initial command and the corrected remainder. Sol and Opus report zero findings at `e3943c97`.
PR #149 merged as `9ea36b3e` after fifteen passing checks. The combined tree equals tested `768347ab`.

[PR #150](https://github.com/agentstation/starmap/pull/150) merged as `852a548c` with fifteen successful checks and exact reviewed-tree equality.
Both repositories have no open PRs. Twenty-eight campaign PRs merged.
Native clocks and the four Starport consumer cases remain UNVERIFIED.

Host clock `0868781d` adds explicit Linux and macOS observations.
Its [native proof](csp4/host-clock-2026-09-11/verification.json) records 25 Linux and 15 macOS test events.
Both supported toolchains pass 142 permission events. The elapsed reads allocate zero times.

The macOS host supplies a bounded sample. The Linux container supplies unqualified time, which the adapter refuses.
Windows support, operational bounds, and host scheduling remain incomplete.

Host clock `41071ef4` adds the Windows elapsed counter.
Its [counter and prototype proof](csp4/windows-counter-2026-09-11/verification.json) records four Windows cross-builds and 142 Darwin permission race events.
Windows native interval and allocation tests have not executed. Windows UTC observations still refuse permission.

The isolated W32Time prototype passes twelve stream tests on each supported toolchain.
It retains generated QueryStatus decoding, fixed transport selection, cancellation, reply bounds, and deadline checks.
Native Windows access and the source-age error profile remain unqualified. Product dependencies remain unchanged.

Starport `95354f0` adds configured authority and policy identity pins.
Its [configuration proof](csp4/starport-authority-settings-2026-09-11/verification.json) records twelve focused tests and 394 package race events.
One Valkey integration case skips. Lint, vet, and six dependency checks pass.
Cold construction retains catalog diagnostics and refuses inference without contacting the upstream.
Native clock qualification and the four mapped consumer cases remain open.

Starport `52970f7` enforces current permission before route attempts and cache delivery.
Its [request proof](csp4/starport-request-permission-2026-09-11/verification.json) records 645 package race events and one skipped overhead benchmark.
Both API formats return HTTP 503 before stream headers when permission is unavailable. Four consumer cases remain UNVERIFIED.

The combined public-catalog worktree is `/Users/jack/src/github.com/agentstation/starmap-catalog-public-acceptance`, branch `codex/catalog-public-acceptance`, at clean `0a143a94`.
It includes actual main `32737432`, the provider-documentation repair, and signed public fixtures.
The [acceptance proof](csp4/public-acceptance-2026-09-11/verification.json) records four A07 checks passing after main integration.

The development toolchain passes seven scoped runtime race events. The minimum toolchain passes fourteen runtime/artifact events.
All five negative runtime cases return their expected error category. Failure and restart preserve the accepted catalog.
The verifier regression passes 73 tests. Static checks and the 1,406-file prose check pass.

Integrated runtime, remote, and artifact verification passes 912 events across three packages.
There are no failures or skips. Runtime completes in 1,373.259 seconds. The integrated CSP4 verifier passes four A07 checks.
Its four Starport consumer checks remain UNVERIFIED.

The isolated generation probe passes in 92.017 seconds. Providers, authors, models, and cross-references each report zero issues.
The probe uses the committed source without provider credentials. It creates only a local candidate.


The provider-list API now prefers general documentation and keeps its acquisition-documentation fallback.
Fresh and cached response tests reproduce the gap before this correction. Both supported toolchains pass 82 handler, cache, and OpenRouter events.

Explicit fixture setup now downloads and verifies the published archive before tests. The final tracked tree excludes the archive.
The prepared tests send catalog requests only to their local HTTP fixture. Missing archives yield UNVERIFIED acceptance cases.
CI requires the prepared archive and fails when it is absent. Both absence checks reproduce these outcomes.

The setup passes seven tests for integrity, cache reuse, and failed publication. Anonymous download verifies 411,974 bytes.
The minimum-toolchain race check passes fourteen test events across two packages. Workflow tests pass 28 events.
All four A07 checks pass with the prepared fixture. Lint, ago, complete documentation, and the 1,458-file prose check pass.

The first review attempt refused an output path inside the repository. The second refused a binary change before model review.
The explicit fixture setup resolves the binary refusal without a review bypass. The final verifier adds the seven-test fixture stage.
The earlier complete forty-stage run remains bound to its source commit.

[PR #149](https://github.com/agentstation/starmap/pull/149) delivers the public checks and complete provider-documentation repair.
The original Sol/Opus review passes with zero findings at `079c390b`. Integrated-base review passes with zero findings. All six native jobs pass. Verification Gate and the actual merge remain open.
These local checks do not qualify the released pair or complete CSP4.

The first PR #149 native run fails both public signature tests on both Windows architectures.
Windows text checkout changes captured JSON bytes. A fresh-checkout regression reproduces the changed digest before the attribute correction.

Source `f29ec2a4` preserves those bytes. Eight setup tests, fourteen runtime/artifact race events, and the 1,461-file prose check pass.
Sol and Opus correction review passes with zero findings. The remote PR now names `f29ec2a4`.

The separate Homebrew PR #147 merged as `32737432`. Integration `0a143a94` adds its README and release configuration.
The full PR diff has the same bytes against the new base. Release validation and the 1,462-file prose check pass.
Sol and Opus review passes again against the new base. The remote PR names `0a143a94`.

This separate Homebrew merge adds no campaign credit.

Native PR #149 artifacts record 10,212 passing test events and zero failures. Two ordinary Linux skips pass in separate privileged checks.
There are no unresolved review threads. Automatic squash merge waits for Verification Gate on reviewed `0a143a94`.

Consumer source `20b1ee41` remains clean in `/Users/jack/src/github.com/agentstation/starmap-catalog-consumer-permission`, branch `codex/catalog-consumer-permission`.
The [consumer permission proof](csp4/consumer-permission-2026-09-11/verification.json) records 307 scoped race events per supported toolchain.
The complete catalog suite passes 1,253 events at `3d023778`. Main integration changes only the README and release configuration.
Full runtime, remote, and artifact race verification passes 938 events across three packages at `20b1ee41`.
Required Sol and Opus review passes with zero findings at `20b1ee41`.

`AllowsCatalogAttempt` checks the caller's accepted authority head against current permission state. Runtime readiness alone cannot authorize a withdrawn consumer catalog.
Equal permission revisions permit continued use during route preparation. Contradictory known heads, unknown clocks, expiry, and foreign authority identities refuse.

The consumer must bind the authenticated, validated head to its exact catalog. Starport still needs that integration.

The initial allocation check measured six allocations during checksum validation. The final validator preserves lowercase SHA-256 syntax without temporary decode-and-encode buffers.
Both valid consumer admission and checksum validation now pass their zero-allocation assertions. Static checks, complete documentation, and the 1,457-file prose check pass.
The proof retains the original readiness gap, allocation failure, and contradictory-head test failures.

Starport source `c505a09` is clean in `/Users/jack/src/github.com/agentstation/starport-catalog-authority-consumer`, branch `codex/catalog-authority-consumer`.
The [binding proof](csp4/starport-snapshot-2026-09-11/verification.json) records fourteen focused events and 148 complete catalog race events.
Real Badger tests prove retained authority after reopen and refusal of mismatched candidates, including idempotent repeats.
The private workspace selects Starmap `768347ab`. No module pin changed.

The current published pin lacks the permission API package, so the new Starport source cannot compile against it.
The earlier release verifier passed sixteen checks at `d45bb30`. Architecture gate V01 still requires a stable release.
Permission checks at provider attempts and cache delivery remain incomplete.

Clock-parent verification at `7641eaea` passes 82 ordinary packages, 82 race packages, and fifteen coverage thresholds.
The full command stops at prose checks on five copied research files. Compression preserves their exact bytes.
Corrected prose and the six remaining stages pass. All forty stages have evidence across these runs.
The original full command exits 2. Native clock qualification remains open.

CSP4 remains in progress. Four A07 subcases now pass locally. Four Starport consumer subcases remain UNVERIFIED.

The preserved repair checkpoint is `/Users/jack/src/github.com/agentstation/starmap-catalog-provider-docs`, branch `codex/catalog-provider-docs`, at clean `b81b827067aa860096bed208e541dd84e31274ab`.
The [repair proof](csp4/provider-docs-2026-09-11/verification.json) records 846 race events per supported toolchain across four packages.
Lint, ago, generated documentation, and the 1,415-file prose check pass. The full repository verifier passes all forty stages at `b81b8270`.
Both ordinary and race modes pass 81 packages. All fifteen coverage thresholds pass, and prose covers 1,417 files.

The catalog workflow fails after models.dev documentation creates invalid acquisition metadata for Cohere.
Both parser and fetch mappings now use `DocsURL`. Curated documentation and provider contracts retain their existing authority.
CSP6 owns this publication repair, found during CSP4 public-download qualification. CSP4 remains the only active task.

The public channel still names September 8's schema 6 catalog. Current source emits schema 9 and explicitly supports schema 6 reads.
GitHub CLI verifies the channel and archive attestations. The live Starmap source also verifies and decodes that release without credentials.

The live runtime starts usable on embedded fallback. Explicit refresh activates the signed public generation and clears fallback.
Source health becomes healthy, while catalog freshness remains critical. Provider acquisition stays disabled.

The publication failure blocks freshness, not decoding. The live probes remain separate from the new repeatable A07 tests.
This review made no release or channel changes.

Clock source `7641eaea2b98f9ffbefb2807f67253919227b234` remains clean in `/Users/jack/src/github.com/agentstation/starmap-catalog-native-clock`, branch `codex/catalog-native-clock`.
Its [cache proof](csp4/clock-cache-2026-09-10/verification.json) records 127 permission tests per supported toolchain, including 37 new cache cases.
The cache bounds query delay, counter error, rate drift, age, and concurrent invalidation. No production clock adapter qualifies yet.

The clean parent `7239f4b76ee4d079479ae1703ac6e0ac55683925` remains in `/Users/jack/src/github.com/agentstation/starmap-catalog-permission-clock`, branch `codex/catalog-permission-clock`.
Its [clock proof](csp4/permission-clock-2026-09-10/verification.json) retains the complete-sample tests.
Admission, relay, and permission status use one complete time and uncertainty sample.

Parent verification passes 82 ordinary packages, 82 race packages, and 15 coverage thresholds. The race suite uses host Go 1.27.0.
The original invocation exits 2 because the prose gate scans two untracked Apple research files.
Compression preserves those files without a product change. The prose gate then passes over 1,453 files, and the remaining six verifier stages pass.

The combined proof covers all forty stages. The original command retains its failure and exact recovery evidence.
Main integration preserves both tested clock trees.

The explicitly pinned Go 1.26.6 and Go 1.25.12 authority/status runs each pass 51 events.
Earlier unpinned artifact names do not establish Go 1.26.6 qualification. The proof preserves that correction. No owner decision remains.

Parent `717ba914` remains clean in `/Users/jack/src/github.com/agentstation/starmap-catalog-authority-issuer`.
[PR #148](https://github.com/agentstation/starmap/pull/148) publishes that delivery. It includes actual PR #146 merge `faa9cc8b`.
The [runtime transaction proof](csp4/origin-runtime-2026-09-10/verification.json) records complete repository verification.

Required Sol and Opus review passes with zero findings. All six native jobs pass with 10,170 test events and 92 package outcomes.
Separate privileged Linux tests cover the two ordinary skips.

PR #148 merged as `21e7356b621773fd1808bed5e7a83df7a0683ce7` on September 11 UTC.
All fifteen checks pass. The merged tree equals reviewed `717ba914`. This is the 26th merged campaign PR.

The [recovery record](csp4/worktree-recovery-2026-09-10.json) verifies the restored branches after their temporary directories disappeared.
Canonical checkpoint `91aa43c6` preserves the completed checks. Historical proof keeps its original paths.

The [publication-order proof](csp4/publisher-order-2026-09-10/verification.json) records implementation `4031c084` and the final generated documentation correction.
The publisher checks a durable predecessor before atomic publication. Reopening the store cannot permit an older authority sequence.
An ordinary store requires explicit bootstrap. Established authorities reject repeated bootstrap, except for exact retries.
Unknown permission semantics block new publication while independent metadata remains observable to receipt readers.

Both toolchains pass 46 issuer and publisher race events. Lint, ago, and the 1,434-file prose check pass.
The complete documentation check passes after correcting five stale runtime source links.
The extended runtime suite passes 813 events in 1,286.36 seconds with no failures or skips.
It uses the planned thirty-minute limit. The preceding ten-minute timeout remains in the historical proof.

The [issuer proof](csp4/issuer-library-2026-09-10/verification.json) retains 21 runtime identity events per toolchain and all six external consumer checks.
The [storage proof](csp4/authority-record-2026-09-10/verification.json) retains independent records, current-read capabilities, and migration qualification.
Current catalog and storage suites also pass 1,240 and 88 events respectively.
The publisher proof preserves the nonprivate fixture failure, generic-store ordering gap, and the failed test draft.

The [origin proof](csp4/origin-generation-2026-09-10/verification.json) records the latest preparation library and 80 passing race events per toolchain.
Its 34 new events cover withdrawal, scope and identity changes, invalid sources, alias retention, explicit removal, and independent copies.
The required revision includes all semantic catalog facts. Metadata-only catalog changes can also require a new enforced revision.
Provenance and manifest observation metadata do not affect it. Lint, ago, and the 1,427-file prose check pass.

The [origin publication proof](csp4/origin-publication-2026-09-10/verification.json) adds durable sequence selection with 89 passing race events per toolchain.
Exact retries keep their identity after reopen. Stale proposals cannot replace a committed withdrawal.
Static checks, complete documentation, and the 1,442-file prose check pass.
The [native clock survey](csp4/native-clock-survey-2026-09-10.md) records one macOS observation and selects no adapter.

The runtime now prepares the authority identity before input-journal staging and activates those exact bytes.
Both Go toolchains pass seven origin runtime cases. Final transaction regression passes 36 events with no failures or skips.
Completed root and permission suites pass 112 and 90 events. Final lint, ago, complete documentation, and the 1,451-file prose check pass.

A regression test exposed a bootstrap write before caller-supplied publication guards. The committed implementation puts bootstrap inside the guarded client commit.
The proof retains that failure, the correction, source hashes, and compressed artifacts.
The intermediate runtime suite passes 820 events in 1,386.154 seconds. Its source predates the alias-validation extraction and guarded-bootstrap repair.
Do not report it as final-code qualification.

Full verification passes 82 packages in both ordinary and race modes. All 15 coverage thresholds and the container smoke check pass.
Documentation, prose, static analysis, and offline CLI checks pass. Catalog reads take 8.100–11.43 ns with zero allocations across three runs.

The [delivery proof](csp4/origin-delivery-2026-09-10/verification.json) binds the reviewed head, public PR, and CI run `34540446314`.
The merge now contains this reviewed library and runtime delivery. Dependent branches must include actual main commit `21e7356b`.
Then continue canonical server settings, qualified clocks, shared-store followers, authority transitions, and Starport consumers.
The current APIs do not complete the standalone production recipe.

### First subscriber delivery


The [manifest-arrival proof](csp4/manifest-arrival-2026-09-10/verification.json) records 16 changed files and the original failing TLS regression.
The runtime records a trusted current requirement before payload compatibility checks or transfer.
A stalled download cannot preserve admission after a known withdrawal.
Historical addressed reads do not change current permission requirements.
Shutdown refuses new source callbacks and joins accepted callbacks before sealing retained state.

The final focused race check passes 40 test events and two package outcomes on each supported toolchain.
These checks include authority admission, observer binding, shutdown, relay, and strict source startup.
The complete remote protocol suite passes 60 test events and one package outcome.
Lint, ago, generated documentation, prose, and all six external consumers pass.
Prose checks cover 1,415 files with zero diagnostics. The proof preserves the complexity and comment failures before their repairs.

The complete runtime, remote, and artifact race suite passes 877 test events and three package outcomes without failures or skips.
It includes the observer and shutdown changes. Compilation preceded the equivalent source-startup extraction and comment correction.
The final focused checks cover that extraction.
The preceding relay suite completed 875 passing test events and three package outcomes.
The [relay proof](csp4/relay-2026-09-10/verification.json) preserves that separate tested source.

The first repository check failed two YAML authority fixtures under the wrong source kind.
Local `44a400a4` corrects the fixture source context. All 28 focused YAML events pass on both toolchains. Lint and ago pass.

The [verification proof](csp4/subscriber-verification-2026-09-10/verification.json) preserves the failed gate and focused regression.

The complete retry passes at clean `44a400a4`. Both test modes pass 81 packages, and the container smoke check passes.
All 15 coverage thresholds, static checks, isolated CLI checks, and the catalog accessor budget pass.
The generator rejects `GOFLAGS=-p=2` despite a successful wrapper exit. A separate `GOFLAGS= make docs-check` run passes without that diagnostic.

Required Sol and Opus review passes with zero accepted or actionable findings across two chunks.
The [publication proof](csp4/subscriber-publication-2026-09-10/verification.json) binds PR #146 and CI run `34510928894` to reviewed `44a400a4`.
PR #146 merged as `faa9cc8b0bc01eb787f567f568f1b9fe05e0740a` at 19:39:56 UTC. All fifteen checks pass, including six native jobs.
The merged tree equals reviewed `44a400a4`. CSP4 remains in progress.

Native artifacts record 9,714 passing JSON test events and 92 successful package outcomes across six runners.
The ordinary Linux runs skip the administrator-owned configuration check. Both separate privileged runs pass that check.
These component results give no complete product acceptance credit.

The merged 91-file delivery owns permission transport, retained subscriber enforcement, and relay.
Local merge `e27c473e` includes actual main and preserves the committed issuer tree.
Three overlapping files required resolution from the qualified issuer source. Both draft publisher files remain intact.
The [current authority storage contract](csp4/issuer-storage-contract-2026-09-10.md) owns the next storage change.

The next Starmap delivery owns origin issuance, qualified clock evidence, shared-store follower activation, and authority transitions.
The [consumer inspection](csp4/starport-consumer-inspection-2026-09-10.md) requires a separate Starport PR after a compatible Starmap module release.
It preserves all eight task checks. Complete gateway admission coverage remains with CSP10.
The first merge gives no task completion credit. All acceptance requirements remain in force.

The [clock research](csp4/authority-head-2026-09-10/clock-research.md) selects no clock adapter.
The [issuer storage inspection](csp4/issuer-storage-inspection-2026-09-10.md) records the gap before the current storage checkpoint.
The object backend promises conditional writes, but does not promise current reads. Receipt issuance requires both guarantees and bounded observation timing.
Complete product verification, required review, native CI, and merge before task completion.

## Runtime integration checkpoint history

### Permission relay checkpoint

CSP4 remains in progress. All eight assigned acceptance subcases remain UNVERIFIED.
The current local source is `cc4e2a017c779517251a4a050e335f0a9f77b260`. No CSP4 PR exists.
The worktree has one task-owned regression file: `runtime/authority_manifest_arrival_test.go`. No owner decision remains.

The [relay proof](csp4/relay-2026-09-10/verification.json) records 21 passing focused test events on `go1.25.12`.

The complete server suite passes 329 test events and ten package outcomes before the final schema correction.
The schema-normalizer suite passes all 63 test events. Final focused checks cover the generated wire schema.
Lint, ago, generated documentation, prose, and all six external consumers pass. Prose checks cover 1,409 files with zero diagnostics.

`Runtime.ReadPermission` forwards a confirmed upstream receipt from memory with its original expiry.
It can report an unsupported permission revision before catalog activation.
The focused check records zero allocations and no upstream permission reads.
The HTTP endpoint applies configured API authentication, uses `no-store`, and refuses unavailable receipts with a generic 503.

The proof preserves the missing-route failure, a compile error, the cold-fixture timing failure, and the generated schema defect.
It also records the YAML-offset and expression-lint repairs. These failures do not count as acceptance passes.

The complete runtime, remote, and artifact race suite remains running in session `14376`.
Its output is `/tmp/starmap-csp4-permission-relay-runtime-suite-2026-09-10.jsonl`.
Runtime and remote source match the current commit. Compilation preceded the later schema tooling correction.

The new TLS regression confirms that a trusted withdrawal manifest leaves old permission active during a stalled payload transfer.
The required revision also remains old. The final red test reports these two assertions without a cleanup failure.

Next, propagate the trusted requirement before payload processing and rerun the regression.
Then complete origin receipt issuance, qualified clock evidence, shared-store follower activation, authority transitions, and required consumer checks.
The [clock research](csp4/authority-head-2026-09-10/clock-research.md) selects no clock adapter.
Complete product verification, required review, native CI, and merge before task completion.

### Committed head checkpoint

CSP4 remains in progress. All eight assigned acceptance subcases remain UNVERIFIED.
The current local source is `671b54d531f928f65d550411190293d4cde842b0`. No CSP4 PR exists.
No command remains running. No owner decision remains.

The [head snapshot proof](csp4/authority-head-2026-09-10/verification.json) records 106 passing root test events and one package outcome.
Its focused root/runtime check passes 17 test events and two package outcomes.

The minimum-toolchain check passes one test and one package outcome. Static checks pass across 1,402 prose files with zero diagnostics.
The complete runtime, remote, and artifact suite passes 864 test events and three package outcomes against predecessor `a9b369dc`.
These completed suites contain no failed or skipped events. They do not qualify all CSP4 product cases.

The runtime retains authority requirements separately from finite receipts.
It preserves refusal after a known withdrawal or rejected receipt. Ordinary renewal keeps only the previously confirmed lease until the replacement is durable.
Private retention supports bounded clean restart and requires fresh evidence after a crash.
Client guards reserve update, activation, and rollback for the owning runtime.

The [generation-binding proof](csp4/generation-binding-2026-09-10/verification.json) verifies exact generations after refresh and retained restart.

`CurrentAuthorityHead` reads the committed record from memory with no storage reads and zero allocations in the focused test.
That snapshot does not authenticate a publisher, prove fleet freshness, or renew permission.
The [integration proof](csp4/runtime-integration-2026-09-10/verification.json) preserves earlier failures and their repairs.

Next, implement publisher receipt issuance, qualified clock evidence, shared-store follower activation, authority transitions, and required consumer checks.
The [clock research](csp4/authority-head-2026-09-10/clock-research.md) identifies restart, suspend, replay, and timing constraints. It selects no clock adapter.
Complete product verification, required review, native CI, and merge before task completion.

The following record preserves earlier results and commands that were still running when recorded.
The current section above owns execution state.


CSP4 remains in progress. All eight assigned acceptance subcases remain UNVERIFIED.
Local commit `671b54d531f928f65d550411190293d4cde842b0` follows generation binding `a9b369dc`, ownership fix `ea4f7f11`, and runtime integration `9ddf5fd0`.
The [runtime proof](csp4/runtime-integration-2026-09-10/verification.json) binds 47 changed files to the committed tree.

Each final focused suite passes 29 test events and two package outcomes.
The suites use Go 1.26.6 and Go 1.25.12.
Lint, ago, generated documentation, and prose checks pass. The prose check covers 1,400 files with zero diagnostics.
The complete root race suite passes 105 test events and one package outcome.

The runtime retains the highest authenticated requirement separately from its finite receipt.
It checks configured authority and policy identities before activation.
The private `catalog-runtime/permission.json` checkpoint stays uncertain until shutdown stops every reader and retains the complete state.
A crash needs a fresh verified receipt before admission. A clean restart can use a valid retained receipt with a qualified clock.

The runtime reserves client publication through cumulative guards on `Update`, `Activate`, and `Rollback`.
Direct calls cannot alter the serving catalog or add alias history that prevents trusted-source recovery.
Local provider acquisition stops before provider calls in `require_authority` mode.
The serving client and retained authority catalog align before readiness.

A same-permission renewal preserves the confirmed lease and its original expiry until the replacement receipt is durable.
A changed requirement or unknown permission semantics blocks new attempts immediately.
A catalog manifest cannot restore a rejected permission receipt.
Admission reads the runtime snapshot and cached clock evidence. These component checks do not measure Starport overhead.

The proof retains failures for local acquisition, warm client alignment, direct client publication, rollback, ordinary renewal, and rejected-receipt recovery.
An intermediate broad run passed 1,012 test events and failed three settings checks.
The complete settings repair suite passes 98 test events plus two package outcomes.
The proof preserves an earlier root API-list failure and two command syntax errors that ran no tests.
None of these intermediate runs qualifies the final runtime tree.

The [generation-binding proof](csp4/generation-binding-2026-09-10/verification.json) records a second publication defect and its repair.
The runtime now activates the original authority generation with its complete manifest and payload.
The exact generation survives refresh and retained restart. Both toolchains pass 27 authority test events and one package outcome.

An unchanged generation requires no catalog-store write. This preserves retained diagnostics during a write outage.
The complete runtime, remote, and artifact race suite remains running. Current resume state names its session and output path.

The [head snapshot proof](csp4/authority-head-2026-09-10/verification.json) records the latest nine-file change.
`CurrentAuthorityHead` reads the committed authority record from memory.
Its focused test records zero storage reads and zero allocations.
Construction, activation, ordinary publication, rollback, and failed writes preserve the expected head.
The final root/runtime check passes 17 test events and two package outcomes. The minimum-toolchain check passes one test and one package outcome.

The snapshot does not authenticate a publisher, prove fleet freshness, or renew permission.
The current root suite remains running. The older complete runtime suite tests `a9b369dc` before this root change.
Current resume state names both sessions and output paths.

Next, complete publisher receipt issuance, the host clock adapter, shared-store follower activation, authority transitions, and required consumer checks.
Full CSP4 verification, review, native CI, PR, and merge remain pending.
No owner decision remains. No task completion credit applies.

## Historical permission-envelope checkpoint

CSP4 remains in progress. All eight assigned acceptance subcases remain unverified.
Local commit `1f1b01148f86115a053a0dc8ac9fd77c5b8a9b47` adds the permission-envelope contract.
The [primitive proof](csp4/permission-envelope-2026-09-10/verification.json) records 1,212 catalog race events and 54 minimum-toolchain events.
Both suites pass without failed or skipped events. Static checks pass.

The envelope binds authority, policy, publication sequence, payload identity, and required permission revision.
A renewal can extend a receipt without changing the publication head.
Its time check uses the approved five-minute lifetime and thirty-second uncertainty ceiling.
Unknown clock validity refuses the receipt. The isolated time check allocates no memory.
This result does not measure Starport request overhead.

The strict parser rejects duplicate fields, case variants, unknown fields, invalid UTF-8, trailing documents, and oversized envelopes.
Three case-variant regressions fail before the parser fix.
Unknown positive permission versions remain readable so a consumer can record a withdrawal before refusing unsupported semantics.

The [startup baseline](csp4/startup-baseline-2026-09-10/verification.json) retains the two failing authority probes.
The [preparation](csp4-preparation-2026-09-10.md) names the transport and retention boundaries.
Independent authenticated receipt reads, publication, retention, runtime activation, and admission enforcement remain incomplete.
Complete those paths before task qualification or publication.

The [context record](csp4/permission-envelope-2026-09-10/context-maintenance.json) verifies unchanged task contracts, goal, and invariants during history rotation.

## Independent permission transport

Local commit `12c89f6901c7fb07fe7257fca87c138d60458bf6` adds the independent permission request.
The [transport proof](csp4/permission-transport-2026-09-10/verification.json) records 51 remote-client race events and 16 minimum-toolchain events.
All pass without skips. Static checks pass.

The transport verifies the configured publisher before decoding the envelope.
It bounds the body before parsing and preserves cancellation and refusal deadlines.
A renewal reads no catalog payload. Runtime and publisher integration remain incomplete.

## Committed authority binding

Local commit `115f0c7e4a6b7c5f848ca2918bdc7c6db97fa974` adds authority manifest version 3.
The [manifest proof](csp4/authority-manifest-2026-09-10/verification.json) records 1,525 catalog-package race events and 72 minimum-toolchain events.
All pass without skips. Static checks pass.

The manifest stores the authority head with its catalog generation.
Validation binds the generation ID and payload digest. Ordinary manifests retain version 2.
Memory, filesystem, and the conditional object adapter preserve this binding through compare-and-swap.
The object test uses the existing memory backend fixture. It does not qualify an external service.

The filesystem interruption test preserves the prior catalog and permission revision after reopen.
A retry publishes the complete replacement. The permission receipt can renew against that immutable head.
Publisher policy, receipt issuance, retained permission state, and runtime admission remain incomplete.

## Runtime permission state draft

The active worktree contains seven uncommitted files above `d2c738d6`.
The [draft proof](csp4/runtime-state-draft-2026-09-10/verification.json) preserves their exact bytes, test output, and remaining lint findings.
The permission state passes nine race events on each Go toolchain.
The full remote-source suite passes 45 events. Its focused minimum-toolchain test passes.

The late-retention regression fails before the fix. A pending confirmation now cannot restore permission after an invalid authenticated receipt.
The isolated state check allocates no memory. It does not measure gateway latency.

Whole lint reports five unused private declarations.
Connect the state to startup, independent receipt refresh, durable retention, and readiness before clearing those findings.
Do not suppress them or count the draft as task completion.

The [main integration](csp4/main-integration-2026-09-10.json) records actual CSP3 merge `a87262e3`.
The integration preserves the qualified committed tree and every draft file.


### Starport permission binding and authority order

The [order proof](csp4/starport-order-2026-09-11/verification.json) records sequence checks and explicit-transition refusals.
The [permission proof](csp4/starport-permission-2026-09-11/verification.json) records snapshot binding to live permission, with 171 catalog/registry race events.
No request-path call, native qualification, or consumer acceptance credit applies yet.
