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

The run passed all 23 selected subcases across 22 Go invocations.
It records 443 passing test events, with no failures or skips.
The compressed report and per-invocation evidence remain beside this record.
This record does not establish task completion.


Fifteen additional consumer checks pass.
The operator guide now states both accepted policies and batch schema 5.
Changed prose and document links pass. Fifty unchanged prose diagnostics remain, compared with 51 in the prior revision.
The PRD, engineering specification, and repository findings pass prose checks.

The producer check found stale OpenAPI and runtime reference files.
Commit `713813e0c` regenerates those artifacts. The complete documentation check then passes.
Consumer `4825016` changes only operator prose.

The task run retains its original source manifest because those documentation commits followed its start.
Later lint refactors changed handwritten source.
Both repository linters now pass without new suppressions.
The refactored consumer contracts pass 2,289 race results, with one SDK setup skip.
The separate SDK smoke script covers that test.

The adoption suite passes nineteen results and fails one transport fixture.
The launcher now supplies the required `redis://` URL.
Its focused rerun is active.
The producer repository gate runs separately with `GOWORK=off`.
Final-source qualification, review, and paired merges remain open.
