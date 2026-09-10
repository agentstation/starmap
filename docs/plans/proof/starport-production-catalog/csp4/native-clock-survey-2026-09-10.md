# Native permission clock survey

This survey selects no production adapter and gives no CSP4 acceptance credit.
The five-minute permission bound, 30-second maximum uncertainty, and refusal under unknown clock validity remain unchanged.

Linux exposes clock state, maximum error, and estimated error through `adjtimex`.
A zero mode reads without changing clock parameters. `TIME_ERROR` identifies an unsynchronized or unreliable clock state.
The error fields use microseconds. [Linux clock interface](https://man7.org/linux/man-pages/man2/adjtimex.2.html)

Apple's syscall table exposes `ntp_gettime`.
The installed macOS SDK describes its return value.
It contains a timespec, two error bounds, TAI offset, and time state.

The read-only probe returned these values on this host.

| Field | Observed value |
| --- | --- |
| Platform | macOS 15.7.2 arm64 |
| Structure size | 48 bytes |
| Return status | 0 |
| Maximum error | 165,685 microseconds |
| Estimated error | 500,000 microseconds |

The [observation](darwin-clock-observation-2026-09-10.json) records the exact platform and SDK hash.
The native API appears in the [Apple syscall table](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/syscalls.master).

The Windows time service exposes `W32TimeQueryStatus` over RPC.
Its result includes leap state, stratum, root delay, root dispersion, phase offset, and last successful synchronization information.
A status query does not itself select a trustworthy upstream time source. [Microsoft time-service status contract](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-w32t/7e80a465-f5f4-4c3c-87ef-12f76e45f8d1)

The local macOS observation proves that the read interface works on this host.
It does not prove persistent accuracy, sleep behavior, clock-step handling, supported Go bindings, or Windows and Linux behavior.
No clock or time-service setting changed.

A candidate adapter can cache a qualified UTC sample and age its uncertainty against a clock that includes system sleep.
Admission would read that cached sample and a cheap elapsed-time counter, without a time-service request.
This approach still needs a measured error-growth bound, a finite validity interval, native bindings, and failure tests before selection.
The implementation must reject stale evidence and unknown validity. It must not treat an ordinary wall-clock timestamp as qualification.
