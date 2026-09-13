# CSP6 exact promotion staging

Commit `0837ed09c` stages an existing verified release as exact embedded input.
The [verification record](promotion-stage-2026-09-13/verification.json) binds the six changed files, commands, and results.
Its parent `7326a1c54` applies the unchanged CSP5 Windows correction to the publication branch.

The new command accepts a fresh destination or an existing exact result.
It validates the archive, preserves generation identity and observation bytes, and derives the complete endpoint projection.
It verifies the complete staged tree before publishing the directory without replacement.
Changed output, unrelated existing input, and invalid assets cause refusal without replacement.

An unchanged retry preserves file identity, modification time, and report bytes.
An uncertain directory flush returns a publication error after preserving the visible catalog.
Retry verifies that catalog and confirms the parent-directory flush before success.

| Check | Result |
| --- | --- |
| Initial staging behavior | Fails before implementation because the command lacks the staging flag |
| Retry durability behavior | Fails before correction because retry omits the directory flush |
| Final Go 1.26.6 release-command race suite | 54 passing test events, zero failures or skips |
| Final Go 1.25.12 focused race suite | Seven passing test events, zero failures or skips |
| Restriction policy and package lint | Zero findings or issues |
| Maintained prose | 1,665 files, zero diagnostics |

The proof retains the initial lint and prose failures and their passing corrections.
It also retains both behavior failures and the corrected results.

The manual source review found no remaining defect in the six changed files.
Required pre-PR review and native CI remain open.

Publication admission, immutable run receipts, checked bot promotion, and merged-source confirmation remain required for CSP6.

This local command neither publishes a release nor advances the public channel.
No A05 completion credit applies.
