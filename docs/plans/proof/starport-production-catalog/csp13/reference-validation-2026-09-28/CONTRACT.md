# Backup reference validation

`InspectBundleReferences` first verifies artifact integrity and the selected-key challenge.
It then reads verified private copies through credential, file, and job owners.
Every retained provider credential must decrypt, including shared credentials without grants.
Historical-key mismatch causes refusal without exposing plaintext.
No live store or provider opens during verification.

Ready, unexpired files require their published bytes at capture time.
Pending, deleting, and expired records retain their states.
Job assets and native receipts require matching lengths and available digests.
An unconfirmed provider submission remains unconfirmed even when its receipt survives.
Verification never dispatches work.

Every batch execution claim needs its retained parent, account identity, and line number.
A batch must retain every line that its stored claim count includes.
Missing claims cause refusal because missing history cannot prove that work never started.
Explicitly deleted batch files remain diagnostic references.
A result digest can precede the output file commit.
That interrupted state remains valid when the result is not ready.

The command reports credential counts, unfinished lines, unconfirmed submissions, and missing batch file references.
The scratch directory must be private and already exist.
It defaults to the backup parent. Verification removes only its owned copies.
Large payload checks stream bytes.

The focused cohorts pass 47 race results and 47 pure-Go results.
The five full owner suites pass 939 race results with real storage fixtures.
All three cohorts have no failures or skips.
Lint, affected-package vet, six dependency checks, and Linux and Windows cross-compilation pass.
Cross-compilation does not qualify native execution.

SQL identity and budget references, independent later history, and full restore remain required.
A verified bundle cannot approve admission or authorize a retry.
No additional CSP13 acceptance subcase is complete.
