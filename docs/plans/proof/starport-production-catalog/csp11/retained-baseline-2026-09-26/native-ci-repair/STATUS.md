# Native CI repair

CSP11 remains in progress. Both PRs remain open.
Native Windows startup rejects the inherited scratch ACL before readiness.
Starmap now creates a private directory exclusively through the native file API.
Starport keeps its scratch layout, owner record, and cleanup contract.

The macOS retention test allowed an automatic startup pass during its explicit-call assertion.
Its controlled timer now holds a positive startup delay.
The assertion remains unchanged and passes 25 repetitions with race detection.

The cask verifier assumed architecture-first nesting. GoReleaser 2.17.1 emits platform-first nesting.
The repair parses checksum guards without executing Ruby cask code.
Both nesting orders must select exactly Apple silicon macOS and ARM/x86-64 Linux.
Existing hook, checksum, and artifact checks remain required.

The fleet CI job exhausted its ten-minute limit after 74 passing catalog tests and before initialization completed.
Its total limit is now twenty minutes, including cold compilation and artifact upload.
The seven-minute catalog and three-minute initialization test bounds remain unchanged.

The JSON report binds local checks to the candidate source.
The consumer used an explicit temporary workspace for the new producer API.
Final checks require a published module with GOWORK=off and native CI on the final heads.
