# Local post-backup activity recovery evidence

The local disaster-recovery test passes with actual Badger and SQLite owners.
This evidence covers one observed post-backup interval.

The source contains an account, two gateway API keys, a SQL account grant, and an original zero-spend window.
The fixture captures an immutable complete backup before subsequent activity.
The source then settles 700 nanoUSD and retains a 200 nanoUSD uncertain reservation.
A controlled provider runner records one dispatch and returns a lost-response error.
The source removes one gateway API key and the SQL account grant.
The source owner closes its recovery boundary after those changes.

The test retains independent evidence outside the backup and derives typed history from actual source-owner records.
Expected KV and SQL preimages come from the original backup.
Incomplete interval evidence keeps both native imports restricted.
Complete history permits full coordinator activation after catalog selection and final authorization rotations.

Recovered accounting preserves both original attempt records, including their timestamps and valuation.
The budget refuses another 101 nanoUSD because only 100 nanoUSD remains.
The reservation worker retains the uncertain attempt.
Job refresh, cancellation, and restart sweep cause no additional provider operation.
An exact completed activation retry preserves the same decision and domain state.

The rebuilt application accepts the unaffected key.
It rejects the withdrawn key and the removed explicit-account grant.

The final race test passes in 65.309 seconds.
The pure-Go test passes in 26.523 seconds.
Each mode reports one passing top-level test, with no failures or skips.
Both use Go 1.27.1 and the published e0845d601fb5 Starmap module.
The final fixture omits duplicate source startup under the parent-approved test split.

Actual producer materialization still supplies the original retained catalog descriptors.
Normal source startup remains a separate mandatory test.

Historical build and fixture-access failures remain in this evidence directory.
The earlier passing race result remains as timing evidence before duplicate startup removal.
No product defect required a source change.

This test does not establish continuous external-history completeness.
It does not qualify shared-storage disaster recovery, old-primary process fencing, maximum capacity, or deployment RPO/RTO.
It does not execute Linux or Windows.
The test controls the provider runner and incurs no provider charge.
The test uses the application API rather than the operator CLI.
