# Console catalog surface design

Date: 2026-09-02. Owner task: CAT8.1. Decision: CAT-D19.

This record designs the Starport console catalog surface from first
principles. It replaces the freshness bar and the overview card with one
shell-owned catalog chip and one detail panel.

## Current surfaces

The console shows catalog facts in three places today.

| Surface | File | Space | Facts |
| --- | --- | --- | --- |
| Freshness bar | `console/src/components/models/FreshnessBar.tsx` | one full-width row on Models, 36 px tall | short generation ID, age, four state badges, details popover, changes button, refresh button |
| Catalog card | `console/src/components/overview/CatalogCard.tsx` | half of the two-column Overview grid | generation, generated time, catalog sequence, availability revision |
| Changes panel | `console/src/components/models/ChangesPanel.tsx` | a 480 px sheet | models, offerings, and prices that changed between two generations |

The bar and the card spend a full row and a half card on four facts. Neither
surface shows the catalog source, the derivation chain, the acquisition
schedule, or the next update. The console reads none of `source_observations`,
`sync_run_id`, `validation`, or `payload_checksum` from `GET /api/v1/catalog`.
The Overview, Models, and Chat pages each answer the freshness question in a
different place, and Chat does not answer it.

## Principles

1. The catalog is a fact of the whole gateway. Every model, provider, and price
   on every page derives from it, so its indicator belongs in the shell.
2. The healthy state deserves almost no attention. A current catalog earns one
   small indicator, and an anomaly escalates the indicator.
3. Detail is on demand. A click opens one panel. The panel names the source,
   the derivation, the changes, and the next update.
4. The server evaluates freshness. The chip, the panel, the metrics, and the
   alerts read one evaluation.
5. The surface reuses the shipped status vocabulary from `DESIGN.md`. A dot
   and a label show liveness, and a tint pill shows lifecycle.

## Catalog chip

The chip is a shell-owned control in the top-right corner of the content area.
It renders on every route, including Chat, at one fixed position.

- Content: a state dot, the label `Catalog`, the short generation ID in the
  mono face, and the age. Example: `● Catalog 01J9…K3Q 2h`.
- Size: 32 px tall, one line, no card border in the healthy state.
- Fresh state: the success dot.
- Stale state: the warning dot and the age in the warning color.
- Degraded or fallback state: the error dot and a one-word reason pill.
- Progress: an active acquisition replaces the dot with the spinning refresh
  icon. The tooltip names the active stage.
- Tooltip: one sentence with the full generation ID, the age, the source
  identity, and the next update time.
- Placement: `position: fixed`, 16 px from the top and the right edge of the
  content area, above the page header. The shell reserves 40 px of top padding
  when the viewport is narrower than 1440 px. A page header then cannot collide
  with the chip. On a small screen the chip collapses to the dot and the icon in
  the small-screen header, next to Search.
- Click and Enter open the catalog panel. The chip has `aria-label` with the
  same sentence as the tooltip.

The Overview status hero keeps the model count. The Models page keeps its
filter row and its count. CAT8.1 deletes the freshness bar and the catalog card.

## Catalog panel

The panel is a right-side sheet, 480 px wide, with flat sections and hairline
dividers. It opens from the chip and closes with Escape.

1. Identity. Full generation ID with a copy control, catalog digest, generated
   time, activated time, and age. The section also shows the server freshness
   verdict and the provider, model, and offering counts.
2. Sources. A derivation diagram, described below.
3. Schedule. Acquisition policy, last attempt, last success, and the next
   attempt as an absolute time and a relative time. An active run shows the
   stage, the bytes or pages completed, and the last progress time. Source
   polling shows the next check or the connected SSE stream.
4. Changes. The existing changes content moves here, with the from and to
   generations and the added, removed, and repriced rows.
5. Providers. Provider outcomes with safe reason codes. Neutral skipped rows
   collapse by default.
6. Actions. `Refresh catalog` and `Cancel refresh` for an admin session, and a
   `Copy status` control for the sanitized status document.

A `models:read` session sees sections 1 through 4 from the safe metadata
route. An admin session also sees sections 5 and 6 from the admin status
route. A missing admin scope shows one sentence, not an error.

## Derivation diagram

The diagram answers "where did this catalog come from" in one glance. It is a
vertical chain of nodes, drawn with flex layout and SVG connectors, with one
node per hop in the sanitized source chain.

- Node kinds: embedded baseline, GitHub release, Starmap server, operator
  acquisition, and this Starport.
- Each node shows its kind icon, safe identity, generation ID, observed time,
  and health dot. A GitHub release node shows the channel and the verified
  signer workflow. A Starmap server node shows the upstream generation ID and
  the hop age.
- The operator acquisition node branches from the side and shows the provider
  outcome counts, for example `14 succeeded, 2 skipped, 1 failed`.
- The bottom node is this Starport with the effective generation ID.
- The header shows the hop count against the configured maximum, for example
  `2 of 8 hops`.
- A fallback state draws the active fallback path in the error color and dims
  the disconnected upstream path.

The chain data comes from the runtime status contract in
`cat2-final-review.md`. The contract gives the selected source, the direct
source health, the upstream-reported chain, the provider outcomes, and the
fallback state.

## Data contract additions

The safe route `GET /api/v1/catalog` adds five fields:

| Field | Meaning |
| --- | --- |
| `source` | selected source kind and safe identity |
| `chain` | sanitized hop list with kind, identity, generation ID, observed time, and health |
| `freshness` | the server verdict, `fresh`, `stale`, `degraded`, or `fallback`, and the policy age |
| `next_attempt_at` | the next scheduled source check or acquisition run |
| `acquisition` | enabled flag, interval, in-progress flag, and provider outcome counts |

The admin route `GET /api/v1/admin/catalog/status` keeps the detailed
document. The console reads `source_observations` only through the
`chain` and `acquisition` fields, so the two routes share one shape.

## Alternatives considered

- Sidebar footer dot beside the gateway status. Rejected. The footer already
  holds four controls and collapses to icons. The catalog is a data-plane fact,
  not process liveness.
- A Settings section. Rejected. Settings is for operator configuration, and
  the catalog question is a status question on every page.
- Keep the freshness bar and add fields. Rejected. The bar answers the question
  only on Models and spends a full row on the healthy state.

## Verification

- CAT-V50 runs the chip test. The chip shows the state, the generation, and
  the age, and it opens the panel.
- CAT-V51 runs the Starport metadata test. The safe route reports the source,
  the chain, the freshness verdict, and the next attempt.
- CAT-V52 runs the panel test. The sources section draws one node per hop and
  names the next update.
- CAT8.1 acceptance also requires that no console file imports
  `FreshnessBar` or `CatalogCard`, and that `pnpm check` passes.
