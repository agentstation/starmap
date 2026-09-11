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


## Native binding measurements

The [cache proof](clock-cache-2026-09-10/verification.json) retains the probe source and two explicitly pinned, read-only macOS runs.
Each run uses `CGO_ENABLED=0`. No clock or time-service setting changed.

| Toolchain | Typed `purego` call | Direct `purego` call | `unix.ClockGettime` |
| --- | --- | --- | --- |
| Go 1.26.6 | 177 ns, 2 allocations | 67 ns, 0 allocations | 36 ns, 0 allocations |
| Go 1.25.12 | 264 ns, 2 allocations | 80 ns, 0 allocations | 44 ns, 0 allocations |

These isolated measurements compare bindings on one macOS arm64 host. They do not measure Starport request overhead.
The existing `unix.ClockGettime` binding is the current candidate for macOS elapsed-time reads.
Production selection still requires native qualification and supported error bounds.

Apple's implementation maps `CLOCK_MONOTONIC_RAW` to `mach_continuous_time`.
The kernel also grows maximum time error with elapsed seconds and reports unsynchronized or faulty state.
[Apple clock implementation](https://github.com/apple-oss-distributions/Libc/blob/71bbe350ab79eef58113991d817ccc6165061a64/gen/clock_gettime.c),
[Apple NTP implementation](https://github.com/apple-oss-distributions/xnu/blob/f6217f891ac0bb64f3d375211650a4c1ff8ca1ea/bsd/kern/kern_ntptime.c)

The Windows RPC status contract remains a candidate observation source.
The current design selects no DLL binding or RPC adapter. Complete Windows clock qualification remains open.
The cross-platform cache does not resolve that native requirement.


## Windows elapsed-counter candidates

Microsoft documents `GetTickCount64` as elapsed milliseconds since startup.
Its typical resolution is 10–16 milliseconds. That typical range is not a qualified maximum error bound for every supported host.
The [counter contract](https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/nf-sysinfoapi-gettickcount64) defines the return value and platform requirements.
The [interrupt-time contract](https://learn.microsoft.com/en-us/windows/win32/sysinfo/interrupt-time) recommends this counter for elapsed time that includes sleep and hibernation.

`QueryInterruptTimePrecise` reads hardware for finer precision and returns its result through an output pointer.
Its [API contract](https://learn.microsoft.com/en-us/windows/win32/api/realtimeapiset/nf-realtimeapiset-queryinterrupttimeprecise) documents no error result.
Neither interface establishes UTC correctness. The native profile still needs a separate time-service observation and a supported error-growth bound.

`W32TimeQueryStatus` is a documented RPC operation.
Its [protocol contract](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-w32t/7e80a465-f5f4-4c3c-87ef-12f76e45f8d1) returns synchronization state, phase offset, delay, dispersion, and observation timing.
That contract alone does not establish an exported DLL function with the same name and signature.
A production adapter must qualify its actual transport and native behavior before selection.

This September 11 review changes no runtime source or platform support claim.
