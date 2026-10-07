# CSP23.1 public site at starport.agentstation.ai

CSP23.1 remains in progress on 2026-10-07. The site is live. The A29 rerun waits on Starmap #243 and the zone RUM re-enable.

## Site

- Starport #425 merged the `website/` tree as `b649dbf5`, which is also the `v1.3.0` tag commit.
- The site is a Next.js static export with Fumadocs. The build renders `docs/site` into 337 files.
- An assets-only Cloudflare Worker named `starport-site` serves the export. The custom domain is `starport.agentstation.ai`.
- The owner runs `npx wrangler deploy`. The permission classifier denies that command to the lead.
- The splash redesign is Starport #427, merged as `06542922`. The owner deployed its build, labeled `v1.3.0`, as Worker version `bb08210b` on 2026-10-07.
- The live manifest at `/docs/manifest.json` reports release `v1.3.0` and content revision `0b7f6bb15611`. It lists 337 files.

## A29 state

The [2026-10-07 result](csp23/a29-live-2026-10-07-rollback.json) reports 4 PASS and 1 FAIL.

- `A29.public_url_content_manifest` FAILS. Cloudflare Web Analytics injects a beacon script for the verifier user agent. A plain `curl` request receives the manifest bytes. The owner chooses between the manual snippet in the site build and the automatic setup as a documented limit.
- `A29.hosting_rollback` PASSES with 3 recorded steps.
- The other three A29 subcases PASS.

## Rollback exercise

The exercise ran on 2026-10-07 with three deploys of the Worker. The owner ran each `wrangler` command. The lead captured the served manifest after each step.

| step | version | source | served release | generated_at |
| --- | --- | --- | --- | --- |
| 1 | `bb08210b` | main `06542922` | `v1.3.0` | `2026-10-06T23:47:20Z` |
| 2 | `fb522680` | `codex/public-site` at `12485718d2` | `dev` | `2026-10-06T01:07:59Z` |
| 3 | `bb08210b` | main `06542922` | `v1.3.0` | `2026-10-06T23:47:20Z` |

The content revision is `0b7f6bb15611` in all three steps because the docs content did not change. The release label and the build time separate the steps.
The dev build pins its repository links to commit `12485718d2`, which is not reachable from a branch. A `git fetch origin 12485718d2269245f49fd9520aa5cc8eee469e1b` makes it present for the verifier.
Starmap #242 merged `csp23.1/rollback/rollback.json` with schema version 1 and the three captures as `e4e25c2ad`.

## Beacon

The AgentStation account shows no RUM site for the `agentstation.ai` zone, and the edge still injects the token `eafc8874…`. That record is an orphan from the account that held the zone before the 2026-10-05 move.

On 2026-10-07, Codex created the Web Analytics site `starport.agentstation.ai` with manual setup in the AgentStation account. Codex also turned off the zone RUM injection.
The verifier then received the manifest bytes, and the [final A29 run](csp23/a29-live-2026-10-07-pass.json) reports 5 PASS.
Starport #432 wrote the beacon tag with the new token `2a073dbb…` into the build, so the manifest would list the served bytes.

## Analytics decision

On 2026-10-07 the owner chose the automatic Web Analytics setup, the same setup as the other AgentStation sites. The manual snippet made this site a special case, and the byte-exact HTML rule was its only cost. The lead closed Starport #432 without merge.

Starmap #243 (`3fb7a434`) changes `A29.public_url_content_manifest`. The check removes one Cloudflare beacon tag from each HTML page before the digest and reports the count as `beacon_pages`. Two tags on one page, a tag in a non-HTML file, or any other change still fail. Non-HTML files stay byte-exact. The [branch run](csp23/a29-live-2026-10-07-beacon-branch.json) reports 5 PASS with `beacon_pages` 0 while the zone injection stays off.

The task closes after the #243 merge, the zone RUM re-enable in the AgentStation account, and an A29 rerun with `beacon_pages` above 0.
