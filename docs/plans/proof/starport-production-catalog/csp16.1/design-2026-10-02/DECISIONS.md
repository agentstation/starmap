# CSP16.1 decisions

Date: 2026-10-02. Each decision is an engineering interpretation inside the accepted task scope.
None needs new owner authority. The owner can reverse any of them before the final acceptance run.

| ID | Decision | Reason and consequence |
| --- | --- | --- |
| 16.1-1 | Origin checks are same-origin checks, not loopback-only checks. | The auth-mode switch is loopback-only because it changes the lock. Configuration saves must work from a remote operator workstation. A console session always carries a browser Origin, so a cross-site page cannot drive a save. |
| 16.1-2 | A field save refuses a change to the acquisition policy identity. | D41 requires one coordinated `config apply` for a policy change. The save API reports the refusal with the apply instruction. CSP17 presents that path. |
| 16.1-3 | The local revision token is the SHA-256 checksum of the file bytes. | `productfiles.CompareAndPublish` compares bytes. One token serves the effective report, the save body, and the receipt. |
| 16.1-4 | Local receipts live in an operation journal next to the configuration file. | Local mode has no SQL store. The journal and the file share one writer lock, so an exact retry can complete from the published checksum. |
| 16.1-5 | Deployment authorization is the admin scope plus a `deployment_id` binding on each write. | No narrower operator scope exists, and the console session holds `*` by design. A foreign deployment identity refuses before any write. Denied-access journeys with a non-admin key belong to CSP17. |
| 16.1-6 | `db.Migrate` leaves the save path. | A field save must not change the schema. A store behind the current schema refuses the save and names the migrate operation. |
| 16.1-7 | The console UI stays in CSP17. | CSP17 owns console settings, forms, and types. CSP16.1 owns the HTTP API, the writers, and the receipts. |
| 16.1-8 | CSP16.1 lands as one Starmap contract PR and two Starport PRs. | The configuration operations and the baseline promotion are different concepts with different owners inside Starport. Two reviewable PRs keep each review bounded. The promotion PR follows its own survey. |

## Pending

- Baseline promotion design (D41 conditions 13 and 16): waits for the promotion survey. No promotion operation exists in either repository.
