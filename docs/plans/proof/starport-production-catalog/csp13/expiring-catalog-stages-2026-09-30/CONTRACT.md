# Catalog expiry retirement

The catalog compiler derives retirement stages from original captured fleet
records. Only lease and maintenance controls can use the expiry owner. Each
stage retains exact original keys, values, value presence, and absolute expiry.
The ordered topology seal and the small backup record bind the stage kind.

`TopologyExpiringTarget.ApplyExpiringCatalogTopology` receives the topology SHA,
original ordinal, and detached original records. A target without that owner
refuses the stage. Ordinary persistent CAS never receives these records.

Each expiry stage has at most 128 records and 4 MiB of raw keys, values, and
expiry fields. The native import owner checks the original claim, cursor,
incarnation, value, and expiry. Absence requires the native owner's expiry
proof. Retirement deletes records. It does not renew or recreate them.

A separate persistent marker stage follows expiry retirement. The recovery lane
must enforce the contiguous native receipt chain before that marker. Final
selection requires all retired controls to remain absent. Fleet restore uses
an absent lease preimage only after its declared retirement stage.

The compiler preserves the source archive, including original lease and
maintenance facts. Archived controls remain historical. Recovery creates no
refresh grant or lease. Admission remains closed after catalog selection.

The persistent Badger fixture imports original captured records unchanged.
It proves cross-topology and same-backend fleet selection, expired controls,
exact retries, lost replies, changed values, stale cursors, wrong claims, and
future unexplained absence. An old retry leaves later independent state intact.
The import barrier remains closed and the final native position is nonzero.

The compiler fixture does not qualify the SQL recovery guard, application
activation, or durable local preparation journals. The native storage owner
has separate Badger and Valkey evidence. The application owner qualifies the
complete shared SQL and KV preparation lane. Existing capacity bounds remain.
These small fixtures do not qualify 2 GiB capacity or recovery objectives.
