# CSP23.1 public site at starport.agentstation.ai

CSP23.1 remains in progress on 2026-10-06. The site is live. Two A29 checks wait on owner actions.

## Site

- Starport #425 merged the `website/` tree as `b649dbf5`, which is also the `v1.3.0` tag commit.
- The site is a Next.js static export with Fumadocs. The build renders `docs/site` into 337 files.
- An assets-only Cloudflare Worker named `starport-site` serves the export. The custom domain is `starport.agentstation.ai`.
- The owner runs `npx wrangler deploy`. The permission classifier denies that command to the lead.
- The live manifest at `/docs/manifest.json` reports release `v1.3.0` and content revision `0b7f6bb15611`. It lists 328 files.
- The splash redesign is Starport #427. Its merge commit goes into the execution log.

## A29 state

The [live result](csp23/a29-live-2026-10-06.json) and the [task roster](csp23/task-roster-2026-10-06.json) agree.

- `A29.public_url_content_manifest` FAILS. Cloudflare Web Analytics injects a beacon script for the verifier user agent. A plain `curl` request receives the manifest bytes. The owner turns off the automatic beacon in the dashboard.
- `A29.hosting_rollback` reports UNVERIFIED. The record `csp23.1/rollback/rollback.json` does not exist yet.
- The other three A29 subcases PASS.

## Rollback exercise

The exercise needs three deploys of the Worker. Step 1 is the live `v1.3.0` deploy. The owner runs the `wrangler rollback` command for step 2 and for step 3.
After each owner step, the lead captures the live manifest. The lead then assembles `rollback.json` with schema version 1 and publishes it in a Starmap proof pull request.
The verifier requires a different content revision between step 1 and step 2, and the same revision between step 1 and step 3.
