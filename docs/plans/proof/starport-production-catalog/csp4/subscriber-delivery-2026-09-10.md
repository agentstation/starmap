# CSP4 subscriber delivery

This delivery owns permission transport, retained subscriber enforcement, and receipt relay.
The source is `44a400a481748c5e7ae1268cecb63b260a91bdab` above actual Starmap main `a87262e3`.
The branch changes 91 files, with 4,596 insertions and 201 deletions.
CSP4 remains in progress until the remaining Starmap and Starport deliveries merge and all eight task checks pass.

## Review scope

The protocol binds authority, policy, publication sequence, payload identity, permission revision, and finite receipt validity.
Ordinary generation manifests retain version 2. Authority manifests use version 3.
Strict permission parsing and authenticated transport precede acceptance. Receipt reads remain independent of payload transfers.

The runtime retains the highest authenticated requirement separately from the confirmed finite receipt.
Known withdrawals block new attempts before payload compatibility checks or transfer.
An incompatible catalog can preserve diagnostics while admission remains closed.
Shutdown joins active readers before sealing permission state. A crash requires fresh receipt evidence.

The root publication guard reserves mutations for the owning runtime.
The serving store retains the exact authority generation. Cached admission reads do not consult storage.
The relay preserves the confirmed upstream receipt and its original expiry.
The HTTP route applies configured API authentication and returns a generic unavailable response without private error details.

## Manual audit

The review inspected publication ownership, receipt transitions, source callback lifetime, transport order, server authentication, and retained checkpoint handling.
The focused regression covers a trusted withdrawal followed by a stalled payload transfer.
The shutdown test refuses callbacks after close. Protocol tests cover concurrent binding, invalid trust, incompatible schemas, and historical reads.

The version-2 JSON schema fixture remains a test fixture with a version-2 identifier.
Only its required-field test reads it. It is not the authority wire parser or the generated permission endpoint schema.
No fixture change is necessary for this delivery. A published authority schema remains part of origin delivery review.

The maintained settings document states that no standalone clock adapter or complete internal-server recipe exists yet.
The library host must supply cached clock evidence for the same clock as runtime time.
A missing clock callback blocks admission. This delivery does not qualify a standalone production deployment.

## Verification and publication

Both toolchains pass 40 focused race events. The complete remote protocol suite passes 60 events.
Static checks and all six external consumer checks pass.
The complete runtime, remote, and artifact race suite passes 877 events with no failures or skips.

The first repository verification failed two YAML authority fixtures. Their source context now selects the required paired authority settings.
All 28 YAML test events pass on both toolchains.

Complete repository verification passes at `44a400a4`.
Both test modes pass 81 packages. Container smoke, all 15 coverage thresholds, static checks, and isolated CLI checks pass.
The catalog accessor records 8.167–8.523 ns/op, zero bytes, and zero allocations across three runs.
The documentation generator rejects the build-concurrency flag despite a successful wrapper exit. Its clean-environment retry passes.

Required Sol and Opus review passes with zero accepted or actionable findings across two chunks.
Starmap PR #146 is open at the reviewed source. Native CI and merge remain pending.
The current plan resume state owns session IDs and output paths.

## Next delivery constraints

Origin receipt issuance must establish current durable authority independently of the server's cached catalog.
A stale local snapshot cannot renew permission after another publisher commits a withdrawal.
Head observation and receipt timing must preserve the selected maximum permission-staleness interval.
Clock evidence must account for the issuer and consumer bounds, process restart, suspension, and clock correction.

Shared-store followers must recover from durable state without relying on notifications or another process's private layer files.
Authority transitions must refuse retained permission from the old authority.
The remaining Starmap delivery must complete these contracts. A separate Starport PR must qualify the four mapped consumer checks.
All eight CSP4 acceptance subcases and the implementation merges remain necessary for task completion.
