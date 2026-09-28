# Final source qualification

Starmap `485b54376` separates provider model override validation.
Starport `af12f46` separates settlement, retained evidence, receipt replay, and asset publication.
Both linters pass without added suppressions.
Stored values, deadlines, and permission checks retain their prior contracts.

The consumer race suites pass 2,289 test events with one SDK setup skip.
The separate SDK smoke suite passes that test with Python, TypeScript, and Go clients.
Goago reports no findings, stale ignores, or errors.
Changed source prose adds no diagnostics. Fifty-one existing diagnostics remain.

The full consumer suite passes 5,877 test events, fails three events, and skips thirteen tests.
The failures belong to two top-level tests.
Catalog fixtures shared one Valkey authority key across unrelated runs.
Each fixture now gets a unique storage namespace that its child processes inherit.
The focused adoption test passes while a second invocation runs against the same services.
The second invocation also checks process takeover and leader-directory loss.

The scan test exceeds its original deadline in a shared database with 36,247 keys.
It passes all four events against empty logical database 15.
No deadline or assertion changed.
The initial failures remain in the compressed evidence.

The prior task gate passes all 23 selected subcases with 443 test events.
That result predates the lint refactors.
Final-source task qualification, required review, native CI, the published dependency, and paired merges remain open.
CSP22 owns full production latency and capacity qualification.
CSP12.2 retains its measured backend-call and spending-safety requirements.

The final-source task gate passes all 23 selected cases across 22 invocations.
It records 443 passing events, with no failures or skips, in 1,311.9 seconds.
The tested commits are producer `485b54376` and consumer `af12f46`.
All four task containers now report a stopped state.
The broader producer repository gate remains active.

Consumer review preflight passes.
Producer preflight refuses the generated catalog gzip before model review.
The owner question requests bounded JSON-gzip support in the shared review tool.
No final review, push, native CI run, or merge occurred.
