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
| starmap | [#135](https://github.com/agentstation/starmap/pull/135) | Retain as the qualification-evidence parent for #136. The initial diff contained 21 proof files. Published head 50557f05 now contains 174 changed proof files. This existing PR also receives the audit and repair evidence. The required code review for #136 used this actual base. Revalidate ancestry after lower stack merges. |
| starmap | [#136](https://github.com/agentstation/starmap/pull/136) | Retain as the qualified CSP2 implementation PR. Reviewed source 7e591262 passes all eleven jobs in run 34193810413. The final component report passes all 22 CSP2 subcases. Earlier run 34188963931 preserves the repaired allocation failure. Update its plan and evidence through the existing stack. This PR depends on #135. |
| starport | [#366](https://github.com/agentstation/starport/pull/366) | Retain as the Starport documentation, README, performance, and native-workflow PR. All 16 recorded checks pass at published head 5820a370. The local branch now has six unpublished commits through 69b5aff. Those commits have separate evidence and do not inherit this head's green checks. The published-commit candidate passes both full suites and 31 of 32 other required commands. V01 still requires a stable Starmap release tag. Adopt the approved tag, complete review, then update this existing PR. |
| starport | [#367](https://github.com/agentstation/starport/pull/367) | Retain as the Homebrew action update. All ten current checks pass. The pin review verifies upstream commit f8d4222 and unchanged setup-homebrew implementation. The remaining changes affect upstream maintenance files. This update is independent of the catalog PR. |
| starport | [#368](https://github.com/agentstation/starport/pull/368) | Retain as the Go dependency update. All ten current checks pass. It changes AWS clients, MySQL, SQLite, cryptography, and documentation dependencies. Complete dependency review and validate catalog-branch integration before merge. |

The catalog merge order is Starmap #125, #126, #132, #134, #133, #135, then #136. Evidence PRs hold required parent commits. Closing them would leave unresolved integration work. Starport #366 remains a separate PR. Its unpublished integration tests require a compatible Starmap module before publication.

Stack PRs target task branches. The Starmap workflow automatically runs for PRs targeting main. Empty PR check lists therefore require exact-head manual workflow evidence. Revalidate each combined tree when its parent changes. Never transfer green status from a different commit.

Use the existing PR for each owned change. Add another PR only when an independent scope or the required review tool needs a separate parent. Recheck this inventory at each publication and before merge. Close a superseded PR only after its replacement preserves all required changes.

The Azure comparison checked additions and removals in both module files against main. #128 preserves all changes from #127 and adds the identity-client update. Both branches remain outside main.

Evidence: the JSON snapshots retain exact heads, bases, check results, and complete local diff paths. GitHub list output caps file arrays at 100. The audit therefore used git diff for complete path inventories. A separate file retains the #125 fuzz log.

All 15 PR descriptions now state their dispositions. The initial audit added no PRs and changed no branch heads. The later parent check updated PR 132 as recorded below. The full prose check passed across 1,165 files.

The [foundation parent check](foundation-parent-sync/verification.json) records the published PR 132 update. Its prose check passed across 1,081 files.

The [later inventory](inventory-after-recovery.json) confirms 14 open PRs with unchanged heads. PR 127 remains closed. No additional PR is redundant.

The [owner-requested recheck](inventory-owner-followup.json) confirms the same 14 open PRs, with unchanged published heads and bases. PR 127 remains closed. All reported checks are successful or neutral. Six stacked Starmap PRs report no attached checks. Their manual evidence remains subject to the commit boundaries above.

At this capture, Starmap has 19 unpublished commits through `d641743c`, plus uncommitted CSP3 work. Starport has six unpublished commits through `69b5aff`, with a clean worktree. Published PR checks do not cover these local changes.

| Group | Owner | Next action and trigger |
| --- | --- | --- |
| Starmap catalog stack | Catalog plan executor | Finish the current coherent CSP3 repair, run its checks and required review, then update the existing PR. Preserve parent order. |
| Starport #366 | Catalog plan executor | Adopt the compatible approved Starmap tag, repeat integration checks and review, then update the existing PR. |
| Starmap #128–#131 and Starport #368 | Dependency reviewer | Complete dependency review and combined-tree checks before an authorized merge. Keep the updates independent of unfinished catalog acceptance. |
| Starport #367 | Release owner | The pin review and ten checks pass. Merge after authorization and a current-head check. |

Recheck this inventory before each publication and merge. A changed head or base requires a fresh evidence assessment. A replacement must preserve the required changes before the older PR closes. No additional PR qualifies for closure in this recheck. No new PR, merge, release, or branch deletion occurred.

The [maintenance capture](inventory-maintenance.json) records fourteen open PRs at 12:00 UTC on 2026-09-08. All attached checks report 54 successes and four neutral results. Six stacked PRs have no attached checks. PR 127 remains closed. No additional replacement justifies closure.

The ancestry check found one parent gap. PR 134 does not contain the latest PR 132 head, `205dd041`. Five missing commits change 23 proof paths. They change no implementation paths. Integrate that parent before the next stack publication, then validate each dependent tree. This gap does not make PR 134 redundant.

At this capture, Starmap has 27 unpublished commits through `c434a3b7` and six changed or new CSP3 source files. Starport has six unpublished commits through `69b5aff` and a clean worktree. These counts precede this audit commit. Finish the coherent CSP3 repair and required review before the next source publication. Update PR 136 and its existing evidence parent. Do not wait for the whole product plan to finish.

The plan executor owns the stack refresh and the dependency-review queue. Review independent dependency PRs separately from CSP3. A passing check does not replace dependency review or merge authorization. Before each publication, compare remote heads, bases, attached checks, local commits, and parent ancestry. Record a concrete next action for each retained PR.

The [follow-up parent update](followup-parent-sync/verification.json) publishes PR 134 head `99843ff8`, which includes foundation head `205dd041`. The merge changes 23 proof paths and no implementation paths. The prose check passes across 1,072 files. The automatic pre-PR gate skips model review because the diff contains no substantive code. Remote readback confirms the published head.

PRs 133, 135, and 136 still need that parent before their next publication. Preserve their implementation evidence while validating each combined tree. No default-branch merge, release, new PR, or additional closure occurred.
