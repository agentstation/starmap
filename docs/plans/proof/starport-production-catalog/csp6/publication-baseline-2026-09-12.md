# CSP6 publication baseline

The accepted public channel and the default branch's embedded input contain different catalog digests.
This read-only inspection supplies CSP6 fail-before evidence while CSP5 native qualification runs.
The [verification record](publication-baseline-2026-09-12/verification.json) binds the source files, channel bytes, producer result, and attestation output.

The inspection captured the channel at 22:10:20 UTC on September 12.
It compares that channel against main commit `f9951ee6a3644986a3bb7a3b302649cce790068e`. A fresh remote check confirms the same main commit.
[Scheduled run 34710936558](https://github.com/agentstation/starmap/actions/runs/34710936558) completed successfully from that commit.
Its signed channel is at sequence 26, with a channel update time of 18:28:25 UTC.

| Input | Semantic catalog digest |
| --- | --- |
| Signed public channel | `sha256:a172a9c1c2d23bf6ac5fc0a51f1c49ba6f9056f901a1f457ea70f41175b7eaa6` |
| Default-branch embedded manifest | `sha256:0bbbff4c5fd3c8b376725a808b3132a67dcee99bdc8ed45d7f64af5d6581cf05` |

The embedded manifest records September 10 at 06:38:28 UTC as its generation time.
The channel attestation passes the expected repository, workflow, and GitHub-hosted runner checks.
Its verified subject digest matches the captured channel bytes. Its source commit and invocation match the successful producer run.

The inspected workflow publishes immutable catalog assets and advances the channel.
It contains no checked bot-PR promotion into the default branch. The generator changes only its checkout and local catalog store.

This evidence does not qualify the full A05 contract. It does not independently verify the public archive's bytes.
Required-source admission, immutable receipts, failure recovery, and the controlled bot-PR test remain unverified.
CSP6 remains planned. This inspection changed no publisher, release, channel, bot, or remote branch.
