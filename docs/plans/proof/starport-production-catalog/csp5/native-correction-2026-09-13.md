# CSP5 native correction status

CSP5 remains incomplete. [PR #154](https://github.com/agentstation/starmap/pull/154) remains open at `d15c6ef4cbb21827007f6400a8996bed09967535`.
The [verification record](native-correction-2026-09-13/verification.json) preserves the native evidence and current CI snapshot.

Both Linux jobs and both macOS jobs pass on this correction.
Both Windows jobs fail their native runtime, configuration, and paths step.
The full workflow remains active at capture time.

Each Windows artifact contains 2,689 passing test events, 71 failing test events, and five skipped test events.
Each Windows job reports fifteen passing packages and three failing packages. These counts include parent tests and nested subcases.
Several failures share the same locked-file read error. The count does not represent 71 independent defects.

The new empty-record and migration snapshot tests pass. Filesystem retention serialization also passes.
Other migration paths still attempt reads against locked files.
Replacement checks also encounter Windows handle restrictions. Two private-record cases report missing staging evidence after identity changes.
The exact root causes and corrected native behavior remain unverified.

## Completion work

Repair the affected file-access paths and recovery behavior.
Run focused native tests before the next full qualification cycle.
Merge PR #154 after its required review and checks pass.
Complete shared catalog cleanup with protection for publishers, readers, and retained pins.

The shared-storage decision remains unresolved.
The existing enterprise recipe uses shared KV coordination with immutable objects in object storage.
The pending choice concerns mandatory shared coordination versus an additional S3-only coordination and recovery protocol.
No answer or scope reduction follows from this status review.

## Delivery delay

Earlier qualification records show aggregate suite timeouts, local disk exhaustion, and a failed review process without a verdict.
The reviewed correction then failed native Windows qualification.
These failures required further work. The long verification cycle delayed feedback about Windows behavior.

CSP5 now returns to active repair. CSP6 retains its five uncommitted receipt files and failed allocation regression.
The regression allocated 51,726,448 bytes against a 16,777,216-byte budget. Its result remains evidence, not acceptance credit.
