# Native test deadline repair

Two functional tests hit production deadlines during native CI.
Starport PRs #388 and #389 failed concurrent policy acceptance on Windows.
PR #390 failed shared credential resolution on macOS.

The tests now use controlled time while retaining their behavioral assertions.
Policy acceptance still uses the native filesystem and real publication locks.
A separate test holds the native lock until the five-second production deadline.
It verifies refusal, unchanged policy, and successful retry after lock release.
Production deadlines and implementation remain unchanged.

The proof records repeated race tests, complete owner-package tests, pure-Go tests, and exact source commits.
One Valkey test skipped in each initial complete package because `TEST_VALKEY_URL` was absent.

A separate run used the CI-pinned Valkey 7.2.14 image and passed all three test results without skips.
The test runner removed the temporary container. The [Valkey proof](valkey-verification.json) records both commands and cleanup.

The Windows cross-build passes. Native tests on these changes remain UNVERIFIED.
The repairs require review and publication before they can unblock the dependency PRs.
