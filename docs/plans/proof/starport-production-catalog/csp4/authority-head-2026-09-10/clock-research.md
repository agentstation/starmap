# Permission clock research

This research selects no clock adapter and changes no permission contract.
CSP4 still requires the five-minute validity bound, at most 30 seconds of uncertainty, and refusal when clock validity is unknown.

Go compares monotonic readings when both `time.Time` values contain them.
Serialization removes those readings. Some operating systems stop their monotonic clock during sleep.
A process-local elapsed-time deadline alone therefore does not establish retained validity after restart or suspend.
The Go 1.25.12 toolchain source contains these same conditions. [Go time documentation](https://pkg.go.dev/time#hdr-Monotonic_Clocks)

NTS binds an authenticated response to an outstanding request through a random identifier of at least 32 bytes.
A client rejects a missing match or failed authentication. [RFC 8915, sections 5.3 and 5.7](https://www.rfc-editor.org/rfc/rfc8915.html#section-5.3)
NTP also distinguishes measured offset, network delay, and accumulated uncertainty. [RFC 5905](https://www.rfc-editor.org/rfc/rfc5905.html#section-8)

A request-bound receipt is a possible design input, not an implemented guarantee.
Its design must cover replay, delay, suspend, process restart, and stale authority replicas before it can qualify permission freshness.
A cached catalog head cannot prove these conditions.
Do not replace the current unknown-clock refusal with an unchecked local timestamp or elapsed-time assumption.
