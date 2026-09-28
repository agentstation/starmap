# CSP12.2 follow-up calls and admission modes

The operation matrix and quota-lease support boundary pass their registered checks.
The matrix result combines eighteen unchanged passes with the repaired image fixture.
The initial matrix run failed because its image fixture expected an obsolete provider path.
The fixture now uses the catalog-declared `/openai/images/generations` path.

The assembled matrix records 140 passing results across nineteen checks. It does not represent one complete task-gate invocation.
The remaining five task checks stay unverified. The complete task gate remains required.

## Production changes

Preset resolution omitted the primary model from the ordered routing list.
The router treated that list as complete, so requests skipped the intended first model.
The repair retains the primary model before its fallbacks. Explicit request model lists keep their existing precedence.

Batch chat and Responses tests verify provider model order, unique attempt identities, retained uncertain capacity, and refusal before an unfunded fallback.
Fallback cases use one line and token budgets. Ordinary batch cases still cover two lines and both budget dimensions.
The fixture does not enable automatic same-route retries or change embedding behavior.

The loader previously ignored the requested budget mode because no field accepted it.
`STARPORT_BUDGET_ADMISSION_MODE=atomic` now selects the only supported mode.
Other values fail validation in standalone and shared configurations. This change adds no local quota lease implementation.
The default, environment, and private configuration file select the same mode. Direct application construction also validates it.

## Video scope

Production HTTP tests seed accepted asynchronous jobs through the job repository.
They test provider polling, cancellation, and completed-asset collection through both API families.
Required spending or token budgets refuse provider dispatch without a verified follow-up billing bound.
Confirmed absent budgets permit one provider call. No case submits another generation or creates a duplicate generation reservation.

Refused asset collection preserves the completed job and leaves its asset absent.
Local asset reads reach no provider. These fixtures do not qualify live asynchronous provider support.

## Local application restart

The administrator test closes the application and reopens its real Badger and SQLite stores.
The uncertain reservation keeps its identity and prevents a second paid submission.
After the operator records usage or no-charge evidence, another restart preserves the decision.
An exact replay does not change the decision or repeat generation. Later corrections preserve the original audit record.

The thirteen race results include this lifecycle, disputed late evidence, original identity checks, and retained-evidence recovery.
These tests do not prove abrupt process loss or shared-backend replacement.
Twenty separate pure-Go results cover images and administrator reconciliation.

## Evidence and limits

Focused application race tests pass 33 results. The final configuration package passes 357 results with one container check skipped.
The proxy package passes 285 results with three Valkey checks skipped. Those checks remain unverified in this invocation.

Pure-Go checks pass 47 results without failures or skips. The verifier suite passes 99 tests.
Goago reports no findings, stale ignores, or incomplete errors. Vet and package layout pass.

Seven changed source and configuration files pass prose checks.
The broad prose run retains existing findings in the operator guide. Its new paragraph split repairs the added paragraph diagnostic.

The initial video fixture compared serialized time against an in-memory monotonic value. It now compares the same timestamp with `Time.Equal`.
The initial batch fixtures assumed an automatic same-route retry. The final fixture uses the supported preset fallback path.

A later probe used concurrent lines while asserting one global model order. The fallback fixture now isolates one line without changing production concurrency.
The retained failures include test-development errors and the obsolete image fixture path. The preset omission and silently ignored mode are the production defects.

Docker Desktop remains stopped. No paid inference or external publication occurred.
Interrupted-batch continuation still awaits the owner decision. Storage failover, retention horizons, latency, capacity, and final paired qualification remain required.
