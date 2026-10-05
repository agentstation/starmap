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

## Pending owner decisions

1. Public documentation. CSP23 names versioned public docs. GitHub Pages stays disabled on `agentstation/starport`. Enabling it is new authority. The lead asked the owner on 2026-10-05 with these consequences:
   - Enable: the owner sets the Pages source to GitHub Actions and creates the `github-pages` environment with a required reviewer. After the `v1.3.0` release, the lead starts `docs-pages.yaml` with tag `v1.3.0`. The owner approves the deployment. The site serves `https://agentstation.github.io/starport/v1.3.0/` and `/latest/`. The lead registers `A29.public_url_content_manifest` against that URL. `A29.hosting_rollback` needs an earlier release with a docs archive. Only `v1.3.0` has one, so that subcase stays a documented limit until the next release.
   - Keep off: `A29.public_url_content_manifest` and `A29.hosting_rollback` stay UNVERIFIED. A29 fails its primary case in the final gate. The final 50-case result records A29 as UNVERIFIED, and no complete production claim follows.
2. Real provider inference for CSP24. Real inference is a paid action outside the current authority. The README recording is the only real inference the plan needs.
