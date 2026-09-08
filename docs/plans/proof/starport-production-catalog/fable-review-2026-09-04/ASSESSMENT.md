# Fable review findings and corrections

Fable completed the requested product, developer-experience, and operator review.
It identified 11 new findings: three high, six medium, and two low.
It also agreed with the ten findings from the earlier audit.
The plan still needs revision before activation.

Read the [verbatim Fable review](../../../../reviews/FABLE_CATALOG_PRODUCT_REVIEW_2026-09-04.md)
alongside this assessment. The review includes persona journeys, scope proposals,
task changes, and five product questions.
This assessment distinguishes supported findings from proposals that need correction.
It changes no product requirement, implementation task, or accepted decision.

## Recommended revisions

The strongest additions are earlier first-use work, an explicit credential
migration rule, and a client-facing model identity contract.
Native path checks must also precede dependent implementation work.

| Finding | Assessment | Required next action |
| --- | --- | --- |
| FBL-01: delivery order | Supported | Move independent README and documentation fixes earlier. Define a user journey for each phase. |
| FBL-02: credential precedence | Supported | Define an upgrade conflict rule before reversing credential lookup order. Extend A11 and CSP9. |
| FBL-03: model identity | Supported, with authority limits | Define stable IDs, allowed aliases, removal errors, and request behavior across catalog changes. Extend CSP3 and CSP10. |
| FBL-04: GIF duration | Reasonable proposal | Keep pacing targets in human review. Retain measurable readability, size, and truthful recording checks. |
| FBL-05: public docs hosting | Partly supported | Specify the hosting service, deployment pipeline, URL ownership, and recovery under CSP18 and CSP23. |
| FBL-06: baseline record only | Conflicts with the requested outcome | Retain the inspectable baseline payload unless the user changes that requirement. |
| FBL-07: remove manual refresh mode | The proposed equivalence is incorrect | Preserve manual source refresh independently from a pin. Simplify the UI around operator intent. |
| FBL-08: defer local UI saves | Unsettled product choice | Compare limited local editing with export-only setup through a developer journey before removing writes. |
| FBL-09: daily branch promotion | Challenges confirmed D3 | Keep the four-hour requirement unless the user explicitly revises it. |
| FBL-10: earlier native checks | Supported | Add native path and interrupted-migration evidence to CSP2 and CSP8. Keep release checks in CSP22. |
| FBL-11: local provider enrollment | Useful later proposal | Assess guided discovery separately. Keep Starmap as the owner of catalog facts. |

The first-use work can use current commands and current product behavior.
Static docs must remain separate from authenticated deployment information.
A new published demo still needs a real inference path and artifact identity.
Earlier delivery does not waive those checks.

## Corrections and contract limits

### Preserve the requested baseline files

The original request explicitly requires the embedded baseline to reach the
product directory, even without another source.
D6 leaves the application-versus-library interpretation proposed.
It does not make the requested file outcome unnecessary.
The [specification](../../../../design/catalog-lifecycle/ENGINEERING_SPEC.md)
section 3.1 requires an inspectable manifest and payload.

Replacing that payload with a record changes the requested behavior.
A record-only design also retains accepted-head ownership and initial-write races.
Do not count all shared-store complexity as savings from removing the export.

### Keep manual refresh distinct from a pin

Specification section 6.2 defines manual source refresh separately from acquisition.
A pin freezes the effective catalog, including provider observations and workspace changes.
Manual source mode permits an explicit source refresh under the current authority.
These controls are not interchangeable.

Present a few task-based choices in the normal UI.
Keep their exact effects available through advanced configuration and diagnostics.
Reducing visible choices does not require deleting distinct operator capabilities.

### Bound model aliases by permission and capability

A rename contract must not restore an offering that internal authority withdrew.
An alias must remain within the permitted catalog and preserve the declared operation.
Do not silently route a retired ID to another model with different cost or behavior.
A successor suggestion can remain advisory.

One generation is an unstable deprecation interval when update frequency varies.
Define a time-based policy and an explicit exception for urgent withdrawals.
Distinguish new requests, retry attempts, queued work, and existing streams.
D13 and the unresolved AUD01 propagation contract still govern permission changes.

### Keep credential migration explicit

Current Starport code appends the derived environment name after conventional names.
Specification section 7.2 proposes the reverse for inference.
FBL-02 correctly identifies a missing migration mechanism.

Distinguish existing installations from fresh installations under the new precedence.
An upgrade needs an explicit selection or a diagnostic before changing the payer.
Resolve multi-field credentials as one handle. Compare values without disclosing them.
Do not block fresh installations merely because the documented precedence selects an override.

Evidence: Starport's
[credential resolver](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/credentials/resolver.go#L623)
and [configuration reference](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/config/README.md#L73).

### Qualify the budget-outage claim

Usage-total reads occur inside configured budget rules.
However, Starport reads a team's budget before it knows whether that rule exists.
A failed lookup does not prove that the team has no budget.
Fable's claim that all unbudgeted traffic avoids relevant reads is too broad.

The proposed policy must distinguish confirmed absence from unknown policy.
The budget decision remains pending. The earlier recommendation still blocks
requests when Starport cannot verify their required budget policy.

Evidence: Starport's
[team-budget lookup](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/budget.go#L85),
[lookup failure](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/budget.go#L116),
and [batch governor](https://github.com/agentstation/starport/blob/042eb97851496ecdd1ef20bfa44fe3d84b86b2d8/internal/server/batch_governor.go#L53).

### Interpret publication and recording claims accurately

CSP23 already owns public documentation publication.
The valid hosting gap concerns the mechanism and operational owner.
The plan's current authorization limit does not prohibit a later authorized site release.

The four-hour schedule does not guarantee a four-hour maximum checkout age.
Scheduler delay, source failure, and rejected publication can extend that age.
Daily promotion still needs the bot identity and validation machinery.
A release must embed the latest accepted input under D3.

The 10 MiB GIF limit is a project target in the reviewed brief.
The review provides no evidence that it is a renderer limit.
Fable provides no measurements for delivery time or configuration scope savings.

## Persona implications

| Persona | Recommended experience | Required failure path |
| --- | --- | --- |
| Local developer | Install, browse without keys, add inference access, and run one request. Show advanced settings only when needed. | Explain missing credentials and temporary state. Keep recovery docs accessible. |
| Startup | Keep client identifiers predictable. Make credential origins visible. Provide one tested path from local stores to a fleet. | Reject incomplete migrations. Explain budget uncertainty and catalog changes before traffic resumes. |
| Enterprise | Make authority, egress, accepted policy, and each replica's applied state explicit. | Explain the next authorized action when no approved catalog exists. Preserve authenticated diagnostics during refusal. |

Export-only local configuration might reduce implementation scope while increasing setup friction.
Validate that tradeoff before changing D12.
The early release proposal also needs dependency review before becoming a new ledger.

## Evidence and review limits

The reviewer used `claude-fable-5` at `xhigh` effort through the local Claude runner.
All recorded reviewer messages identify that model. The runner completed with exit code zero.
It used 32 read-only tool calls and reported no tool errors or permission denials.
The nine primary input hashes remained unchanged.

The runner telemetry also lists a Haiku call with 17 output tokens.
The command did not select that model. The saved result does not identify its purpose.
No reviewer message used Haiku. Preserve this distinction when citing model provenance.

Fable inspected the six main documents and all five screenshots.
Its claim to read Starport's configuration README fully is inaccurate.
The tool read requested its first 80 lines. That file contains 127 lines.
The Starport worktree also contained the existing `docs/TASKS.md` change.
It was not clean, as Fable's opening paragraph states.

Only one Fable review ran for this request.
The folder that Fable described as a prior review held this run's own evidence.
The review did not run tests, browse current external sources, or test interactive accessibility.
The earlier test evidence remains unchanged and does not qualify the proposed contracts.

The complete review and original request remain verbatim under the existing historical-review policy.
Its factual corrections appear here instead of changing attributed feedback.
The maintained assessment must pass the repository writing checks.

- [Review request](../../../../reviews/FABLE_CATALOG_PRODUCT_REVIEW_REQUEST_2026-09-04.md)
- [Input identities](input-manifest.json)
- [Runner result and model provenance](runner-result.json)
- [Recorded read requests](read-requests.json)
- [Earlier audit](../audit-2026-09-04/AUDIT.md)
