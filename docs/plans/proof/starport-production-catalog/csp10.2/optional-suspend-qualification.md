# Optional physical suspend qualification

Owner: CSP22 platform qualification. Status: UNVERIFIED. Decision: D40, September 26, 2026.

This evidence does not block CSP10.2 or a product release. Native clock checks and deterministic permission tests remain mandatory.
The gateway authorization lifetime remains at most 60 seconds. The normal withdrawal propagation target remains 2 seconds.
An observed withdrawal takes effect immediately. Starmap authority receipts retain their separate expiry contract.

Run this check when controlled hardware and explicit permission to suspend it are available.
A successful check qualifies only the tested operating system, architecture, and environment.

1. Record the binary commit, operating system, architecture, and clock adapter.
2. Get gateway authorization and prevent its refresh.
3. Suspend the operating system for more than 60 seconds.
4. Resume and submit a new request before a successful refresh.
5. Verify that the request cannot use the expired authorization.
6. Capture operating-system sleep and wake evidence with application results.
7. Restore connectivity and verify recovery with new authorization.

Keep raw evidence and exact results. Do not count a deterministic clock advance as physical suspend evidence.
Do not disable expiry or extend cached permission to pass this check.

The required automated evidence uses these platform contracts:

- Linux: [CLOCK_BOOTTIME includes suspended time](https://man7.org/linux/man-pages/man3/clock_gettime.3.html).
- macOS: [mach_continuous_time](https://developer.apple.com/documentation/kernel/1646199-mach_continuous_time) supplies the continuous clock.
- Windows: native tests compare the runtime counter against [interrupt time](https://learn.microsoft.com/en-us/windows/win32/api/realtimeapiset/nf-realtimeapiset-queryinterrupttime).

These contracts and adapter tests do not constitute a physical suspend result.
