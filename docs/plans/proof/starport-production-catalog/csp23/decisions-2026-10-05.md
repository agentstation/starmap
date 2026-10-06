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

## Pending owner decisions

1. Hosting target for the public site. The exemplar runs on Vercel. The owner named the Cloudflare account. Vercel needs a Vercel project and a CNAME on Cloudflare. Cloudflare Workers needs the OpenNext adapter and a Workers route on the zone. The lead proposes Vercel to match the exemplar, with Cloudflare DNS only.
2. Repository location for the site. The lead proposes a `website/` directory in `agentstation/starport` so that the site reads the release documentation from the same tree.
3. Real provider inference for CSP24. Real inference is a paid action outside the current authority. The README recording is the only real inference the plan needs.
