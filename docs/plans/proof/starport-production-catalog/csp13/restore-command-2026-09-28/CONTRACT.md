# Operator restore preparation

`starport backup prepare` imports a verified backup into isolated configured targets.
It verifies the complete source, encryption-key access, and deployment ID before target creation or connection.
A private source object retains those verified facts. Every importer still checks its component bytes.
The source configuration never selects active target paths.

The command requires an operation ID, independently retained manifest digest, inactive file directory, and external fencing reference.
The reference binds the component claims and preparation receipt. It does not fence a process or prove network isolation.
Retries require the same operation, manifest, reference, and stores.
The command starts neither the gateway nor catalog acquisition.

SQL setup initializes empty state or accepts the current schema.
It refuses unknown tables and populated incompatible schemas.
Empty interrupted setup resumes at completed migration boundaries. Pending MySQL intents retain their explicit reconciliation requirement.
KV import access preserves its barrier and disables application maintenance workers.

Local and shared application recipes preserve every import barrier.
Saved tokens and runtime files remain inactive. A command error does not prove rollback.
Independent later history, canonical file placement, and activation remain required.

The final package results contain 171 race passes and 67 pure-Go passes across five packages.
Four packages retain their passing results from the broader runs.
A corrected storage fixture passes four race results and four pure-Go results in focused reruns.
The selected final results have no failures or skips. Earlier failures remain in the evidence.
Counts include parent and subtest outcomes without double-counting reruns.

Build, lint, vet, six dependency checks, and twelve source writing checks pass.
Fifteen CSP13 acceptance subcases remain UNVERIFIED.

The final SQL guard refuses NULL metadata before schema changes.
Its regression reproduces the earlier unintended schema mutation.
All 22 schema preparation results pass under race detection and pure Go across SQLite, PostgreSQL, and MySQL.
