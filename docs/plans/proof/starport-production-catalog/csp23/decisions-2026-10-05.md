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

## Pending owner decisions

1. Starmap release version. The lead recommends `v0.17.0`. The catalog schema moved from 10 to 19, and the publication protocol changed. A release candidate tag `v0.17.0-rc.1` avoids the Homebrew update but gives Starport a pre-release pin.
2. Starport release version. The lead recommends `v1.3.0`. Main adds the catalog-driven runtime, the console work, the storage recipes, and the inference origin approval.
3. Tag authority. Each tag push starts a release that the lead cannot undo. A failed release workflow leaves a tag that needs a new version number.
4. Public documentation. CSP23 names versioned public docs. GitHub Pages stays disabled. Enabling it is new authority.
5. Dedicated runner for CSP22.1. The four A50 subcases stay UNVERIFIED without it.
6. Real provider inference for CSP24. Real inference is a paid action outside the current authority.
