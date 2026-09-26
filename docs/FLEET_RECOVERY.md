# Fleet recovery across binary upgrades

An established fleet retains its reconstruction baseline in shared recovery data.
A new binary uses that retained baseline when it replays acquisition inputs.
The packaged baseline remains available as a candidate for an explicit promotion.
A binary upgrade does not promote it.

The recovery record retains the complete baseline manifest and payload, upstream source, provider observations, manual history, removal rules, and pin evidence.
Replay validates the baseline and reconstructs the selected catalog before it permits acquisition.
An acquisition-policy mismatch preserves follower reads and refuses refresh ownership.
Those reads remain subject to the existing authority and permission rules.

Configured upstream updates continue under their existing settings.
Pins and internal authority still restrict publication.
Credential values do not enter the replay compatibility digest.
A changed credential value within the same declared identity and scope does not change that digest.

## Storage and validation

The private version 2 recovery format contains one gzip member with deterministic JSON.
Both compressed and decoded data have a 256 MiB limit.
The decoder rejects trailing bytes, additional gzip members, corrupt data, unknown fields, and unsupported versions.
Storage adapters retain the opaque compressed bytes and their digest.
They report stored recovery bytes separately from public catalog bytes.

The runtime keeps validated baseline bytes in memory to avoid repeated catalog encoding.
It can reuse a decoded baseline only when the retained manifest and payload match exactly.
A changed record receives full validation before replay.
These operations run during recovery and publication. They add no inference-request storage calls.

Version 1 recovery records omit the complete baseline and cannot support this upgrade contract.
They require explicit recovery. The runtime must not substitute the current packaged baseline.
This format change belongs to the unreleased fleet implementation.

## Remaining work

CSP16 owns coordinated acquisition-policy apply through shared configuration authority. CSP16.1 owns explicit baseline promotion.
Their implementation, native fencing, and retry qualification remain incomplete.
Until those operations ship, an operator must not clear shared storage to bypass a policy mismatch.
Shared configuration UI integration remains with the configuration tasks in the production catalog plan.
