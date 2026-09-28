# Restore file inventory and target report

The operator preparation command validates product inventory before creating or opening target stores.
Valid outer hashes do not prove that inventory metadata names valid product files.
Each selected payload must have one canonical role, relative name, artifact identity, and verified digest.
Unknown metadata, incomplete roles, missing payloads, duplicate identities, and unsafe names cause refusal.

The private metadata reader checks its byte bound and retained digest again.
The target report uses current configuration for all destination paths.
Source absolute paths remain historical data. Environment-file indexes do not select target environment files.
Lexical duplicate and ancestor destinations cause refusal. Native alias checks remain a publication requirement.

| Action | Required procedure |
|---|---|
| `target-configuration` | Keep selected target configuration and trust. Reconcile captured inputs explicitly. |
| `operator-credential` | Reconcile administrator access before token creation or restoration. |
| `verified-copy` | Verify content and target conflicts before publication. |
| `owner-recovery` | Let the owning component validate retained state and journals. |
| `new-instance` | Create a distinct replica identity and recover required evidence through its owner. |
| `retain-inactive` | Keep data for a role that the target does not select in inactive recovery storage. |

`backup prepare --json` returns this report as `file_plan`.
The text result states the pending file count and directs operators to JSON output.
The report does not publish files, approve admission, or prove later history.
Runtime ownership, native publication, independent reconciliation, and activation remain required.

Eight malformed inventories passed before this change. Their regression tests now refuse preparation before target access.
The final focused suites record 75 race passes and 75 pure-Go passes without failures or skips.
The broader configuration and CLI suite records 529 passes and one optional container-image skip.
Final constant naming checks add 20 race passes and 20 pure-Go passes.

Counts include parent and subtest outcomes. Separate runs are not combined into a unique-test total.
