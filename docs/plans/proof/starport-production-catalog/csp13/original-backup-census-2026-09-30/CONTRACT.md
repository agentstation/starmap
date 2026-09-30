# Original backup catalog topology

`CompileRecoveryTopology` reads the verified original backup. It derives catalog
selection and complete reconstruction evidence from captured KV records and the
original file inventory. It accepts no caller-supplied capsule census.

`DeriveRecoveryTopologyDirection` checks the original captured catalog and its
closed boundary. The destination storage owner supplies its fleet selection.
The result names local restore, fleet restore, or a cross-topology transfer.
Compilation independently checks that direction against source ownership.

`Record` returns at most 64 KiB of metadata. It binds the original backup
manifest, file inventory, file census, boundary, domain request, topology, ordered
stages, and selection. The stage seal contains a count and a framed ordered SHA.
It does not grow with capsule bytes. `Digest` identifies this metadata record.
`TopologyDigest` separately identifies the native preparation lane.

`InspectRecoveryTopology` recompiles the verified original source and compares
the exact canonical record. It reads no current target. It does not select a
catalog, query providers, renew permission, or open admission. A low-level
compiler result cannot produce a backup record.

The file census binds every selected artifact and each original runtime file
observation. Portable original paths use lexical comparison. The reader does
not open those original absolute paths. It reads the verified bundle artifacts.
Ordinary descriptors and baselines retain the producer's 256 MiB bound.

Encoded retained envelopes use its existing 512 MiB bound. Metadata retains its
16 MiB bound. The source census retains the existing 2 GiB limit.

The producer decodes original local descriptors and retained envelopes. The
consumer does not decode private producer manifests or journals. Their exact
fingerprints remain inactive history. Staging and orphan files have explicit
inactive dispositions. These fingerprints grant no selection or permission.

Missing original reconstruction evidence refuses with a source-owner recapture
procedure. Several original references for one selected local generation refuse
as ambiguous. Target settings never substitute for missing original evidence.

Local restore preserves accepted and candidate pointers separately. Fleet
restore preserves the original fleet archive, creates publications under the
new destination identity, and retires original live records. Recovery origin
has no refresh grant. The fencing floor is one. Recovery creates no refresh lease.

Producer materialization selects accepted catalog inputs for local targets and
candidate inputs for fleet targets. Both selected generations still require
semantic replay under the selected target settings.

# Evidence limits

The actual backup fixture starts a persistent source through its normal runtime
owner. It captures native Badger, SQLite, filesystem blobs, and the canonical
file inventory. It seals and reopens the full embedded catalog. This fixture
proves original reconstruction retention and passive reopening. It does not
prove maximum-size 2 GiB capacity or recovery-time objectives.

Small native compiler fixtures check fleet restore representation and exact
original archival. They do not qualify native Valkey or SQL guard activation.
The application/native owner qualifies those paths separately.

The combined race command reached its six-minute limit after the full backup
case passed. Its evidence remains in `race-final.jsonl`. The same bound applies
to separate full embedded and small-contract race commands. Separating the
commands changes test scheduling. It does not extend timeout or omit contracts.

Native lease and maintenance expiry require a distinct deletion-only import
owner contract. This API commit preserves exact CAS refusals. It does not claim
that an already-expired imported lease can pass ordinary persistent CAS. The
next compiler and native-owner changes must seal and retire those TTL records
without renewing or recreating them before final selection.
