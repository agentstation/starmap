# Gateway recovery approval and known-withdrawal checks

The tests use normal `New` construction, the shipped HTTP router, real gateway authentication, and the actual provider connector. Native Valkey and PostgreSQL use isolated deployment namespaces and SQL schemas. The local HTTP provider makes no paid inference call.

The fixture explicitly approves an unused namespace and SQL schema with the existing test-owner procedure. It does not claim fresh physical database initialization. An actual positive gateway starts and dispatches before the owner injects a failure.

Fresh child process cases cover missing SQL approval, missing native approval, stale native epoch, and copied records on a different native backend. Each refusal preserves the complete bounded native KV census and the independent SQL approval record. These cases use an ordinary fleet catalog. They do not qualify a real HistoryPackage or native import barrier.

The old child first dispatches a real request and retains its normal authorization cache. The parent closes independent SQL approval while the old Valkey process remains reachable. The child observes closure through its retained budget recovery authority. The next HTTP request must return 503 before provider dispatch. Required token budgets and confirmed absence of budgets use the same path.

The parent then approves a different actual Valkey process for this controlled namespace. The old gateway remains alive and reaches its original backend. It must refuse the next request after observing the changed approval. Readiness returns 503 and liveness returns 200. Verified local operator diagnostics remain available.

`Authority.CheckAdmission` reads a shared process-local atomic state without allocation or storage queries. `Authority.Check` marks authoritative closure or changed approval. Native bound-store operations mark an observed incarnation change. A canceled SQL query or transient storage failure cannot mark known withdrawal. Copied owners share the sticky state. A later approval cannot clear it.

An administrator or external restore can reinstate the original SQL row bytes. The old owner must still refuse `Check` and native mutation after it observed withdrawal. This test does not prove that a newly constructed owner can safely trust identical restored approval. Independent recovery evidence and external fencing remain mandatory.

These results cover participating reachable gateways and the stated owner observations. They do not qualify autonomous propagation from every catalog owner, full replica import history, complete deployment fencing, failover, RPO, or RTO. Fresh process checks against actual closed or pending native imports remain mandatory and UNVERIFIED. The 60-second authorization lifetime and 2-second propagation target remain unchanged.

Pure-Go and race gateway profiles passed 24 test cases each, with no failures or skips. Final focused authority profiles passed 7 test cases each with no failures or skips. The final evidence records the source boundary of each profile. Test setup limits remain distinct from product RPO and RTO.
