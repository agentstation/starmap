# CSP0.4 merged-source acceptance

CSP0.4 passes against Starmap merge `12fca002` and Starport merge `cffa9300`.
Both assigned subcases pass with Go 1.26.6.
The fresh complete HTTP baseline records 200 warm pairs and two initial pairs.
The numeric profile matches its retained engineering review.

All three accepted measurement source files match the merged Starport files.
The dependency files include the separately qualified PR 368 update.
The [verification record](verification.json) binds both merges and all captures.

Server and execution tests pass 473 test results and three packages with the race detector.
Two packages have no tests. The optional controller timer benchmark remains skipped.
The complete HTTP harness runs separately through the task verifier.
Router and credential benchmarks pass with allocation reporting.

This task establishes measurements and engineering targets.
The local loopback baseline does not establish production latency or complete A50.
CSP22 owns production performance qualification.
