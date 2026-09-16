# Coordinated object catalog storage

`storage.NewCoordinatedObject` combines immutable object uploads with a separate coordination record.
The object service holds generation bytes. Valkey or Redis holds the current generation, upload reservations, and reader claims.
The existing `storage.NewObject` remains available without collection or reader leases.

This library adapter requires explicit caller configuration. It does not change the default Starmap filesystem store or add a CLI storage selector.
Starport's KV generation and chunk adapter has a separate integration contract.

## Construction and storage locations

The caller creates both backend clients and controls their credentials, transport, retries, and lifetime.
The coordinated-store constructor validates configuration without network access.
The first explicit `Commit` initializes an empty namespace.

| Setting | Purpose |
| --- | --- |
| `Prefix` | The object namespace for one catalog store. |
| `CoordinationKey` | The logical key for its complete coordination record. |
| `OwnerID` | A unique process-incarnation identity for reader diagnostics and recovery. |
| `WriteLifetime` | Protection interval for unfinished uploads. Zero selects five minutes. Positive values cannot exceed one day. |
| `Now` | Clock callback for unfinished-upload age. The default uses the system clock. |

Configure the object backend with `storage.MaxCoordinatedObjectBytes` as its object-size limit.
The limit includes the 32 MiB payload limit, the 64 MiB manifest limit, and ownership metadata.
The coordination backend must accept records up to 8 MiB. The registry also limits combined generation and reader entries to 100,000.

The object namespace contains:

```text
<Prefix>/current.json       Immutable binding to the coordination record
<Prefix>/uploads/<token>    One complete manifest and payload with an ownership header
```

Each upload receives a new random token. The header binds its format version, store identity, upload identity, and manifest.
The payload follows the header. Its manifest supplies payload integrity and compatibility checks.

The Valkey adapter maps `CoordinationKey` to a SHA-256 key suffix beneath its separately configured prefix.
See [Valkey coordination records](../pkg/catalogs/storage/valkey/README.md) for permissions, durability settings, and client ownership.
All coordination clients need write access for reader protection. These clients must belong to the trusted deployment control plane.
Use the Starmap server protocol when consumers must receive catalog data without coordination credentials.

## Publication and recovery

A commit reserves a unique upload identity through compare-and-swap before writing object bytes.
The object write uses conditional creation. Publication then compares the expected current generation and exact reservation before selecting the candidate.
A stale reservation cannot publish after collection retires it.

A delayed object write can finish after retirement. Its unique identity cannot replace a later upload.
The ownership header lets a later collection recognize and remove these orphan bytes.
No persistent tombstone is necessary for each retired upload.

An interrupted response can leave an ambiguous publication result.
Retry the same complete generation and original expected generation ID.
An identical accepted retry succeeds. Different content under an existing generation ID causes a conflict.

Collection protects current, required generations, and reader claims.
It verifies committed object versions against their accepted acknowledgments without downloading their payloads.
Changed object versions or sizes cause refusal. Unknown objects remain untouched.

`RetentionRequest.ScanEntries` bounds the inventory and registered claims.
`RetentionRequest.InputMaxBytes` bounds raw object bytes inspected during recovery. Zero selects 256 MiB.
The runtime forwards its configured input-byte limit. Its retained-input scan remains a separate bounded phase.
An incomplete inventory or exceeded byte bound stops before retirement or deletion.

The registry compare-and-swap commits retirement before physical deletion.
A deletion failure leaves unreachable bytes for a later recovery pass.
It does not restore publication rights or change the current generation.
Required content can exceed retention targets. The retention report then sets `OverLimit`.

## Reader protection

`Current` and `Get` create temporary reader claims while they load stored bytes.
`AcquireGeneration` returns independent bytes and a release function that retains stored content until successful release.
If the caller cancels the acquisition context, reader protection continues.
The release function supports repeated calls and retries after a failed release.
Release uses a separate thirty-second timeout. Failed acquisition also attempts cleanup with a separate timeout after a possibly accepted claim.
If coordination remains unavailable, cleanup can fail and leave a claim for fenced recovery.

Reader claims do not expire automatically. A process crash can leave claims that require operator recovery.
`ReaderClaims` reports the owner, generation, token, and inspected coordination revision.

1. Stop every process that uses the abandoned `OwnerID`.
2. Read the current claims and coordination revision.
3. Call `ReleaseFencedOwner` with that owner and exact revision.
4. Run collection again.

A changed revision refuses recovery without removing claims.
The caller must establish process fencing. The adapter cannot determine whether an external process can still access storage.
Use a new `OwnerID` after restart so the replacement process has a separate identity.

`CurrentAuthorityHead` reads current registry metadata without catalog payload access.
It creates no reader claim, repairs no data, and renews no permission receipt.
Call catalog storage during startup, refresh, and maintenance.
Use the active catalog in memory for inference requests.

## Namespace and failure boundaries

The immutable object binding prevents automatic initialization after coordination loss.
Missing, mismatched, oversized, or unsupported coordination records cause refusal.
A plain object writer also refuses the coordinated namespace because its expected current-pointer format does not match.

Use a new object namespace when migrating an existing plain object store.
Copy and verify every required generation before switching consumers.
Keep the old namespace until all readers stop using it.

This adapter requires persistent primary coordination without eviction or record expiration.
Automatic failover, coordination restoration, and service-specific disaster recovery need separate qualification.
A replica or unavailable coordinator cannot supply current authority through this adapter.

## Qualification

The storage conformance suite covers the coordinated store alongside memory, filesystem, and plain object storage.
Focused tests cover concurrent publication, reader protection, malformed state, lost coordination, bounded recovery, ambiguous publication, and late uploads after retirement.

The native integration test uses independent coordination clients with an actual local object service.
It checks shared reader protection, conditional deletion, current-generation recovery, and concurrent publication.
Set `STARMAP_TEST_VALKEY_ADDRESS` and `STARMAP_TEST_S3_ENDPOINT` before running:

```sh
go test -race -count=1 -timeout=5m \
  -run '^TestCoordinatedCatalogUsesNativeObjectAndCoordinationServices$' \
  ./pkg/catalogs/storage/valkey
```

The object endpoint must use HTTP on `127.0.0.1` and the fixture's test-only credentials.
The test creates a unique bucket and removes that bucket's objects after qualification.
The fixed compatibility fixture uses [MinIO RELEASE.2025-09-07T16-13-09Z](https://github.com/minio/minio/releases/tag/RELEASE.2025-09-07T16-13-09Z).
The upstream owner archived the repository. These results apply to the fixed fixture version.

The process-restart test starts separate reader and writer processes against the same services.
Each process exits before cleanup. The writer exits after the object upload and before publication.
The test restarts both services, verifies retained reader protection, and recovers the abandoned upload after its reservation expires.
It removes the exited reader's claim through checked owner recovery before collecting that reader's generation.

This test requires `STARMAP_TEST_COORDINATION_SERVER_CHANGES=1`, `STARMAP_TEST_COORDINATION_CONTAINER`, and `STARMAP_TEST_OBJECT_CONTAINER` in addition to the endpoint variables.
Use dedicated Docker containers with fixed published ports. Run one suite at a time against each service pair.
The pull-request workflow runs these tests with pinned service images on Go 1.27.1 for both Valkey and Redis.
