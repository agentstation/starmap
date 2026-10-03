# CSP17 decisions

Date: 2026-10-03. Decisions 17-1, 17-2, and 17-4 are engineering interpretations inside the accepted task scope.
Decision 17-3 is an owner decision. The owner can reverse any of them before the final acceptance run.

| ID | Decision | Reason and consequence |
| --- | --- | --- |
| 17-1 | The catalog panel has one action, "Refresh sources", named by its effect. The console adds no pin control. | Starport has no route that pins the effective catalog, and CSP16.1 recorded that Starport never calls `PinAcceptance`. FBL-07 kept manual source refresh and pinning as different effects, so the panel names the one effect it has. |
| 17-2 | The panel shows the joined refresh run as the bounded-pressure fact and the permission result at completion as the admission result. | The catalog refresh has no queue. Overlapping requests join one run. The webhook queue is the only bounded queue with a depth and a capacity, and Settings already shows it. The console invents no queue. |
| 17-3 | CSP17 proceeds without step 4. Starport exposes no removal targets. Starmap keeps the rename and restore behavior. | Owner decision on 2026-10-03, with three options presented. Starport has no removal store, route, command, or screen, and the PRD and specification have no removal-target row. The CSP16.1 record states the limit. The CSP16.2 receipt still reports inert removals. The console documentation states the limit. |
| 17-4 | The light theme text-3 token changes once, from `#71717a` to `#66666e`. | Light text-3 on the raised background measured 4.40:1, below AA for prose in the catalog panel. The new value measures 5.18:1 on raised. The measured 2.56:1 settings pair was light text-4 on white, which CSP0.1 already moved to text-2 at 10.44:1. The implementation records both-theme ratios on each background. |

## Pending

None.
