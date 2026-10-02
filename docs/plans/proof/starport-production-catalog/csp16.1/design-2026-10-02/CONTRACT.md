# CSP16.1 configuration operations contract

Date: 2026-10-02. Base: Starmap `80de6d830`, Starport `a4e7e8fd`.
This file is the shared input for the Starmap contract PR and the Starport configuration PR.
Both repositories use the registry test names below. A test name change needs a change in both repositories.

## Subcase contract text

Register this text in `acceptance-map.json` under `subcase_contracts`.

| Subcase | Contract |
| --- | --- |
| `A26.local_file_stale_revision` | A local-mode save names the expected file revision. The writer publishes only when the current file bytes match that revision. A stale revision refuses with the expected and current revisions and changes no bytes. |
| `A26.shared_sql_stale_revision` | A shared-mode save names the expected head sequence. The store commits only at that sequence. A stale sequence refuses with the expected and current sequences and writes no revision. |
| `A26.failed_save_activation` | A receipt reports the saved revision and the applied revision separately. A saved revision that the running replica cannot activate stays saved, reports the activation error, and leaves the applied revision unchanged. |
| `A26.idempotent_operation_receipt` | Each save carries an operation ID. An exact retry returns the original receipt without a second write. The same operation ID with different edits, expected revision, or deployment refuses. A receipt is readable by operation ID after a process restart. |
| `A27.local_shared_authorization` | Configuration writes require the admin scope and a deployment identity that matches the configured deployment. An anonymous caller, a key without the admin scope, or a foreign deployment identity refuses before any write. |
| `A27.external_controller_lock` | External management refuses every configuration write operation, including save, initialize, and migrate. Read operations continue and report the external controller. |
| `A27.origin_protection` | A configuration write with an Origin header succeeds only when the origin matches the gateway's own origin. A console session write requires that header. A cross-site origin refuses with 403 before any write. |
| `A27.transactional_audit` | A shared write commits the head move, the revision row, and the audit record in one transaction. A failed audit record aborts the write and leaves the head unchanged. A local write publishes the file and its receipt under one writer lock. A retry with the original operation ID completes from the published checksum. |
| `A27.redaction` | Schema, effective, validation, save, and receipt responses never include a sensitive value. Audit records carry setting names and revisions, never values. A sealed value appears only as a presence marker and a checksum. |
| `A27.migration_boundary` | A field save cannot change the management mode or run a migration. The schema marks `STARPORT_CONFIG_MANAGEMENT` as migration-only. A save that includes it refuses. Initialization and migration remain separate operations with their own operation IDs. |

## Registry entries

Register each subcase in `scripts/catalog-product-checks.json` with `kind: all` over these `go_test` checks.
Every check uses `repository: starport` and `timeout: 5m`.

| Subcase | Package | Test |
| --- | --- | --- |
| `A26.local_file_stale_revision` | `./internal/config` | `TestLocalConfigurationSaveRefusesStaleRevision` |
| `A26.local_file_stale_revision` | `./internal/server` | `TestAdminConfigSaveReportsStaleLocalRevision` |
| `A26.shared_sql_stale_revision` | `./internal/app` | `TestSharedConfigurationSaveRefusesStaleRevision` |
| `A26.shared_sql_stale_revision` | `./internal/server` | `TestAdminConfigSaveReportsStaleSharedRevision` |
| `A26.failed_save_activation` | `./internal/app` | `TestFailedActivationReportsSavedAndAppliedRevisions` |
| `A26.failed_save_activation` | `./internal/server` | `TestAdminConfigReceiptSeparatesSavedAndApplied` |
| `A26.idempotent_operation_receipt` | `./internal/app` | `TestConfigurationSaveExactRetryReturnsOriginalReceipt` |
| `A26.idempotent_operation_receipt` | `./internal/app` | `TestConfigurationSaveRefusesReusedOperationID` |
| `A26.idempotent_operation_receipt` | `./internal/server` | `TestAdminConfigOperationReceiptSurvivesRestart` |
| `A27.local_shared_authorization` | `./internal/server` | `TestAdminConfigWritesRequireAdminScope` |
| `A27.local_shared_authorization` | `./internal/app` | `TestConfigurationSaveRefusesForeignDeployment` |
| `A27.external_controller_lock` | `./internal/app` | `TestExternalManagementRefusesConfigurationWrites` |
| `A27.external_controller_lock` | `./internal/server` | `TestAdminConfigSaveRefusesExternalController` |
| `A27.origin_protection` | `./internal/server` | `TestAdminConfigWritesRefuseCrossSiteOrigin` |
| `A27.transactional_audit` | `./internal/configrevision` | `TestCommitAbortsWithoutAuditRecord` |
| `A27.transactional_audit` | `./internal/config` | `TestLocalConfigurationSaveRetryCompletesFromPublishedChecksum` |
| `A27.redaction` | `./internal/server` | `TestAdminConfigResponsesRedactSensitiveValues` |
| `A27.redaction` | `./internal/app` | `TestConfigurationAuditRecordsOmitValues` |
| `A27.migration_boundary` | `./internal/app` | `TestFieldSaveRefusesManagementChange` |
| `A27.migration_boundary` | `./internal/server` | `TestAdminConfigSaveRefusesMigration` |

The shared-mode tests run against the SQL store that the Starport test fixture provides.
A test that needs an external fixture skips with `UNVERIFIED` in its message when the fixture variable is unset.
Local-mode and HTTP tests run without fixtures.

## HTTP admin API

All routes live under `/api/v1/admin/config` behind `RequireAdmin`.
Write routes add the deployment check, the origin check, and the management check before any store access.

| Method and path | Behavior |
| --- | --- |
| `GET /schema` | Setting descriptors from the Starmap `catalogconfig.Descriptors` projection: id, key, type, scope, mutability, sensitive, applicability. `STARPORT_CONFIG_MANAGEMENT` reports `migration-only`. |
| `GET /effective` | The redacted `EffectiveReport` from memory. No store or file access. |
| `POST /validate` | Validates field edits against the current revision. Returns the validation result and the resulting revision preview. No write. |
| `POST /test-connection` | Tests the catalog source reachability with the proposed values inside a bounded timeout. No write. The response never echoes a credential. |
| `POST /save` | Body: `operation_id`, `deployment_id`, `expected_revision`, `edits` (setting key to value). Returns a receipt. Stale revision: 409 with expected and current. Reused operation ID with a different target: 409. Exact retry: 200 with the original receipt. External management: 423. Missing admin scope: 403. Cross-site origin: 403. |
| `GET /operations/{operation_id}` | The stored receipt. 404 when unknown. |

## Receipt

```json
{
  "operation_id": "…",
  "deployment_id": "…",
  "management": "local|shared",
  "status": "applied|saved|refused",
  "saved": {"revision": "…", "sequence": 0, "checksum": "…"},
  "applied": {"revision": "…", "sequence": 0, "checksum": "…"},
  "activation_error": "…",
  "actor": "…",
  "created_at": "…"
}
```

`saved` is the revision the write produced. `applied` is the revision the running replica serves.
`status: saved` with `activation_error` is the failed-activation case. The receipt never carries setting values.

## Local management

- The local revision is the SHA-256 checksum of the configuration file bytes. The effective report and the receipt expose it.
- The writer uses the Starmap `productfiles.Directory.CompareAndPublish` primitive with the expected bytes derived from the expected checksum.
- The writer keeps an operation journal next to the configuration file under the same writer lock.
  An operation whose target checksum equals the current file checksum is complete. A retry returns its receipt.
- The writer records the audit entry after the publish. The receipt reports a failed audit entry, and the writer never reverts the file.

## Shared management

- A save reads `Current()` at the expected sequence, applies the validated field edits, and calls `configrevision.Commit(expectedSequence, values, actor, operationID)`.
- `db.Migrate` moves out of the save path. A save against a store behind the current schema refuses with a migrate instruction.
- A field save refuses a change to the acquisition policy identity. `config apply` remains the only path for that change (D41).
- Activation uses the existing revision observation path. The receipt reads the applied revision from memory after activation.

## Boundaries

- Request-path reads never touch the store or the file. Extend `TestRequestsReadAppliedRevisionInMemory` to cover the schema and effective routes (D41 condition 17).
- The console UI, forms, and types belong to CSP17.
- Baseline promotion (D41 conditions 13 and 16) is a separate Starport PR with its own survey and record.
