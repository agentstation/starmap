# CSP12.1 stream cache bounds

Starport `a53776dc66a32bc53d0511e79bd6397ed2aafe53` bounds optional stream retention while preserving caller delivery.
The [qualification record](stream-bounds.json) records 327 passing race-test events and no skips.
The final proxy and response-cache run includes concurrent close and nested payload cases.
Lint, vet, prose, and whitespace checks pass.

## Behavior

The buffer limits charged retention to 256 KiB and 1,024 events.
A limit breach releases accumulated input and prevents further caching for that stream.
The proxy still returns every upstream event. EOF, failure, and close release retained input.
Concurrent close and accumulation use the same lock. The lock does not cover the upstream read.

A preflight charge includes strings, structures, media bytes, tool arguments, and probability arrays.
The buffer copies accepted payloads and strings. Small substrings cannot retain larger provider buffers.
The charge includes allowances for slice growth and allocation rounding on supported 64-bit hosts.
It does not qualify actual process heap or concurrent-stream capacity.

## Evidence limits

The first control failed on a fixture comparison between nil and empty slices. It did not prove the target defect.
The corrected control compares the source stream's clone contract and fails on both unwanted cache writes.
A Go overlay supplies the original proxy source from `eee04f9` without changing the worktree.

A single local buffer benchmark reports 146.6 ns/op, 589 B/op, and 3 allocs/op.
It measures buffer insertion only. Full gateway stream latency remains unverified.
EOF reconstruction and encoding still run synchronously with bounded input.

Dedicated shared-cache configuration, model and extraction fills, capacity, and discovery contracts remain open.
Review, native CI, published dependencies, and merges remain required. CSP12.1 remains incomplete.
