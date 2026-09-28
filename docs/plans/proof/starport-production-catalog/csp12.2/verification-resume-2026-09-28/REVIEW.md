# Verification evidence retention

Producer commit `c0209d288` saves each completed Go invocation when `CATALOG_PRODUCT_GO_EVIDENCE_DIR` names an evidence directory.
The JSON files retain the command, exit status, duration, output, and named test results.
A timeout retains partial output.
An evidence-write failure produces `UNVERIFIED`.
The verifier does not reuse these files to grant later passes.

The verifier suite passes 101 tests.
Changed prose passes its check.
The earlier task run stopped without a final report.
Its test outcomes remain unverified.
The baseline merge changed ancestry without changing source bytes.
Stopping that run was unnecessary.

The replacement task run uses the exact commits in `csp122-task-resume-source.json`.
Its process session is `14422`.
Its output is `/tmp/csp122-task-resume.json`.
Its evidence directory is `/tmp/csp122-task-resume-evidence`.
Collect those files after completion.
This record does not establish task completion.
