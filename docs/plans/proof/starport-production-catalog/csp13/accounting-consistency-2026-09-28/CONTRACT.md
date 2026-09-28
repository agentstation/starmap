# Captured accounting consistency

The budget owner derives each attempt's contribution to its original windows.
The recovery coordinator stores temporary totals in a private on-disk index with a bounded SQLite page cache.
Checks compare reserved capacity, seeded consumption, active disputes, and overflow with every captured window.
Canceled attempts retain zero consumption. Uncertain attempts retain their capacity.
Both aggregate overflow and unresolved valuation overflow retain their restrictions.

Each correction must reproduce its successor through the live correction-state contract.
Cycle detection uses constant memory. The linked receipt count must equal the retained receipt count.
A disconnected receipt or a changed correction head causes refusal.
Every receipt must remain within the original correction deadline.
Immutable state bindings establish ancestry even when authority time moves backward within that interval.

The preceding verifier accepts six distinct accounting inconsistencies in the retained regression.
The broader suite passes 476 race results. The focused suite passes 91 pure-Go results.
Those cohorts precede the timestamp repair.
Final qualification passes 91 focused race results and one pure-Go transition regression after that repair.
Every passing cohort has no failures or skips.

These checks establish internal consistency, not completeness against later acknowledged work.
Job/reservation links, correction-publication links, independent history, and full restore remain open.
Verification cannot change a balance, settle work, issue a retry, or approve admission.
Fifteen CSP13 acceptance subcases remain UNVERIFIED.
