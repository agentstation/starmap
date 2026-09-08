# Planning evidence

Date: 2026-09-04. Status: proposed. Implementation tasks remain unstarted.

## Scope and ownership

The user requested an implementation plan from the existing PRD, engineering
specification, and repository findings. The user also requested README installation instructions and a demo of the first successful request.

The [plan](../../starport-production-catalog-plan.html) has one status ledger in
Starmap. Starport `docs/TASKS.md` names that canonical owner. Both indexes identify
the work as proposed.

The PRD now contains P01–P28. The specification now contains A01–A39.
The [acceptance map](acceptance-map.json) assigns each case to one primary task.
The [README brief](readme-demo-brief.md) defines the recording sequence and checks.

## Source baseline

| Repository | Inspected main revision | Planning branch |
|---|---|---|
| Starmap | `4780dfea9f7e002ea6eaaa82aa8f52ed0d4cc225` | `codex/catalog-requirements` |
| Starport | `042eb97851496ecdd1ef20bfa44fe3d84b86b2d8` | `codex/catalog-lifecycle` |

The remote main references still matched these revisions during plan creation.
Starport pins Starmap `v0.16.5` at its inspected revision.
Existing source checkouts and the separate Starport review worktree remain
outside this planning change.

The user confirmed internal authority, release-time embedding, and opt-in
Starmap-prefixed inference credentials. The user also confirmed embedded and
public docs, Valkey with PostgreSQL, and single-region support with disaster
recovery.

Library side effects, merge rules, operator control scope, and local configuration
ownership remain recorded proposals. Activation must also settle path migration,
retention expiry, workload, recovery objectives, and version qualification.
The plan preserves these distinctions.

## README and GIF baseline

The inspected Starport README leads with dated catalog counts and includes a
console tour. Installation and request examples appear later.
The existing GIF is `docs/assets/2026-08-29_starport-console.gif`.
Its Git blob is `7dc4e2a976203344318a2fdba107fb6d43006f29`.
That blob matches the original checkout and the inspected main revision.

The [raw media metadata](readme-baseline.json) records these values:

| Property | Observed value |
|---|---|
| Dimensions | 1440 × 777 px |
| Duration | 17.1 seconds |
| Frames | 11 |
| Size | 2,208,444 bytes |

Command, from the Starport worktree:

```sh
ffprobe -v error -show_entries format=duration,size:stream=width,height,nb_frames -of json docs/assets/2026-08-29_starport-console.gif
```

Visual review of the first, middle, and last frames found catalog, provider, and
incident views. Those sampled frames do not show installation or a successful
inference request. This evidence covers the sampled frames of the existing demo.

The README quickstart verifier checks specific prose and headings. CSP20 must
preserve its behavioral intent and add installation and request evidence.

## Validation boundary

This change creates planning artifacts and updates design scope. It does not
change Go source, console code, workflows, the product README, or media assets.
It does not publish a release or run provider inference.

Earlier repository tests and UX measurements remain in the
[findings](../../../design/catalog-lifecycle/REPOSITORY_FINDINGS.md).
They are historical evidence for the inspected revisions.
CSP0 must create the 39-case campaign verifier.
No campaign pass count exists yet.

## Planning checks

`make technical-writing-check` passed in Starmap: 77 glossary terms,
no missing candidate terms, and 746 files with zero diagnostics.
The Starport `docs/TASKS.md` writing check passed with zero diagnostics.
`git diff --check` passed in both planning worktrees.

The [structure report](plan-validation.json) records 26 tasks, 39 acceptance
cases, and 35 valid HTML links. It checks status agreement, dependencies,
required fields, unique identifiers, and local resources.
Eleven local Markdown links across the three planning documents also resolved.

The HTML contains 471 lines. Its ledger starts at line 42.
Every task remains `todo`. The browser reports zero of 26 terminal tasks.

Chrome checks used 1440 × 1050 px in the light theme and 390 × 844 px in the
dark theme. Body text measured 16 px. Page width matched each viewport.
The narrow ledger stayed inside a horizontal scroll container.
Visual inspection covered the desktop overview, narrow overview, and narrow
ledger. These checks cover the plan artifact, not native product qualification.
