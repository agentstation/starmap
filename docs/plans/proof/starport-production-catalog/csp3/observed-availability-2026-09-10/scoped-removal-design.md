# Scoped operator removal

This implementation design belongs to CSP3 and decisions D27 through D29. It has no implementation or acceptance credit.

## Current boundary

`Builder.DeleteProviderModel` deletes one provider record from a mutable catalog. It cannot retain an account-specific removal across reconstruction from earlier layers.
`Runtime.UpdateAcquisition` retains original observations and explicit acquisition resets through the existing publication journal.
An acquisition reset discards selected local evidence. It does not represent an operator decision to hide an entry supplied by lower layers.

The immutable catalog contains independent publisher and binding scopes. Starport still needs the explicit profile links required by D25 before it applies those scopes.
`Model.Status`, offering lifecycle, and offering availability apply beyond one account. They cannot represent the default removal action in D29.

## Removal contract

Keep operator removal separate from provider observations and observed availability. Provider acquisition cannot create, clear, or widen a removal record.
An explicit restore clears an operator removal. Provider recovery and ordinary baseline refresh must not clear it.
A restored entry appears only if the remaining catalog inputs still supply it. Restore cannot invent a removed baseline definition.

The default target identifies the selected provider entry and account scope. The target includes provider identity, exact provider model ID, and declared scope dimensions.
Preserve publisher identity, account or project identity, region, and API surface. A public provider target must explicitly identify its public scope.
Resolve the target from the validated scope link. Reject an ambiguous or unmatched target before publication.

Credential rotation must not clear a removal when the account and scope remain unchanged. A reassigned binding must not transfer removal to another account.
Binding revisions remain part of evidence validation. A revision number alone cannot define the operator's durable account target.

Canonical removal is a separate explicit action with a canonical model ID. It hides all offerings of that definition within the operator's authority.
The preview must name the affected entries before the operator applies canonical removal. Scope removal must not invoke canonical removal implicitly.

## Publication and storage

The catalog contract owns target validation and immutable removal queries. Runtime publication owns persistence, recovery, and concurrent operator changes.
Use the existing catalog publication journal for the removal snapshot and resulting generation. A separate file write must not precede catalog acceptance.
Bind each mutation to its expected generation. Concurrent changes must produce a conflict instead of silently replacing another operator's decision.

Persist the accepted removal snapshot with the same private-file policy as other runtime state. Include it in restart, migration, and backup verification.
Retain the selected baseline and original observations as reconstruction inputs. An operator removal filters their effective result without rewriting source receipts.

Generation transport must preserve removal records and their publisher identity. Consumers that cannot enforce the record format must reject that generation.
A generic artifact merge must not clear a removal because an incoming artifact omits it. Trusted replacement and explicit restore need distinct validated operations.
Upstream data cannot claim the local operator's publisher identity. A matching publisher string does not establish transport authority.

## Consumer boundary

Starmap exposes separate observed-availability and operator-removal results. Starport applies the validated scope link before filtering account-specific discovery or routing.
Other accounts retain their own visible entries and routing eligibility. Canonical removal applies only after the separate canonical action.
Internal authoritative permission withdrawals retain their immediate enforcement rules. Provider omission and operator removal cannot weaken those rules.

Build removal indexes before publishing an immutable generation. Request-time queries must use memory and avoid storage reads or catalog reconstruction.
Operator diagnostics must distinguish observed absence, explicit removal, and denied permission. General caller responses must preserve membership privacy.

## Required evidence

| Boundary | Required result |
|---|---|
| Account isolation | Removing one account entry preserves another account and another provider. |
| Refresh | Complete, partial, failed, and recovered provider replies cannot undo explicit removal. |
| Restore | Explicit restore removes the local exclusion and uses remaining accepted inputs. |
| Credential change | Rotation preserves the same account target. Reassignment cannot affect an unrelated account. |
| Canonical action | Only the separate canonical action hides every offering of the selected definition. |
| Publication failure | Failed commit preserves prior visibility and retained inputs. |
| Crash recovery | Recovery reproduces the accepted generation and removal state. |
| Concurrency | A stale expected generation cannot overwrite a newer operator change. |
| Transport | Import preserves removals and rejects unsupported enforcement or untrusted publisher claims. |
| Request path | Scoped removal queries perform no storage I/O and allocate no memory. |

These checks must qualify A08.scoped_tombstone before CSP3 closes. Starport end-to-end behavior remains with CSP8 and CSP10.
