# Shared native owner process fencing

The race and pure-Go tests pass against actual PostgreSQL and two independent Valkey processes.
Each mode reports three passing results, with no failures or skips.
The race package completes in 5.637 seconds.
The pure-Go package completes in 1.080 seconds.
Both modes use Go 1.27.1 and the published e0845d601fb5 Starmap module.

The test isolates each deployment in its own Valkey namespace and PostgreSQL schema.
A child process opens actual SQL and KV connections and retains the exact approved native budget owner.
It completes a guarded mutation before the parent closes SQL approval.
It then refuses another mutation while the former primary remains reachable.
The refused mutation appears on neither backend.

The parent kills the child through the operating system and waits for its exit.
SQL approval remains closed through that completed process fence.
The parent then approves the exact selected backend incarnation in the next epoch.
A retained parent-side native authority refuses that new epoch and causes no mutation.
The killed child makes no post-reapproval attempt.

A new owner process on the former primary refuses the different approved incarnation.
A new owner process on the selected primary reads the exact new approval.
It completes a guarded native mutation and exits successfully.
The same-backend case verifies the corresponding epoch change without backend replacement.

Vet, pinned lint, and prose checks pass.
The repository lint configuration excludes test files.
Vet and native behavioral tests check the changed source.
The original JSON-duration fixture failure remains in the evidence directory.
No product defect required a source change.

This evidence covers native budget authority and an actual local child-process fence.
It does not prove that SQL closure fences generic gateway or administrator operations.
It does not qualify full gateway startup, inference, deployment network controls, unreachable hosts, or complete disaster recovery.
Deployment RPO/RTO and maximum capacity require separate evidence.
