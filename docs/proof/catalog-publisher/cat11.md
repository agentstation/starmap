# CAT11 proof: the hosted channel, the releases, and the Starport pin

CAT11 verified the public catalog channel from outside the repository. The
first checks found two defects in `v0.16.2`. Two pull requests corrected them,
and `v0.16.3` carries the corrections. The container check on `v0.16.3` found a
third defect in the connected runtime. Pull request #121 corrected it,
`v0.16.4` carries that correction, and Starport pins that release. Two findings
stay open for an owner decision: the immutable channel release blocks the
scheduled publisher, and a durable runtime reports a local generation
identity.

## The public channel

The publisher dispatch on main published channel sequence 1. The channel
document is the `catalog-latest.json` asset of the immutable `catalog-latest`
release. It names generation `1e23f911-ec24-4f9d-95a3-38059b347f7e`, tag
`catalog-d109a3e2…`, and a `catalog_digest` of `sha256:d109a3e2…`. The
document published on 2026-09-04 at 06:30 UTC. The immutable release at that
tag holds three assets: the archive, its checksum file, and the in-toto
statement.

**The attestations.** `gh attestation verify` accepted both public artifacts
with the repository, the signer workflow, and the hosted-runner restriction.

| Artifact | Digest | Predicate | Signer |
| --- | --- | --- | --- |
| `catalog-latest.json` | `sha256:a23e3814…` | SLSA provenance v1 | `catalog-generation.yaml@refs/heads/main` on a GitHub-hosted runner |
| `starmap-catalog.tar.gz` | `sha256:36c3ef9a…` | SLSA provenance v1 | `catalog-generation.yaml@refs/heads/main` on a GitHub-hosted runner |

## The first defect: the channel digest

**The observation.** A clean Go module pinned to `v0.16.2` opened the
connected runtime against the public channel and reported a fallback. The
consumer compared the exact payload checksum of the verified generation with
the channel `catalog_digest`. The publisher keys that digest by the facts-only
semantic checksum. The two values differ for every generation, so no connected
consumer of `v0.16.2` can activate a channel release.

**The cause.** The fixtures tagged and recorded releases by the payload
checksum on both sides, so no test compared the consumer with the publisher
vocabulary. The clean consumer was the first check that combined a real
publisher output with the consumer.

**The repair.** Branch `codex/channel-catalog-digest` adds
`Generation.SemanticChecksum` to `pkg/catalogs` and keys the consumer release
digest by it. The publisher moves onto the same method, and the fixtures change
to the publisher vocabulary. The regression test fails at `789dc26a`
on five source tests and passes on the branch. The pre-PR review ran on Opus 5
at high effort and returned clean. Pull request #119 squash-merged as
`18b82ec2` on 2026-09-04 at 07:53 UTC.

**The rate limit.** A second anonymous consumer run in the same hour reached
the GitHub REST limit. An unauthenticated client gets 60 requests per hour.
The architecture document now records that limit and the authenticated
alternative. The clean consumer check therefore used a fresh module cache and
one run per hour.

## The second defect: the container state directory

**The observation.** The documented standalone invocation in `docs/DOCKER.md`
mounts only `/tmp` on the read-only root. The `v0.16.2` image exited at
startup:

```text
opening the catalog runtime: IO error during create of /home/nonroot/.starmap/state/runtime/github-catalog-source: mkdir /home/nonroot/.starmap: read-only file system
```

The documented durable invocation mounts the volume at
`/home/nonroot/.starmap`. Docker gives a new volume the ownership of the image
path, and the image owns only `/home/nonroot` for the unprivileged user. The
server could not create `state` under the root-owned mount point.

**The repair.** Branch `codex/docker-state-directory` adds a `tmpfs` mount at
`/home/nonroot/.starmap` to the evaluation and HTTP-security examples. It
mounts the durable volume at `/home/nonroot` and names `state/runtime/` in the
layout. It states that the server pulls the public channel after the embedded
bootstrap. The change is documentation only, so the pre-PR gate skipped the
model review. Pull request #120 fell behind main after pull request #119
merged, and `gh pr update-branch` restored it. It squash-merged as `112ec64f`
on 2026-09-04 at 08:38 UTC.

**The container check on `v0.16.2`.** The three corrected invocations answered
`/health` and `/api/v1/ready` with status 200 within three seconds.

| Invocation | Writable path | Ready |
| --- | --- | --- |
| `tmpfs` at `/home/nonroot/.starmap` | ephemeral | 200 |
| volume at `/home/nonroot` | durable | 200 |
| `STARMAP_STATE_DIR` under `/tmp` | ephemeral | 200 |

Each container reported the embedded generation with `fallback` true and
`fallback_reason` `awaiting_source` before its first source poll. The later
poll on `v0.16.2` reported the digest defect, as expected before the repair.

## The release

**The simulation.** The release test job ran locally at `112ec64f` with the
pinned `agentstation/skills` revision under `.ci`, before the tag.
`make embedded-catalog-budget-check` reported `passed` true, `make verify`
reported `repository verification passed`, and `make release-check` reported
`Ready for release`. The tree equals the tree of `origin/main` at that commit.

**The tag.** The annotated tag `v0.16.3` points at `112ec64f`, created on
2026-09-04 at 08:39 UTC. The tag push started release run 33854489216.

**The result.** The test, release, and Homebrew verification jobs of run
33854489216 passed. GitHub release `v0.16.3` published on 2026-09-04 at 09:06
UTC as an immutable release with fourteen assets. The image
`ghcr.io/agentstation/starmap:v0.16.3` resolves to digest
`sha256:a44eb3e8…`.

## The clean consumer on `v0.16.3`

A fresh module `cat11consumer` requires `github.com/agentstation/starmap v0.16.3`
with no `replace` directive and an empty module cache. It opened the connected
runtime against the public channel and printed the runtime status.

| Field | Value |
| --- | --- |
| `usable` | true |
| `fallback` | false |
| `generation_id` | `1e23f911-ec24-4f9d-95a3-38059b347f7e` |
| `channel_freshness` | current |
| `source_identity` | `public_github` |
| `source_health` | ok |
| models and providers | 596 and 17 |
| `payload_checksum` | `sha256:b6634e0f…` |

The generation equals the channel document. The consumer activated the
published generation with no fallback.

## The third defect: unlinked provider offerings in the runtime

**The observation.** The `v0.16.3` image ran read-only with a `tmpfs` at
`/home/nonroot/.starmap` and the public channel as its source. It answered
`/api/v1/ready` with status 200 and reported the embedded generation with
`fallback_reason` `awaiting_source`. The first acquisition poll fetched 189
DeepInfra models from the public model endpoint, which needs no token, and then
logged:

```text
Scheduled runtime work failed: failed to publish effective catalog catalog-20260830T083213Z-ad5d2a45e490: failed to index catalog read views: validation failed for field provider_model.model: explicit canonical author/model reference is required
```

The source poll then pulled generation `1e23f911…` from the channel and failed
with the same error. After both polls, the ready document reported
`fallback` false, `channel_freshness` current, `acquisition_health` degraded,
`source_health` degraded, and `source_reason` `publication_failed`. The served
generation stayed at the embedded baseline. The connected consumer in the
section above did not expose the defect, because it holds no provider
credentials and the acquisition controller never ran there.

**The cause.** The connected runtime in `runtime/layers.go` merged every
observed offering of a retained provider layer with the enrich strategy and
then built the effective catalog. The CLI update path applies offering
quarantine in the reconciler, and the runtime never applied it. A live
provider reply carries offerings that the baseline does not link to an
authored model. One such offering fails the build for the complete layer set.
The retained provider layer then blocks every later provider poll and every
later channel pull of that runtime.

**The repair.** Branch `codex/runtime-offering-quarantine` from `112ec64f`
links each observed offering to an authored model before the merge. An
offering keeps a valid link, inherits the link of the same offering in the
baseline, or stays out of the effective catalog. The build logs the count of offerings that stayed
out per provider.

**The tests.** Two regression tests in `runtime/layers_test.go` reproduce the
container failure. One proves the layer build alone. One proves that a
retained layer with unlinked offerings never blocks a later source generation.
Both fail at `112ec64f` with the exact container error and pass with the
change.

**The gates.** The 72 test packages, lint, ago, and docs-check pass at
`fe604864`. The pre-PR review ran on Opus 5 at high effort in one pass and
returned clean. Pull request #121 opened with auto-merge armed.

**The merge.** Verification Gate run 33861540174 passed, and the squash merge
landed on main as `45b1ea03` on 2026-09-04 at 10:34 UTC.

## The fourth release

**The simulation.** The release test job ran locally at `45b1ea03` with the
pinned `agentstation/skills` revision under `.ci`, before the tag.
`make embedded-catalog-budget-check` reported `passed` true, `make verify`
reported `repository verification passed`, and `make release-check` reported
`Ready for release`.

**The tag and the result.** The annotated tag `v0.16.4` points at `45b1ea03`.
The tag push started release run 33864706487. The test, release, and Homebrew
verification jobs passed, and the two recovery jobs skipped as designed. GitHub
release `v0.16.4` published on 2026-09-04 at 11:17 UTC as an immutable release
with fourteen assets. The image `ghcr.io/agentstation/starmap:v0.16.4` resolves
to digest `sha256:9bf9d815…`.

## The container on `v0.16.4`

**The invocation.** The `v0.16.4` image ran with the same read-only root, the
same `tmpfs` at `/home/nonroot/.starmap`, and the public channel as its source.
Before the first poll, the ready document reported the embedded generation
`catalog-20260830T083213Z-ad5d2a45e490` with `fallback` true and
`channel_freshness` critical.

**The polls.** The acquisition poll fetched 189 DeepInfra models, and the
runtime logged one information line:

```text
{"level":"info","provider_id":"deepinfra","unresolved_offerings":31,"message":"Provider offerings without a canonical model reference stay out of the effective catalog"}
```

The other providers skipped with `credential_unavailable`. The sixty log
lines hold no `Scheduled runtime work failed` line, and the source poll
activated the channel generation.

**The ready document after both polls.**

| Field | Value |
| --- | --- |
| `usable` | true |
| `fallback` | false |
| `generation_id` | `fd7a6202-310d-4bc6-9410-53997ce2a43f` |
| `channel_freshness` | current |
| `source_kind` | public |
| `source_health` | ok |
| `upstream_health` | ok |
| `acquisition_health` | degraded |
| `lease` | `lease_not_required` |

The design expects the degraded acquisition health. `runtime/refresh.go`
marks the controller degraded when any provider failed or skipped, and nine
providers skipped without credentials. The change corrects the third defect.

**The generation identity.** The served generation is a fresh random UUID. It
is not the channel generation `1e23f911…`, and it is not the derived local
identity `1e23f911…+local.<fragment>` that the clean consumer form would
report. The cause sits in the durable commit path. The `commit` method in
`runtime/runtime.go` calls `Client.Update` when the client publishes durably.
`newGeneration` in `generation.go` then mints a new identity with `NextID`.

The derived identity from `deriveEffectiveGenerationID` in `runtime/layers.go`
survives only in an in-memory runtime. `starmap serve` always wires a
filesystem catalog store. Starport wires a catalog store in both runtime
compositions. Every serving instance therefore reports a local UUID. The
runtime tests open in-memory runtimes, so no test observed the durable path.
The `Client.Activate` path preserves the manifest identity for a received
generation, and the runtime does not use it. This finding changes no served
data, and it stays open for the owner decision below.

## Starport

**The pin.** Branch `codex/starmap-v0.16.3` from main `4a52d48` carries two
commits. Commit `71a507e` moves the pin to `github.com/agentstation/starmap
v0.16.3`, and commit `537a48e` moves it to `v0.16.4`. Each changes `go.mod`
and `go.sum` only.

**The gates at `71a507e`.** Every command ran with `GOTOOLCHAIN=go1.26.6`. The
two verification scripts that read a Starmap tree pointed at the downloaded
`v0.16.3` module.

| Check | Result |
| --- | --- |
| `go build ./...` and `go vet ./...` | pass |
| `go test ./...` | pass |
| `make lint` | 0 issues |
| the 26 `scripts/verify-*.sh` and `scripts/test-*.sh` gates | 26 pass |
| `scripts/smoke-first-run.sh` | pass |

**The gates at `537a48e`.** The same commands ran again against the downloaded
`v0.16.4` module.

| Check | Result |
| --- | --- |
| `go build ./...` and `go vet ./...` | pass |
| `go test ./...` | 55 packages pass |
| `make lint` | 0 issues |
| the 26 `scripts/verify-*.sh` and `scripts/test-*.sh` gates | 26 pass |
| `scripts/smoke-first-run.sh` | pass |
| Starmap `scripts/verify-catalog-distribution.sh` against this tree | 68 passed, 0 failed, 0 unverified |

**The pull request.** The branch pushed with an explicit refspec, and pull
request #361 opened from it. The change is dependency only, so the pre-PR gate
skipped the model review. Starport has no auto-merge, so the merge waited for
green hosted checks and a manual action.

**The result.** Run 33867553855 at `537a48e` passed all ten hosted jobs. Pull
request #361 squash-merged as `febbad3` on 2026-09-04, and the Starport main
run 33868953313 at `febbad3` completed with success.

## The fourth defect: the immutable channel release

**The observation.** The scheduled publisher run 33855316906 started on
2026-09-04 at 08:49 UTC from main `112ec64f`. It created and attested the
immutable release `catalog-dbfafb9f…` at 08:53 UTC with three assets. The
channel step then failed:

```text
HTTP 422: Validation Failed (https://api.github.com/repos/agentstation/starmap/releases/assets/543903772)
Cannot delete asset from an immutable release
```

The channel document still names sequence 1 and generation `1e23f911…`. The
new immutable release exists, and no connected consumer can discover it.

**The cause.** The repository enables the immutable releases setting. The
channel step in `.github/workflows/catalog-generation.yaml` creates the
`catalog-latest` release once. Each later sequence replaces its one asset
with `--clobber`. GitHub made the channel release immutable at creation
on 2026-09-04 at 05:43 UTC, so no later upload can replace the asset.

**The earlier success.** The dispatch run 33844382175 at 06:26 UTC succeeded.
It ran before the next sequence needed a replacement. `docs/RELEASES.md` and
the CAT2 design assume a mutable discovery release.

**The decision.** The repair changes the public channel protocol or the
release process, so it needs an owner decision. The candidate directions follow.

- A per-sequence channel release with a discoverable name.
- A draft channel release that GitHub keeps mutable.
- A different discovery surface outside releases.
- A workflow step that reads the repository setting and refuses the run with
  a clear error.
 The generation
identity finding above needs a runtime change and a fifth release. One release
can carry both repairs.

## Open findings

| Finding | State | Owner decision |
| --- | --- | --- |
| the immutable channel release blocks every later channel sequence | open, no repair | select the channel direction |
| a durable runtime reports a local UUID instead of the channel-derived identity | open, no repair | confirm the repair at the runtime commit seam |

## Untouched by this task

CAT11 changed one consumer digest comparison, the publisher digest method, the
fixtures, and one architecture paragraph. It changed the container document
and the offering link step of the runtime layer build. The channel document format, the release
layout, the attestation identity, and the connected runtime state layout did
not change. A Starport gateway behaves as before with the new pin, except that
unlinked provider offerings now stay out of its effective catalog.
