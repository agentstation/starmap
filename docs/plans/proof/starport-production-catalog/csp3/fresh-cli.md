# CSP3 fresh CLI controls

The update command now accepts `--fresh`. The existing `--force` and `-f` flags select the same reset behavior.
A dry run asks no confirmation. An interactive apply asks once after the preview. The `--yes` flag skips that confirmation.
The command no longer states that reset deletes all existing model files.

The [regression capture](fresh-cli/regression-red.jsonl) proves that `--fresh` was unknown and that the old confirmation prevented a declined dry run from starting.
The reset-only summary also omitted the acquisition work. Its [regression](fresh-cli/summary-red.jsonl) fails before the correction.
Preview and completion now report the acquisition reset count even when no model values change.

The [focused race run](fresh-cli/focused-race.jsonl) passes 59 test events across the update command and sync result packages.
Tests cover flag aliases, untouched confirmation input during dry run, preview before apply, decline, auto-approval, and reset-only summaries.
The broader CLI race run passes 285 test events across 14 package suites.
The repository lint and generated-document checks pass. The [verification record](fresh-cli/verification.json) binds source inputs, the binary, and captured evidence.

The built binary passed two fixture operations:

- `update openai --source provider-api --fresh --dry-run` made one local `/models` request and left all 1,275 workspace files unchanged.
- `update openai --source provider-api --fresh -y` made one more local `/models` request and completed the reset without confirmation.

Both operations showed one acquisition reset. Both selected the embedded source.
The fixture supplied catalog metadata and used a placeholder credential. It made no paid inference request.
The capture runner reuses a copy of the earlier local developer workspace. It is not a standalone fixture installer.

This change corrects the CLI controls for the accepted reset contract. It does not implement source-scoped tombstones or complete deletion authority.
CSP3 remains in progress. Primary acceptance cases remain UNVERIFIED.
