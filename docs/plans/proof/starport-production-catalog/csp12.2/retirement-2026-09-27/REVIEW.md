# Local retirement repair review

Status: local review. CSP12.2 and its required pre-PR review remain incomplete.

The file owner now uses immutable publication and permanent retirement.
The mutable blob API remains available to its existing owners.
File schema 4 separates the new namespace from earlier file records.
CSP13 owns coordinated migration. The implementation does not read old records through a fallback.

The filesystem backend stages and flushes complete bytes.
A hard link publishes an unused identity without replacement.
Retirement replaces that identity with a flushed marker and flushes the directory chain.
Construction also flushes new root ancestors.
An uncertain flush retains file metadata and its byte claim.

The object-store backend sends conditional creation for single-part writes and multipart completion.
A real retirement object prevents later conditional creation, including in versioned buckets.
A local MinIO test pauses multipart completion, retires the identity, and verifies rejection after resumption.
The HTTP fixture proves SDK request handling only. The MinIO test supplies separate service evidence.

The delayed-publication regression retains its original quota and absence assertions.
Its wrapper now intercepts immutable publication, and its absence check reads the matching namespace.
A lost retirement acknowledgment retains the deleting file and full byte claim.
The next confirmed retirement permits one release.
A separate filesystem process cannot publish after retirement, including after the parent reopens the store.

Durable writes exposed a pagination test's single-pass timing assumption.
The failing Valkey result reached the 30-second sweep deadline.
The repair bounds each pass to 256 records and preserves page continuation.
The revised test still requires all 1,030 expired records to disappear and the live record to remain.
It also requires every retired identity to reject another publication.

The deadline did not increase. Required durability checks remain enabled.

## Limits and next action

Retirement markers remain indefinitely. Payload quotas exclude their backend overhead.
Backups and object lifecycle rules must preserve current markers.
Staging files, incomplete multipart uploads, and noncurrent versions need separate operational accounting and cleanup qualification.
This patch does not claim a physical disk-usage ceiling from logical payload quotas.

Native Linux and Windows execution, other S3 services, and crash-consistent backup restoration remain unqualified for this change.
Application construction still does not contact the bucket. Deployment readiness must not imply conditional-write qualification from construction alone.
Define and test the capability and readiness contract before production use.

The video owner still uses mutable blob writes and deletion. Audit its delayed-write and expiry behavior before full task qualification.

Reconstruct aggregates from durable line results, retain stable aggregate identities, and qualify checkpoint cleanup.
The batch restart-policy answer remains pending. Audited correction and the full paid-operation matrix remain required.
