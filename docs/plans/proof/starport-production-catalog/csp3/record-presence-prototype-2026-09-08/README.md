# Optional record presence prototype

This prototype remains outside production code. It preserves explicit unknown states for fourteen optional Model records during JSON and YAML round trips.

The original code passes 57 test results and fails 29. The prototype passes 101 test results on each supported Go toolchain. The development toolchain uses the race detector. The minimum toolchain uses pure Go.

Legacy JSON bytes match before and after the codec change. Copy and reused-destination tests pass. Reconciliation still fails all fourteen record cases, plus their parent test. The codec change alone does not satisfy CSP3.

The compressed files retain the exact overlay source and test output. `verification.json` records source hashes and result counts. The overlay maps refer to the original temporary directory. No task gains completion credit.

Next, preserve unknown states through source selection and original receipts. Verify nested presence scope before extending the public representation. Publication, restart, policy refusal, definitions, and OpenAPI schema remain unverified.
