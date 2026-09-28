# Shared batch aggregate recovery

Consumer `07578301cd01ce94ce41ffd6797a8d21a7025ddb` adds a Valkey and versioned MinIO process-loss test.
The fixture retains complete line results before aggregate publication. It has no inference runner.

A child process stops after either object publication or batch metadata publication. The parent terminates that child at the recorded boundary.
Two fresh processes then recover through the same repositories and object store.
The test checks stable output identity, both bodies, checkpoint retirement, two visible files, and exactly 22 stored bytes.
Each case removes its own Valkey namespace and versioned object bucket.

The first shared run exposed a race in the older restart test. It began recovery before the original worker completed failed-publication cleanup.
The test now waits for the production `Close` boundary. Output, dispatch, and accounting assertions remain unchanged.
The repaired test passed ten repetitions across memory, Badger, and Valkey.

Final aggregate race checks pass 34 results without skips. The repeated test passes forty results.
The shared process-loss test passes three pure-Go results. Counts include parent tests and subtests.
Vet and the new test's prose check pass. The [record](verification.json) retains commands and failed intermediate evidence.

This test qualifies KV and blob publication recovery. It does not qualify SQL identity, provider dispatch, or all budget-failover cases.
The task gate still reports 23 unregistered checks. Its [output](task-gate.json) establishes the current qualification gap.
CSP12.2 remains in progress. The owner decision about untouched batch lines remains pending.
