# Console catalog surface design

Date: 2026-09-02. Owner task: CAT8.1. Decision: CAT-D19. The third review
corrected the authorization split, the status semantics, the diagram, and the
placement. The fourth review corrected the safe response projection, the chip
glyphs, the age definition, and the shell geometry.

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
update. The Overview, Models, and Chat pages each answer the freshness
question in a different place, and Chat does not answer it.

The safe route `GET /api/v1/catalog` writes the Go type `SnapshotMetadata`
from `internal/catalog/freshness.go` directly. That type carries
`validation`, `source_observations`, `sync_run_id`, `degradation_reasons`,
`degraded`, `completeness`, `manifest_unavailable_reason`, and
`payload_size_bytes`. A `models:read` session receives every one of them
today. The console reads `age_seconds` and `degradation_reasons` from that
response.

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
7. Each status concept has its own element. Usability, authorization,
   freshness, degradation, fallback, source health, and active work never
   share one glyph.

## Authorization boundary

The accepted contract in `cat2-final-review.md` keeps the safe routes for
`models:read` and assigns operational detail to the admin status route. The
console follows that split. CAT8.1 adds no source, chain, health, schedule,
or acquisition field to the safe route.

| Concept | Route | Scope |
| --- | --- | --- |
| effective generation ID, digest, generated time, activated time, and age | `GET /api/v1/catalog` | `models:read` |
| published time and channel update time of the effective generation | `GET /api/v1/catalog` | `models:read` |
| provider, model, and offering counts | `GET /api/v1/catalog` | `models:read` |
| server freshness verdict and policy age | `GET /api/v1/catalog` | `models:read` |
| changes between two generations | `GET /api/v1/catalog/changes` | `models:read` |
| selected source identity and directly observed source health | `GET /api/v1/admin/catalog/status` | admin |
| upstream-reported hop chain and layer generations | `GET /api/v1/admin/catalog/status` | admin |
| fallback and retained last-known-good state | `GET /api/v1/admin/catalog/status` | admin |
| degradation, validation, and the last check time | `GET /api/v1/admin/catalog/status` | admin |
| acquisition policy, schedule, active run, and provider outcomes | `GET /api/v1/admin/catalog/status` | admin |
| refresh start, join, and cancel | the admin refresh routes | admin |

## Safe response projection

The safe route stops writing `SnapshotMetadata`. CAT8.1 adds an explicit
response type, `CatalogSummary`, in the Starport `dto` package. The handler
projects the runtime state into that type. A field reaches the response only
when the type names it.

| Field | Meaning |
| --- | --- |
| `generation_id` | the effective generation ID |
| `catalog_digest` | the normalized fact identity |
| `payload_checksum` | the exact byte identity |
| `catalog_sequence` and `availability_revision` | the existing ordering values |
| `generated_at` and `generation_age_seconds` | the generation time and the age since it |
| `published_at` and `channel_updated_at` | the upstream release time and the last channel verification time, forwarded unchanged through every hop |
| `activated_at` | the time this Starport activated the generation |
| `counts` | `providers`, `models`, and `offerings` |
| `freshness` | `verdict`, `reference`, `policy_age_seconds`, `max_age_seconds`, and `evaluated_at` |

The `freshness.verdict` is `fresh` or `stale`. The `freshness.reference`
names the time the policy measures. For a `public`, `github`, or `starmap`
source it is the propagated `channel_updated_at` of the public release at the
head of the chain. Every Starmap hop forwards that time unchanged. A new
effective generation from local acquisition never resets it, so a stale public
channel stays stale behind any number of hops. Only a `file` or `embedded`
source without a channel measures `generated_at`.

The policy age is the age since that reference. The generation age and the
policy age are different numbers. Direct-source health and the acquisition
age are separate admin status facts, and neither one changes the verdict.
CAT-V65 proves the propagation across two Starmap hops with local acquisition
on each hop.

These fields never reach the safe response: `validation`,
`source_observations`, `sync_run_id`, `degradation_reasons`, `degraded`,
`completeness`, `manifest_available`, `manifest_unavailable_reason`, and
`payload_size_bytes`. The admin status route carries them.

Without an effective catalog the safe route answers `503`. The body is the
existing error shape with the message "No catalog is available." and no
internal detail. The response sets `Retry-After`. The route never maps that
state to `500`.

## Catalog chip

The chip is a shell-owned control in a shell-owned header slot. It renders on
every route, including Chat.

Content:

- Every session: a freshness dot, the label `Catalog`, the short generation ID
  in the mono face, and the policy age. Example: `● Catalog 01J9…K3Q 2h`.
- Admin session: the same content, plus separate pills and a trailing activity
  icon when the admin status document reports them.

The displayed age is the freshness-policy age from `freshness.policy_age_seconds`.
The identity section of the panel shows the generation age separately.

Status elements, one per concept:

- Freshness. The dot means freshness and nothing else. It takes the success
  color for `fresh` and the warning color for `stale`. The verdict comes from
  the server. The age text also takes the warning color when stale.
- Usability. Without an effective catalog the chip shows the unavailable glyph
  and the label `No catalog`. The glyph is a slashed circle in the error color.
  No dot renders. This state has no generation ID and no age.
- Authorization. After a `401` or `403` from the safe route, the chip shows
  the lock glyph and the label `Catalog`. The glyph is neutral. No dot renders.
- Degradation, admin only. A `degraded` pill in the warning tint appears
  while the admin status document reports a failed provider outcome. It also
  appears for a degraded overall health. The tooltip names the failed count.
- Fallback, admin only. A `fallback` pill in the warning tint appears while the
  runtime serves a retained last-known-good generation.
- Source health, admin only. A `source down` pill in the error tint appears
  while the directly observed source is unhealthy. An upstream-reported problem
  never raises this pill. It appears in the panel with the `reported by
  upstream` label.
- Active work, admin only. A small rotating icon follows the age while an
  acquisition or a refresh runs. It never replaces the dot. The tooltip names
  the active stage.

Every glyph differs by shape as well as by color. The freshness dot is a
filled circle. The stale dot carries a short exclamation mark inside it. The
unavailable glyph is a slashed circle. The authorization glyph is a lock.

Size and placement:

- The chip is 32 px tall on a wide screen, one line, with no card border in
  the healthy state.
- The shell owns a 40 px header slot at the top of `main`, after the
  open-gateway banner. The slot is right-aligned and holds the chip.
- On a wide screen the page container keeps 8 px of top padding instead of
  32 px. The slot and the padding total 48 px above the first heading. The
  layout therefore adds 16 px above every page heading on a wide screen.
- The slot adds its height to `--app-banner`. The full-height Chat route reads
  that variable and lays out below the slot.
- On a small screen no slot renders and the page keeps its 24 px top padding.
  The chip becomes a 44 px icon control, `size-11`, in the existing 48 px
  top bar next to Search. The small screen therefore adds no vertical space.
- The small-screen control shows the status glyph alone, with its shape and
  color. The `aria-label` carries the label, the ID, the age, and the verdict.
- The panel width is `min(480px, 100vw)`.

Interaction:

- The chip is a `button` with `aria-expanded` and an `aria-label` equal to the
  tooltip sentence.
- Click, Enter, and Space open the panel. Escape closes it. Focus returns to
  the chip on close.
- The tooltip is one sentence with the full generation ID, the policy age, the
  freshness verdict, and the policy maximum.

The Overview status hero keeps the model count. The Models page keeps its
filter row and its count. CAT8.1 deletes the freshness bar and the catalog card.

## Catalog panel

The panel is a right-side sheet with flat sections and hairline dividers. It
opens from the chip and closes with Escape.

1. Identity, `models:read`. Full generation ID with a copy control and the
   catalog digest. Generated time with the generation age, the upstream
   published time, the upstream channel update time, and activated time. The freshness verdict with the
   policy age and the policy maximum. The provider, model, and offering
   counts.
2. Changes, `models:read`. The existing changes content moves here, with the
   from and to generations and the added, removed, and repriced rows.
3. Layers, admin. The derivation figure, described below.
4. Upstream hop chain, admin. The hop figure, described below.
5. Schedule, admin. Acquisition policy, last attempt, last success, and the
   next attempt as an absolute time and a relative time. An active run shows
   the stage, the bytes or pages completed, and the last progress time. Source
   polling shows the last check time and either the connected SSE stream or
   the next conditional poll. A connected stream shows its last frame time.
   The runtime schedules no fallback poll during a connected stream.
6. Providers, admin. Provider outcomes with the safe reason codes from
   `cat2-final-review.md`, for example `skipped_not_configured`. Each row
   also shows the retained last-known-good observation with its generation
   and time. A `retained` mark shows that the effective catalog serves that
   observation instead of a fresh one. Neutral skipped rows collapse by
   default.
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

The effective generation ID differs from the upstream generation ID whenever
local observations exist. The figure shows both IDs in full so the difference
is visible.

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

## Data lifecycle

The shell owns one summary query. Every route reads the chip, the Identity
section, and the Changes section from that query, and no page sends its own
summary request. The query key is `catalog/summary`.

- Cadence. The summary refetches every 60 seconds while the document is
  visible, on window focus, and on network reconnect. It pauses while the
  document is hidden. The existing catalog queries set a stale time and no
  interval, so this is a new bound.
- Admin status. The admin status query polls only while the panel is open or
  an admin operation is active. It polls every 30 seconds while the panel is
  open and every two seconds during an operation. It stops when the panel
  closes and the operation ends.
- Admin pills. An admin session sends one status request at mount. It sends
  one more after each new generation in the summary. The pills therefore
  reflect the last activation, and the pill tooltip names the status time.
- Unavailable. After a `503`, the shell reads `Retry-After` as seconds or as
  an HTTP date. It sends the next summary request no earlier than that time.
  It clamps the wait between five seconds and five minutes. Without the header
  it waits 30 seconds.
- No authorization. After a `401` or `403`, the chip shows the lock glyph and
  the label `Catalog` without an ID. The tooltip reads "This session cannot
  read the catalog." The shell stops the summary and status queries. A session
  change resets them and sends one new request. A session change is a login,
  a logout, a token refresh, or a scope change. The shell never retries in a
  loop.

## Related surfaces

The accepted contract also lists candidate, accepted, rejected, and pending
route-validation state, model and offering provenance, catalog lifecycle,
credential-specific availability, and routing state. Those facts belong to
the model and provider pages, not to the chip. CAT8 owns the backend values
in the admin status route with CAT-V62. CAT8.1 owns the model page rendering
with CAT-V63.

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
- One dot for every state. Rejected. A dot that also means unavailable or
  unauthorized no longer means freshness.
- Serialize `SnapshotMetadata` and delete fields. Rejected. A deny list fails
  open when the type gains a field. An allowlist fails closed.

## Verification

- CAT-V50 runs the chip test. The chip renders one element per status concept.
  A `models:read` render holds no admin pill and no activity icon. The
  unavailable and authorization states render their glyphs and no dot.
- CAT-V51 runs the Starport HTTP boundary tests in `internal/server`. The
  first test fills every operational field with a sentinel value and proves
  that the safe response holds no sentinel. The second test proves that a
  missing catalog answers a sanitized `503`. The third test proves that the
  admin status route requires the admin scope and serializes the operational
  fields.
- CAT-V52 runs the panel test. The layers figure always holds the embedded
  baseline. The hop chain labels the direct hop and each upstream-reported
  hop. The schedule names the next update. The providers section shows the
  retained last-known-good row of a failed provider.
- CAT-V53 runs the shell test. The chip renders on Overview, Models, and Chat.
  The small-screen top bar holds the 44 px control. The panel never exceeds
  the viewport width.
- CAT-V54 runs the keyboard test. Enter and Space open the panel, Escape closes
  it, and focus returns to the chip.
- CAT-V55 runs the no-authorization test. After a `403` the shell shows the
  sentence and sends no second request.
- CAT-V68 runs the lifecycle test. Three routes share one summary request.
  The interval fires while visible and not while hidden. A `503` with
  `Retry-After: 120` delays the next request by 120 seconds. A `401` stops
  the requests, and a session change sends exactly one new request. The
  status query runs only while the panel is open.
- CAT8.1 acceptance also requires that no console file imports
  `FreshnessBar` or `CatalogCard`, and that `pnpm check` passes.
