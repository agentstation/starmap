# D41 retained baseline implementation

The owner approved the refined third upgrade option.
An established fleet now replays its complete retained baseline across binary upgrades.
Configured source updates continue. Incompatible acquisition policy still refuses refresh ownership.

CSP16 owns coordinated policy apply. CSP16.1 owns explicit baseline promotion.
The transition contract retains all seventeen required conditions.

Producer commit: `41ee1ef29276f6ab4f76a39cf11e1a1ec413e964`.
Consumer commit: `0921b39ecfcfd384ca82e9ac5a2c587b0670d231`.

## Results

The producer fleet suite passes 94 named race results.
The paired native suite passes 46 with real Valkey and PostgreSQL.
Both have zero failures and skips. The verifier regression suite passes 99 tests.
The complete task gate first passed ten subcases and left one UNVERIFIED because its runner omitted the replacement backend.
The corrected task runner uses a separate replacement Valkey service.

Failure evidence covers baseline mismatch, startup replacement, and a lost native catalog head.
The compressed recovery record preserves the complete baseline within both input limits.
One publication measures 40,342,576 bytes, including 3,705,079 recovery bytes.
The 32-publication protected window fits within the 2 GiB bound.

## Evidence limits

`verification.json` records source hashes, work commits, counts, and raw output.
Native runner records include service cleanup results. All services from completed runs stopped successfully.
The local pair uses a temporary Go workspace. It does not prove the published module pair.

Earlier uncompressed results remain historical evidence. They do not qualify the current compressed format.

Complete branch review, module publication, final native CI, and both merges remain open.
CSP11 remains in progress. No policy-apply or explicit promotion operation ships in this change.
