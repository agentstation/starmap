# Pull request audit

The audit found 15 open PRs: 12 in Starmap and three in Starport. Starmap #128 supersedes #127. The audit closed #127 and preserved its branch. The remaining 14 open PRs retain distinct work or required stack ancestry.

All audited PRs remain outside main. The owner has not authorized a release. Passing checks do not establish dependency-review completion or production qualification.

| Repository | PR | Decision and next action |
| --- | --- | --- |
| starmap | [#125](https://github.com/agentstation/starmap/pull/125) | Retain as the evidence root for #126. The failed Security & Reliability job stopped at FuzzParseAPIDataNoPanic with context deadline exceeded. The rerun of run 34162478515 passes all three jobs. The current diff contains 1,642 paths. The earlier 1,100-file scan describes a prior capture set, not this complete diff. A new exact-head inventory and credential scan cover all 1,642 paths and pass. |
| starmap | [#126](https://github.com/agentstation/starmap/pull/126) | Retain as the contract and implementation-plan parent for #132. It depends on #125. The active execution ledger now lives on #136; this earlier plan snapshot is historical. Complete parent validation before merge. |
| starmap | [#127](https://github.com/agentstation/starmap/pull/127) | Close as superseded by #128. Every added go.mod and go.sum line from #127 exists in #128. Every line removed by #127 is also absent from #128. The replacement has passing required checks. Reopen this update if #128 is abandoned without an equivalent replacement. |
| starmap | [#128](https://github.com/agentstation/starmap/pull/128) | Retain as the Azure dependency update. This PR includes all go.mod and go.sum changes from #127, which is superseded. Current required checks pass. Complete dependency review and validate integration before merge. |
| starmap | [#129](https://github.com/agentstation/starmap/pull/129) | Retain as the AWS configuration dependency update. Current required checks pass. Complete dependency review and validate integration before merge. |
| starmap | [#130](https://github.com/agentstation/starmap/pull/130) | Retain as the S3 dependency update. This PR also updates the server-storage consumer fixture module and checksum file. Current required checks pass. Preserve those fixture changes during integration and complete dependency review before merge. |
| starmap | [#131](https://github.com/agentstation/starmap/pull/131) | Retain as the Secrets Manager dependency update. Current required checks pass. Complete dependency review and validate integration before merge. |
| starmap | [#132](https://github.com/agentstation/starmap/pull/132) | Retain as the implementation foundation for #134. Commit 205dd041 integrates parent b58a9df6. Only proof and labels changed. The unchanged implementation diff passed its publication gate. Existing native evidence applies to identical inputs at f216e7c0. |
| starmap | [#133](https://github.com/agentstation/starmap/pull/133) | Retain as the acquisition and reset implementation parent for #135. Run 34186118026 passed all 11 jobs at 637f780a. This PR depends on #134. Revalidate the combined tree after lower stack merges. |
| starmap | [#134](https://github.com/agentstation/starmap/pull/134) | Retain as the captured-evidence parent for #133. The current diff contains 197 proof files. #133 now uses this branch as its actual base and its required review has passed. Revalidate ancestry after lower stack merges. |
| starmap | [#135](https://github.com/agentstation/starmap/pull/135) | Retain as the qualification-evidence parent for #136. The initial diff contains 21 proof files. This existing PR also receives the audit and repair evidence. The required code review for #136 used this actual base. Revalidate ancestry after lower stack merges. |
| starmap | [#136](https://github.com/agentstation/starmap/pull/136) | Retain as the active qualification PR. Run 34188963931 passed ten jobs, including all six native runtime jobs. Its Verification Gate failed the baseline allocation assertion. Repair that assertion and unused hook work, then complete review and new native CI before CSP2 acceptance. This PR depends on #135. |
| starport | [#366](https://github.com/agentstation/starport/pull/366) | Retain as the Starport documentation, README, performance, and native-workflow PR. All 16 recorded checks pass at published head 5820a370. The local branch has five unpublished commits through de0e002. Those commits have separate evidence and do not inherit this head's green checks. Resolve the Starmap module dependency, complete checks and required review, then update this existing PR. |
| starport | [#367](https://github.com/agentstation/starport/pull/367) | Retain as the Homebrew action update. All ten current checks pass. Complete pin provenance and dependency review before merge. This update is independent of the catalog PR. |
| starport | [#368](https://github.com/agentstation/starport/pull/368) | Retain as the Go dependency update. All ten current checks pass. It changes AWS clients, MySQL, SQLite, cryptography, and documentation dependencies. Complete dependency review and validate catalog-branch integration before merge. |

The catalog merge order is Starmap #125, #126, #132, #134, #133, #135, then #136. Evidence PRs hold required parent commits. Closing them would leave unresolved integration work. Starport #366 remains a separate PR. Its unpublished integration tests require a compatible Starmap module before publication.

Stack PRs target task branches. The Starmap workflow automatically runs for PRs targeting main. Empty PR check lists therefore require exact-head manual workflow evidence. Revalidate each combined tree when its parent changes. Never transfer green status from a different commit.

Use the existing PR for each owned change. Add another PR only when an independent scope or the required review tool needs a separate parent. Recheck this inventory at each publication and before merge. Close a superseded PR only after its replacement preserves all required changes.

The Azure comparison checked additions and removals in both module files against main. #128 preserves all changes from #127 and adds the identity-client update. Both branches remain outside main.

Evidence: the JSON snapshots retain exact heads, bases, check results, and complete local diff paths. GitHub list output caps file arrays at 100. The audit therefore used git diff for complete path inventories. A separate file retains the #125 fuzz log.

All 15 PR descriptions now state their dispositions. The initial audit added no PRs and changed no branch heads. The later parent check updated PR 132 as recorded below. The full prose check passed across 1,165 files.

The [foundation parent check](foundation-parent-sync/verification.json) records the published PR 132 update. Its prose check passed across 1,081 files.
