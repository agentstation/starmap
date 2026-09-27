# Credential test repair review

Consumer `a819276e23a126c3eae159d5e2b2744db8bba619` completed one Sol and Opus high review pass.
Sol reports two P1 findings. Opus reports none. The helper exits 1.

Both findings repeat confirmed downstream release blockers. Populated fleets lack an adoption command.
A changed Valkey process identity requires controlled recovery, which CSP13 must provide.
The source still requires independent SQL approval and refuses automatic approval of a replacement backend.

Keep both findings as release blockers under CSP12, CSP13, and CSP15.
The accepted staged-integration disposition still applies. Do not release or deploy this intermediate change to existing fleets.
Reject the suggested return to the unfenced startup path. It would bypass the approved recovery contract.

This commit changes two in-memory tests and closes one test-owned cache.
It preserves the production source, configured deadline, identity assertions, and revocation assertions.
The review reports no new test-repair defect. Final CI and protected merges remain required.
