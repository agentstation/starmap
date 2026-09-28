# Restricted bundle preparation

`PrepareBundle` validates the complete backup and retained references before importing target records.
One operation identity binds the user operation and complete manifest digest across all stores.
SQL retains closed gates, disabled initialization, and its import barrier.
KV preserves original expirations. Blob storage preserves retirement markers.
All target writers must remain fenced.

Retries after lost acknowledgements reuse the same component claims.
They do not repeat SQL restrictions or grant admission.
Filesystem blob retries verify the exact receipt and every retained object.
The original one-shot filesystem API continues to refuse existing destinations.

Selected files retain portable artifact names in an inactive staging directory.
The coordinator never selects active destinations from source absolute paths.
The completion receipt and selected files publish together after all component imports succeed.
Existing staged files require exact contents and inventory. Unknown files, directories, and symlinks cause refusal.

Native path identities prevent aliases from placing the stage inside the backup.
The coordinator checks SQL restrictions again before final file publication.

The complete recovery and blob race suite passes 259 results.
Focused SQL, application, recovery, and blob checks pass 71 race results.
Final path checks pass 24 race results. Final pure-Go checks pass 25 results.
These cohorts have no failures or skips. Counts include parent and subtest outcomes.

Preparation does not prove later permission withdrawals or acknowledged spending.
Independent reconciliation, canonical file placement, operator commands, and activation remain required.
A retained preparation receipt cannot approve itself as independent recovery history.
Fifteen CSP13 acceptance subcases remain UNVERIFIED.
