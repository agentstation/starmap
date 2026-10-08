# CSP23 compatible releases and versioned docs

CSP23 reached its acceptance on 2026-10-07. The release workflows published both releases. A06 passes 4 of 4 and A29 passes 5 of 5 against the live site. A35 belongs to CSP24.

## Published pair

- The Starmap `v0.17.0` tag points at `2b2944be7`. Release run `37402744934` published it. It embeds `catalog/v1` sequence 44 (`c55f31dd9`).
- The Starport `v1.3.0` tag points at `b649dbf5`. Release run `37437368384` published 12 assets with one SLSA provenance attestation.
- Starport `v1.3.0` pins Starmap `v0.17.0`. The docs archive `starport-docs-v1.3.0.tar.gz` carries content revision `0b7f6bb15611`.
- The tags are immutable. The Go checksum database and the attestations record the bytes.

The [decision record](csp23/decisions-2026-10-05.md) keeps the owner answers, the release sequence, and the release run history.

## Task roster of 2026-10-06

The [task roster result](csp23/task-roster-2026-10-06.json) comes from `verify-catalog-product.sh --task CSP23 --released-assets`.
It ran from Starmap main `178705267` against a Starport checkout at the `v1.3.0` tag.
The task selects 10 subcases: 7 PASS, 1 FAIL, 2 UNVERIFIED. The other 48 cases were not selected and show UNVERIFIED.

| subcase | status | note |
| --- | --- | --- |
| `A06.promoted_checkout` | PASS | |
| `A06.new_released_module` | PASS | Starmap #237 compares against the channel head at the tag time. |
| `A06.old_pinned_bytes_unchanged` | PASS | The fail-before comparison of the previous pin bytes is part of this check. |
| `A06.starport_released_module_pin` | PASS | |
| `A29.embedded_offline_search` | PASS | |
| `A29.recovery_without_auth` | PASS | |
| `A29.no_dynamic_data_disclosure` | PASS | |
| `A29.public_url_content_manifest` | FAIL | The analytics beacon changes the served HTML bytes. See below. |
| `A29.hosting_rollback` | UNVERIFIED | The roster ran before the rollback exercise. The [2026-10-07 rerun](csp23/a29-live-2026-10-07-rollback.json) reports PASS with 3 steps. |
| `A35.released_installer_paths` | UNVERIFIED | The record `docs/assets/first-use-v1.3.0/record.json` waits on the CSP24 recording. |

## Open items

The [live A29 result](csp23/a29-live-2026-10-06.json) shows the cause of the FAIL.
The live Worker serves the `v1.3.0` build, and its manifest equals the local build manifest.
Cloudflare Web Analytics injects a beacon script into HTML responses for the verifier user agent.
The injected bytes differ from the manifest digests. The check stays strict. The owner turns off the automatic beacon in the Cloudflare dashboard.

The rollback exercise ran on 2026-10-07. Starmap #242 merged the record `csp23.1/rollback/rollback.json` as `e4e25c2ad`.
The [A29 rerun after the rollback](csp23/a29-live-2026-10-07-rollback.json) reports 4 PASS and 1 FAIL.
The owner then created a manual-setup Web Analytics site in the AgentStation account and turned off the zone RUM injection.
The [final A29 run](csp23/a29-live-2026-10-07-pass.json) reports 5 PASS against the live Worker version `bb08210b`.
Both items belong to [CSP23.1](csp23.1.md). CSP23 meets its acceptance: A06 and A29 pass against the released modules and the public docs URL.
