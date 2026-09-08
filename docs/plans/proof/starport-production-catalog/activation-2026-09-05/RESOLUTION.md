# Goal activation and audit resolution

The user activated the whole-plan goal on 2026-09-05.
CSP0 is in progress. The active plan remains the sole execution ledger for both repositories.

The user authorized local implementation and non-destructive verification.
The worktree retains changes without commits until commit authority is explicit.
External publication, hosted-site writes, paid inference, and customer changes retain their separate authorization gates.
The plan requires preparation and local evidence before any missing external approval becomes a stop condition.

## Audit corrections

The following corrections close the document gaps. Their product behavior remains UNVERIFIED until the owning tasks supply acceptance evidence.

| Finding | Resolution | Owning tasks and evidence |
| --- | --- | --- |
| CA01 | Add eight local prerequisites to candidate qualification without completing publication-dependent parents. | CSP0 and CSP22. Candidate roster contains 305 subcases. |
| CA02 | Define controlled shared-store recovery with an independent PostgreSQL witness, KV incarnation checks, and fencing before access resumes. | CSP11, CSP12.2, CSP13, and CSP15. Three new A33 cases test the recovery boundary. |
| CA03 | Enumerate paid operations, preprocessing, semantic embeddings, and asynchronous reconciliation under one reservation contract. | CSP12.2, CSP15, and CSP22. Five new A47 cases cover operation-specific behavior. |
| CA04 | Bind admission windows, integer valuation, policy revisions, and retained reconciliation evidence. | CSP12.2 and CSP13. Five new A47 cases cover time and accounting. |
| CA05 | Require CSP12.2 before CSP13 and include reservation records in recovery. | CSP13. The active dependency graph must enforce this edge. |
| CA06 | Name Starmap CLI and administrative API reports and require positive parity before catalog readiness. | CSP1 and CSP14. Two new A32 cases cover reports and subscriber denial. |
| CA07 | Remove the blank line that split the acceptance table. | CSP0. The GFM parser must report 50 complete rows. |

The roster retains 50 primary cases and adds 15 subcases, for 324 required subcases.
The candidate gate requires 43 complete primary cases plus eight local checks, for 305 subcases.
The seven publication-dependent parents remain incomplete until their release checks pass.
Historical audit reports and raw evidence remain unchanged.

## Selected engineering defaults

D6 preserves passive library reads and persists the baseline at application startup.
D7 selects a complete source catalog as the distribution base, followed by authority-scoped reconciliation.
D11 keeps upstream Starmap administration separate from the Starport UI.
D15 keeps bootstrap node-local and places shared deployment configuration in PostgreSQL.
Execution selects these defaults. They do not claim explicit user confirmation.

The initial permission profile uses five minutes of validity and at most 30 seconds of clock uncertainty.
Expiry subtracts that uncertainty. Unknown clock validity blocks affected admission.
The model deprecation default remains 30 days, subject to earlier withdrawal or revocation.

The initial fleet recipe qualifies controlled recovery. Transparent automatic failover needs a separate equivalent fencing proof before support claims.
Strict budget enforcement measures Starport-accounted usage at pinned catalog prices. It does not guarantee the provider's eventual invoice.
Backend versions, capacity, reconciliation horizons, and numerical performance limits remain explicit qualification inputs.
The profile roster assigns their selection to the tasks that need them.

## Baseline verifier

The initial command failed with exit 127 because `scripts/verify-catalog-product.sh` did not exist.
The [missing-verifier record](missing-verifier.json) preserves that result.
The new command reports every primary case and named subcase without treating missing implementation as success.

The initial registry contains no product checks. The complete baseline therefore reports `Summary: 0 passed, 50 failed`, with 50 UNVERIFIED cases.
The named Go-test adapter requires a fresh passing test event and rejects missing matches, skipped tests, and failed commands.
The verifier retains command output in its JSON report.

Real environment, artifact, and reviewed-measurement adapters remain unimplemented.
Qualification requests refuse a pass until those adapters exist.
Component tests cannot establish production qualification through an argument alone.
Owning tasks must add executable behavior checks as their contracts become available.

## Remaining CSP0 checks

Verify the dependency graph, rendered acceptance rows, references, roster preservation, and historical hashes.
Run the verifier regression suite and writing checks.
Record exact counts and the initial red result in `csp0.md` before advancing to CSP0.1.
