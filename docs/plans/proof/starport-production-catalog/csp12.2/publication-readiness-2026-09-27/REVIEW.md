# Publication readiness review

The repair checks storage capability before file allocation and video dispatch.
Batch lines prepare their result file before invoking the provider runner.
The video recorder checks capability before binding the reservation or saving
an uncertain dispatch. General gateway readiness does not depend on this probe.

The blob owner defines the capability. Constructors remain passive. A configured
object-store client coalesces concurrent probes and caches success in memory.
Waiters can cancel. Failed probes permit another attempt after one second.

The probe has a 30-second deadline. Each multipart cleanup has a separate
five-second deadline. Warm checks make no network request or heap allocation.

The probe requires successful single-part and multipart creation. It also checks
refusal to replace live bytes and retirement markers. It reads retained bytes
before accepting the contract. HTTP 412 alone does not establish content safety.
A false success response prevents readiness. The tests use real MinIO for the
normal protocol and inject false responses for refusal cases.

Probe identities remain under the immutable publication namespace. Each attempt
uses up to four identities. Interrupted probes can leave small live objects or
incomplete uploads. Operators must preserve retirement markers and configure
incomplete-upload cleanup. This check does not prove backend-wide consistency,
physical erasure, backup recovery, or future availability. Changing backend
semantics requires a fresh client and probe.

The file and video controllers return HTTP 503 with a recovery message. Detailed
causes remain in logs. The startup log now says configured instead of ready.

This manual review does not replace the required pre-PR second-model review.
Native Linux and Windows qualification, shared production recipes, capacity,
aggregate reconstruction, and the full CSP12.2 acceptance remain open.

## Valkey scan work

The broad race run reached its eight-minute limit without an assertion failure.
Its stacks showed absent-key scans using COUNT 1. The shared database held about
20,000 keys. The result limit had incorrectly become the scan work hint.

The repair keeps COUNT 1000 and retains the caller's result limit. A real Valkey
regression inserts 4,096 unrelated keys and checks the scan request count.
The baseline exceeded its five-second deadline. The repaired scan used five
requests and retained the one-result bound.

A separate page test assumed completion within 1,000 cursor steps. Valkey filters
after scanning the whole database, so unrelated keys invalidate that assumption.
The test now follows the cursor to completion under a ten-second deadline.
It still verifies all 73 keys, literal prefixes, invalid inputs, and cancellation.
The production paginated-scan contract and its work hint remain unchanged.
