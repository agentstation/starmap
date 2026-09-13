# CSP6 producer draft

The producer integration remains uncommitted in `starmap-catalog-publication-promotion` at `b6e7891a9`.
Its three files are `internal/catalog/publication/producer.go`, `producer_attempts.go`, and `producer_test.go`.
No required check qualifies this draft.

The initial race check ended with status 1.
`TestProducerKeepsPartialAndFailedAttemptsDistinct` failed because its record issue omitted the required subject.
The complete result remains at `.tmp/csp6-promotion/producer-initial.json` in that worktree.
Session `50481` ended.

## Next correction

Correct the incomplete test fixture. Then verify source failures before accepting fresh evidence.
The draft currently ignores non-provider failures when a complete observation accompanies an error.
Preserve the typed source failure through collection and exclude that observation from fresh admission.
Keep successful provider bindings independent from other failed bindings.

The draft also returns collection errors before retained evidence can satisfy admission.
Preserve dependency-error causes at the pipeline boundary.
Permit retained evidence only for an identified source acquisition failure allowed by the profile.
Arbitrary configuration, filesystem, and cancellation errors must remain fatal.
Add a test through the real provider pipeline before qualification.

Reconciliation, receipt binding, retained-input storage, and workflow integration remain incomplete.
No CSP6 PR, merge, or completion credit applies.
