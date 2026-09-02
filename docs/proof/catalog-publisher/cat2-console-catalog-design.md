# Console catalog surface design

Date: 2026-09-02. Owner task: CAT8.1. Decision: CAT-D19. The third review
corrected the authorization split, the status semantics, the diagram, and the
placement on 2026-09-02.

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

The bar and the card spend a full row and a half card on four facts. The bar
flags a catalog older than seven days with the hard-coded
`STALE_AFTER_SECONDS` rule in `FreshnessBar.tsx`. Neither surface shows the
catalog source, the derivation layers, the acquisition schedule, or the next
update. The console reads none of `source_observations`, `sync_run_id`,
`validation`, or `payload_checksum` from `GET /api/v1/catalog`. The Overview,
Models, and Chat pages each answer the freshness question in a different
place, and Chat does not answer it.

## Principles

1. The catalog is a fact of the whole gateway. Every model, provider, and price
   on every page derives from it, so its indicator belongs in the shell.
2. The healthy state deserves almost no attention. A current catalog earns one
   small indicator, and an anomaly escalates the indicator.
3. Detail is on demand. A click opens one panel. The panel names the layers,
   the upstream hop chain, the changes, and the next update.
4. The server evaluates freshness. The chip, the panel, the metrics, and the
   alerts read one evaluation.
5. The surface reuses the shipped status vocabulary from `DESIGN.md`. A dot
   and a label show liveness, and a tint pill shows lifecycle.
6. Authorization shapes the surface. A `models:read` session sees the
   effective catalog. An admin session also sees the sources, the schedule,
   the provider outcomes, and the actions.
7. Each status concept has its own element. Usability, freshness, fallback,
   source health, and active work never share one indicator.

## Authorization boundary

The accepted contract in `cat2-final-review.md` keeps the safe routes for
`models:read` and assigns operational detail to the admin status route. The
console follows that split. CAT8.1 adds no source, chain, health, schedule,
or acquisition field to the safe route.

| Concept | Route | Scope |
| --- | --- | --- |
| effective generation ID, digest, generated time, activated time, and age | `GET /api/v1/catalog` | `models:read` |
| provider, model, and offering counts | `GET /api/v1/catalog` | `models:read` |
| server freshness verdict and policy age | `GET /api/v1/catalog` | `models:read` |
| changes between two generations | `GET /api/v1/catalog/changes` | `models:read` |
| selected source identity and directly observed source health | `GET /api/v1/admin/catalog/status` | admin |
| upstream-reported hop chain and layer generations | `GET /api/v1/admin/catalog/status` | admin |
| fallback and retained last-known-good state | `GET /api/v1/admin/catalog/status` | admin |
| acquisition policy, schedule, active run, and provider outcomes | `GET /api/v1/admin/catalog/status` | admin |
| refresh start, join, and cancel | the admin refresh routes | admin |

## Catalog chip

The chip is a shell-owned control in a shell-owned header slot. It renders on
every route, including Chat.

Content:

- Every session: a freshness dot, the label `Catalog`, the short generation ID
  in the mono face, and the age. Example: `● Catalog 01J9…K3Q 2h`.
- Admin session: the same content, plus separate pills and a trailing activity
  icon when the admin status document reports them.

Status elements, one per concept:

- Usability. Without an effective catalog the chip shows the error dot and the
  label `No catalog`. This is the only state without a generation ID.
- Freshness. The dot takes the success color for `fresh` and the warning color
  for `stale`. The verdict comes from the server. The age text also takes the
  warning color when stale.
- Fallback, admin only. A `fallback` pill in the warning tint appears while the
  runtime serves a retained last-known-good generation.
- Source health, admin only. A `source down` pill in the error tint appears
  while the directly observed source is unhealthy. An upstream-reported problem
  never raises this pill. It appears in the panel with the `reported by
  upstream` label.
- Active work, admin only. A small rotating icon follows the age while an
  acquisition or a refresh runs. It never replaces the dot. The tooltip names
  the active stage.

Size and placement:

- The chip is 32 px tall on a wide screen, one line, with no card border in
  the healthy state.
- The shell owns a 40 px header slot at the top of `main`, after the
  open-gateway banner. The slot is right-aligned and holds the chip.
- The page container drops its top padding from 32 px to 8 px. The space above
  a page heading stays the same, so no page gains vertical space.
- The slot adds its height to `--app-banner`. The full-height Chat route reads
  that variable and lays out below the slot.
- On a small screen the chip becomes a 44 px icon control, `size-11`, in the
  small-screen top bar next to Search. It shows the dot alone. The label and
  the tooltip carry the rest.
- The panel width is `min(480px, 100vw)`.

Interaction:

- The chip is a `button` with `aria-expanded` and an `aria-label` equal to the
  tooltip sentence.
- Click, Enter, and Space open the panel. Escape closes it. Focus returns to
  the chip on close.
- The tooltip is one sentence with the full generation ID, the age, the
  freshness verdict, and the policy age.

The Overview status hero keeps the model count. The Models page keeps its
filter row and its count. CAT8.1 deletes the freshness bar and the catalog card.

## Catalog panel

The panel is a right-side sheet with flat sections and hairline dividers. It
opens from the chip and closes with Escape.

1. Identity, `models:read`. Full generation ID with a copy control, catalog
   digest, generated time, activated time, and age. The section also shows the
   server freshness verdict and the provider, model, and offering counts.
2. Changes, `models:read`. The existing changes content moves here, with the
   from and to generations and the added, removed, and repriced rows.
3. Layers, admin. The derivation figure, described below.
4. Upstream hop chain, admin. The hop figure, described below.
5. Schedule, admin. Acquisition policy, last attempt, last success, and the
   next attempt as an absolute time and a relative time. An active run shows
   the stage, the bytes or pages completed, and the last progress time. Source
   polling shows the next check or the connected SSE stream.
6. Providers, admin. Provider outcomes with safe reason codes. Neutral skipped
   rows collapse by default.
7. Actions, admin. `Refresh catalog`, `Cancel refresh`, and `Copy status` for
   the sanitized status document.

A `models:read` session sees sections 1 and 2 and one sentence: "Source,
schedule, and provider detail need an admin session." The sentence is not an
error.

## Derivation figures

The panel draws two figures because the catalog has two distinct structures.
The layers are composition order. The hop chain is network distance.

The layers figure is a vertical list of four nodes in composition order:

1. Embedded baseline. Always present, with the generation ID that ships in the
   binary.
2. Selected upstream. The GitHub release or the Starmap server that the runtime
   selected, with its generation ID and observed time. The node reads `none`
   when the operator configures no source.
3. Local observations. The operator acquisition result, with the provider
   outcome counts, for example `14 succeeded, 2 skipped, 1 failed`.
4. Effective catalog. This Starport, with the effective generation ID and the
   activated time.

A fallback state marks the selected upstream node `retained` and names the
retained generation.

The hop chain figure lists the hops that the selected upstream reported, from
the origin to this Starport. Example: GitHub release, then a Starmap server,
then this Starport.

- The first hop above this Starport is the direct source. It carries the
  `direct` label and the health that this Starport observed.
- Every other node carries the `reported by upstream` label and the upstream's
  observation time. It carries no health dot of its own.
- The header shows the hop count against the configured maximum, for example
  `2 of 8 hops`.
- A GitHub release node shows the channel and the verified signer workflow. A
  Starmap server node shows the upstream generation ID and the hop age.

The figure data comes from the admin status document in
`cat2-final-review.md`. That document gives the selected source, the direct
source health, the upstream-reported chain, the provider outcomes, and the
fallback state.

## No-authorization state

When the safe route answers `401` or `403`, the chip shows the neutral dot and
the label `Catalog` without an ID. The tooltip reads "This session cannot read
the catalog." The shell makes one request and stops polling until the session
changes. It never retries in a loop.

## Data contract additions

The safe route `GET /api/v1/catalog` adds two fields:

| Field | Meaning |
| --- | --- |
| `freshness` | the server verdict, `fresh` or `stale`, and the policy age |
| `activated_at` | the time this Starport activated the generation |

The admin route `GET /api/v1/admin/catalog/status` carries the selected
source, the layer generations, the upstream-reported chain, the fallback
state, the schedule, and the provider outcomes. The console reads them only
from that route.

## Alternatives considered

- Sidebar footer dot beside the gateway status. Rejected. The footer already
  holds four controls and collapses to icons. The catalog is a data-plane fact,
  not process liveness.
- A Settings section. Rejected. Settings is for operator configuration, and
  the catalog question is a status question on every page.
- Keep the freshness bar and add fields. Rejected. The bar answers the question
  only on Models and spends a full row on the healthy state.
- Raw `position: fixed` placement. Rejected. It collides with the fixed
  sidebar, the open-gateway banner, the full-height Chat route, and the
  small-screen top bar.
- One merged `fresh`, `stale`, `degraded`, or `fallback` value. Rejected.
  Usability, freshness, health, fallback, and active work are independent.

## Verification

- CAT-V50 runs the chip test. The chip renders one element per status concept.
  A `models:read` render holds no admin pill and no activity icon.
- CAT-V51 runs the Starport HTTP boundary test in `internal/server`. The safe
  route serializes the freshness verdict and no source, chain, or acquisition
  field. The admin status route requires the admin scope and serializes them.
- CAT-V52 runs the panel test. The layers figure always holds the embedded
  baseline. The hop chain labels the direct hop and each upstream-reported
  hop. The schedule names the next update.
- CAT-V53 runs the shell test. The chip renders on Overview, Models, and Chat.
  The small-screen top bar holds the 44 px control. The panel never exceeds
  the viewport width.
- CAT-V54 runs the keyboard test. Enter and Space open the panel, Escape closes
  it, and focus returns to the chip.
- CAT-V55 runs the no-authorization test. After a `403` the shell shows the
  sentence and sends no second request.
- CAT8.1 acceptance also requires that no console file imports
  `FreshnessBar` or `CatalogCard`, and that `pnpm check` passes.
