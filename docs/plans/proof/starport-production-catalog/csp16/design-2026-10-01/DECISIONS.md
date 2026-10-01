# CSP16 design decisions during implementation

Date: 2026-10-01. Decided by the plan lead inside the approved design. The owner did not need a new product decision for these items.

## Decision 1: the authority namespace is the Valkey key prefix

Question: the specification names a bootstrap authority namespace, but the code has no such setting.

Decision: no new bootstrap setting. The namespace is `deployment.KeyPrefix(STARPORT_DEPLOYMENT_ID)`. The head row stores the deployment ID and the namespace. A stored namespace that differs from the bootstrap-derived prefix refuses startup and names the stored namespace and deployment ID. An absent head returns the not-initialized error. When head rows exist for other deployment IDs, that error lists their deployment IDs.

Reason: a new setting adds operator surface that no subcase requires. The key prefix is the real namespace that catalog state uses. The A28 contract names both the deployment identity and the authority namespace, so both fields satisfy it.

## Decision 2: the shared revision seals sensitive source values

Question: `STARMAP_CATALOG_SOURCE_API_KEY` and `STARMAP_CATALOG_SOURCE_TOKEN` are deployment-scope and sensitive. The resolver ignores every local deployment-scope value under shared management, so a private catalog source needs the credential inside the shared revision.

Decision: the revision record seals the two values with the existing store secret through `credentials.EncryptionService` (AES-256-GCM), the same mechanism that shared provider keys use. The checksum excludes them. Reports, audit records, and errors show presence only. A sealed value that the current store secret cannot open refuses startup with a typed error. The checksum mismatch and the unseal failure keep separate error identities.

Rejected alternative: refuse sensitive values in a shared revision. That alternative blocks shared management for a fleet with a private catalog source.

## Noted deviation from the Starmap brief

The registry leaf timeout for the `./internal/app` and `./internal/configrevision` tests is the duration string `5m`, not the integer `300`. The existing registry entries use duration strings, so the lead accepted the change.
