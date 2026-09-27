# Consumer bootstrap repair

CSP11 remains in progress. D41 remains approved.
The independent bootstrap contract resolves the protocol scope before this implementation.

PostgreSQL records one permission for the first catalog publication. Only explicit fresh initialization grants it.
The native Valkey transaction still checks the original grant, expiry, and exact predecessor.
Complete catalog-key loss cannot create fresh permission.
Acceptance retains 32 distinct generation IDs independently from publication receipts.

## Current evidence

- Both prior real-store regressions failed before the repair. The adjacent consumer review evidence preserves them.
- The focused repair run passes 13 named race results with real PostgreSQL and Valkey. No test skips or failures occurred.
- The broader command passes 74 named results before its five-minute package timeout. The timeout remains failed evidence.
- The only unfinished test passes in a separate invocation with the same timeout and assertions. The split broad coverage totals 75 passing results.
- Lint reports zero issues. All 99 acceptance-verifier regression tests pass.
- All eleven required CSP11 task subcases pass with the new regressions included. All 34 consumer repository commands and the full producer checks pass.
- Sol and Opus reviews run on the committed branches. Publication and merges remain open.

## Ownership and limits

A SQL consumption followed by an absent Valkey head requires controlled recovery. There is no transaction across both stores.
CSP13 owns recovery after interrupted publication, populated deployment adoption, and Valkey restart or failover.
The operator documentation now states that these commands are unavailable in this intermediate build. Production fleet use requires their qualification.

CSP12 owns supported-recipe validation. CSP15 qualifies alternative backends.

The source hash map includes catalog and SQL Go files and all three new migrations.
Historical runner hash maps have narrower scope. They do not alone bind the new recovery and SQL source.
The committed source is producer `0f45bbae239381bf6f674e5834409bd4cceec1ef` and consumer `aab7dd7c8bf52b2046e2284e480d81ce1d0b873c`.
Final review evidence must complete the binding before publication.
