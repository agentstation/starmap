# CSP23 release pair decisions (2026-10-05)

The lead recorded these questions before it asked the owner. The owner reported disappearing questions. This file keeps each question until the owner answers it.

## State before the decisions

- Starmap main is `753b2e551`. The last application release is `v0.16.5` from 2026-09-04. Main holds 643 commits after that tag.
- Starport main is `635fb25fc`. The last release is `v1.2.1` from 2026-09-11. Main holds 47 commits after that tag.
- Starport pins Starmap `v0.16.6-0.20261003000954-595e3c7ba959`. That pseudo-version is 12 commits before Starmap main.
- The CSP22 candidate gate at these trees reports 308 PASS and 4 UNVERIFIED of 312 subcases.
- `goreleaser check` passes on both repositories. The release workflows run `make verify` and `make release-check` again on the tag.
- Both release workflows start from a `v*` tag push. The tag commit must be an ancestor of `main`.
- A release is immutable. GitHub refuses asset replacement and asset deletion after creation.
- A stable Starmap tag updates the public Homebrew tap. A release candidate tag does not.
- A stable Starport tag publishes the container image, the Homebrew cask, and the native installers.

## Sequence that CSP23 needs

1. A Starmap release pull request moves the `[Unreleased]` changelog section under the version heading.
2. The owner authorizes the Starmap tag. The lead pushes the tag. The release workflow publishes the release.
3. A Starport pull request pins the released Starmap version and moves its changelog section.
4. The owner authorizes the Starport tag. The lead pushes the tag. The release workflow publishes the release.
5. The lead verifies A06 and A29 against the released assets and records `csp23.md`.

## Owner answers of 2026-10-05

- The owner approved the Starmap version `v0.17.0` as a stable tag at main `753b2e551`.
- The owner approved the Starport version `v1.3.0` as a stable tag after the Starmap pin pull request merges.
- The owner wrote a pause on CSP23 and CSP24 in the third answer. The lead held the tags. The owner then asked why no tag and no release followed, and gave explicit permission for the release commands. The release work resumed on 2026-10-05.
- The owner skipped the dedicated runner. CSP22.1 stays a documented limit.
- The only real inference that CSP24 needs is the README recording: one quickstart session with one real streamed answer. A35 through A39 verify that recording and the installers.
- GitHub Pages, real provider inference, and the dedicated runner stay outside the current authority.

## Release progress of 2026-10-05

- Starmap PR #232 merged as `2b2944be7`. The lead pushed the annotated tag `v0.17.0` on that commit. Release run `37389198612` started.
- Starmap PR #233 registers `A06.new_released_module` and `A06.starport_released_module_pin`, and extends `A06.old_pinned_bytes_unchanged` with the previous-pin comparison. It is scripts only.
- Starport branch `codex/csp23-release-v1.3.0` pins Starmap `v0.17.0`. The test suite and the verify gates run before the pull request.
- Release run `37389198612` stopped at the 75-minute limit of the `test` job after 75 minutes and 17 seconds. The serial race shards used 58 minutes before the application shards started. GoReleaser never ran. No draft release and no `release-dist` artifact exist. The tag stays on `2b2944be7` because the Go checksum database already records the module bytes.
- Starmap PR #234 lets a dispatch publish an existing tag with an empty `source_run_id` and raises the `test` limit to 180 minutes. The lead dispatches `release.yaml` with `tag=v0.17.0` after the merge.
- Starmap PR #233 merged as `ee423bd9f`.
- Starport PR #424 pins Starmap `v0.17.0`. Its `Catalog Fleet Recovery` check stopped at the 30-minute limit of that job. The same job on main `635fb25fc` took 29 minutes and 37 seconds. The check is not required. The lead reruns the job after the workflow completes.

## Public site decision of 2026-10-05

The owner answered the public documentation question twice on 2026-10-05.

1. First answer: the public host is `starport.agentstation.ai`. The `agentstation.ai` zone is on Cloudflare. The owner permits the use of the Cloudflare account for the setup and permits delegation to Codex. The agentstation domains may need a transfer into the agentstation Cloudflare account.
2. Second answer: `starport.agentstation.ai` serves the Starport splash page. The documentation lives at `starport.agentstation.ai/docs`. The site uses Fumadocs in the same way as `https://docs.open-e2ee.dev/`.

Facts that the lead verified on 2026-10-05:

- `docs.open-e2ee.dev` is the open-e2ee console: Next.js 16 with `fumadocs-core` 16.11.4, `fumadocs-mdx` 15.1.1, and `@fumadocs/base-ui`. The `(home)` route group holds the splash page and `/docs` holds the documentation. Vercel serves it. The DNS record is a CNAME to Vercel on Cloudflare.
- `agentstation.ai` resolves through `remy.ns.cloudflare.com` and `mina.ns.cloudflare.com`. `open-e2ee.dev` resolves through `jack.ns.cloudflare.com` and `dora.ns.cloudflare.com`. The two zones are in different Cloudflare accounts.
- `starport.agentstation.ai` has no DNS record.

Consequences for the plan:

- The GitHub Pages path in `docs-pages.yaml` serves raw versioned documentation at `/<tag>/`. It does not produce a splash page or a Fumadocs site. The owner's direction supersedes the GitHub Pages option.
- The site is a new deliverable with its own repository location, build, hosting, DNS, and deployment approval. It is not part of the release pair. CSP23 closes with `A29.public_url_content_manifest` and `A29.hosting_rollback` as documented limits that point to the new task.
- The lead adds a sibling task for the public site after CSP23 closes. The task brief names the repository location, the hosting target, and the DNS change. It also names the release-to-site publication contract and the A29 subcases.

## Owner answers on the public site (2026-10-05, later)

1. Hosting target: Cloudflare Workers with static assets. The site is a static export. It has no login and no database. The OpenNext adapter is not needed.
2. Repository location: `website/` in `agentstation/starport`. Starport PR #425 holds the site, the `Site` workflow, and the `Deploy` job.
3. Timing: now, in parallel with the release pair.

## Owner answers on Cloudflare access (2026-10-05, later)

- Access method: the owner chose the command line. The owner asked for web research on the new `cf` CLI before a tool choice.
- Research result (2026-10-05): `cf` is an open beta since 2026-09-28. It is a whole-API agentic CLI with a `cloudflare.config.ts` file. It has no `assets.directory` setting for a static site without Vite. It writes a generated Wrangler config that can change during the beta.
- Research result, continued: the `cf domains` command does not create DNS records. Cloudflare keeps Wrangler in maintenance for 18 months after the beta ends. Decision: keep Wrangler for the deploy. Revisit `cf` after the beta.
- First deploy: the owner chose "wrangler login now". The lead deploys from the site worktree after the login. The `website/wrangler.jsonc` file binds `starport.agentstation.ai` as a Worker custom domain, so the deploy creates the DNS record and the certificate.
- Account membership: `wrangler whoami` lists only `OpenE2EE LLC`. The `agentstation.ai` zone is in an account where the `jack@open-e2ee.dev` user is not a member. The lead recommended a member invite into that account instead of a logout. Wrangler reads memberships live, so the invite needs no second login.
- Durable flow: the owner later creates an API token and sets the `starport-site` environment secrets. The lead never handles the token value.

## Pending owner decisions

1. Zone move for the first deploy. Waiting on the owner. The owner logged in as `jack@agentstation.ai` at 01:33Z on 2026-10-06. `wrangler whoami` lists the `AgentStation` account `c4ea376a1fc44013b2184fa13b5f4c54`. The first `wrangler deploy` uploaded the Worker `starport-site` into that account. The custom domain binding failed with Cloudflare code 10083: the `agentstation.ai` zone does not exist on the account. The zone is in a third Cloudflare account.
   - Owner decision (2026-10-06): move the `agentstation.ai` zone into the `AgentStation` account. The owner does the move in the Cloudflare dashboard. The lead reruns the deploy after the move, and the deploy creates the DNS record and the certificate.
   - The lead cannot log in, create an API token, or move a zone.
   - Later, for the durable flow: the owner creates an API token. The token needs Workers Scripts edit, zone DNS edit, and zone Workers Routes edit. The owner sets the `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` secrets in the `starport-site` environment.
2. Real provider inference for CSP24. Real inference is a paid action outside the current authority. The README recording is the only real inference the plan needs.

## Release pair published (2026-10-06)

- Starmap PR #234 merged as `6870aec2`. PR #235 merged as `c83f3bd4`. PR #236 merged as `b68e341d9`. It promotes sequence 45 automatically.
- The Starmap release dispatch, run `37402744934`, published `v0.17.0`. The tag `6ff85f561d` points to commit `2b2944be7`. The module embeds the generation that channel sequence 44 promotes at commit `c55f31dd9`.
- Starport PR #424 merged as `05d70da7`. PR #426, the Release Gate limits, merged as `aebb9460`. PR #425, the public site, merged as `b649dbf5`.
- The annotated tag `v1.3.0`, object `37e6f897ad`, points to `b649dbf5`. Release run `37437368384` published 12 assets at 2026-10-06T09:12:50Z with one SLSA provenance attestation over 11 subjects.
- The docs archive manifest reports release `v1.3.0`, Starmap `v0.17.0`, content revision `0b7f6bb15611`, and 58 files.

## Domain consolidation (2026-10-05, Codex)

- Codex moved the zones `agentstation.ai`, `agentstation.dev`, and `agentstation.io` into the `AgentStation` account. The owner did the logins and the payments.
- The registrations of `.dev` and `.io` are now in Cloudflare Registrar. The ICANN email verifications for `.dev` and `.io` are open. The `.io` transfer has a 60-day lock.
- The owner ran the first `wrangler deploy` from the site worktree. The live version is `fb522680-b870-40e4-a91f-2d2db085ef7f`, a `dev` build.

## CSP23 acceptance run on the released pair (2026-10-06)

The lead ran `--task CSP23` with Starmap `b68e341d9` and Starport `v1.3.0`.

- PASS: `A29.embedded_offline_search`, `A29.recovery_without_auth`, `A29.no_dynamic_data_disclosure`, `A06.promoted_checkout`, `A06.old_pinned_bytes_unchanged`, `A06.starport_released_module_pin`.
- FAIL `A29.public_url_content_manifest`: the live site is the `dev` build. The Cloudflare Web Analytics beacon is also injected. The owner deploys the `v1.3.0` build and turns off the automatic beacon setup.
- FAIL `A06.new_released_module`: the check compared the release with the current channel head. Sequence 45 moved the head after the release. The check is time-windowed, not the release.
- UNVERIFIED `A29.hosting_rollback`: the rollback record does not exist until the deploys run.
- UNVERIFIED `A35.released_installer_paths`: the registry has no behavior check for it. CSP24 owns the installers.

## Fix pull requests (2026-10-06)

- Starmap PR #237 compares the released module with the `catalog/v1` channel at the tag commit time. It also compares the tag tree with the module. The real run reports `A06.new_released_module` PASS at sequence 44, commit `c55f31dd9`, tag commit `2b2944be7`. Four A06 subcases PASS.
- Starport PR #428 gives the recovery shards a thirty-minute allowance. The windows-2025 shard 1 takes about nineteen minutes. The twenty-minute allowance cut it in runs `37406845328` and `37437505736`.
- Starport PR #427 redoes the splash page as a sourced landing page. The owner asked for a redesign with nimbusdocs.com as the exemplar. The lead reviewed the screenshots and the sourced-copy tests. The autoreview gate is clean.
- Observed once on PR #425: `TestInitializeConcurrentSingleWinner` in `internal/setup` on macos-15 reported a writer-conflict error under load. The rerun passed. The lead made no change.

## Owner actions that remain open (2026-10-06)

1. Disable the Cloudflare Web Analytics automatic setup for `starport.agentstation.ai`.
2. Deploy the `v1.3.0` build from `/private/tmp/starport-v130-20261006/website` with `wrangler deploy`, and record the version id.
3. Run the rollback exercise: at least three deploys or rollbacks with version ids.
4. Complete the ICANN email verifications for `agentstation.dev` and `agentstation.io`.
5. Optional: create the `starport-site` environment with the Cloudflare secrets so the Site workflow can deploy.
6. Dispatch the Starport `Native Release Verification` workflow for `v1.3.0`. The permission classifier denied the dispatch to the lead on 2026-10-06. Command: `gh workflow run native-release.yaml -R agentstation/starport --ref main -f tag=v1.3.0`. An allow rule for `gh workflow run` is the alternative.

## Merges and the CSP23 rerun (2026-10-06, later)

- Starmap PR #237 merged as `9634fa434`. Starport PR #428 merged as `0af089d3` with 47 of 47 checks. The lead rebased PR #427 onto `0af089d3`, and it waits for CI.
- The lead reran `--task CSP23` with Starmap `9634fa434` and Starport `v1.3.0`: 7 PASS, 1 FAIL, 2 UNVERIFIED. `A06.new_released_module` now passes. The FAIL and the UNVERIFIED subcases wait on the owner actions above.
- CSP24 scope: the verifier has the `reviewed_first_use` and `reviewed_demo` check kinds. CSP24 registers `A35.released_installer_paths` and the 14 A36 through A39 subcases with those kinds against a `csp24/` proof set and a `docs/assets/first-use-v1.3.0/` recording. The six-platform archive evidence comes from the native workflow in item 6.
