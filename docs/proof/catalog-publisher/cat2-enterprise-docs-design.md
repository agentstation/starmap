# Enterprise deployment documentation design

Date: 2026-09-02. Owner task: CAT9.1. Decision: CAT-D20.

This record lists the operator documentation that an enterprise needs to run
Starport with a catalog source, and the gaps in the shipped text.

## Shipped text

Starport documents one process and a Valkey-backed multi-node deployment. The
operator guide covers remote catalogs in one section. It tells the operator to
set the remote URL, and it does not say what runs at that URL. Starmap
documents `starmap serve` as a developer API server. No shipped document in
either repository shows a central Starmap server that feeds a Starport fleet.
No deployment topology diagram exists in either repository. `.env.example` is
the only catalog configuration reference.

## Gaps

1. No runbook explains how to stand up a central Starmap catalog server.
2. No text explains a fleet of Starports, shared or separate subscriptions, or
   staggered activation.
3. No decision guide compares direct GitHub, a central Starmap server, and the
   embedded catalog.
4. The GitHub request budget and the freshness hop arithmetic appear only in
   the plan.
5. No procedure covers air-gapped or restricted-egress operation.
6. The workspace catalog has two sentences and no file layout.
7. No consolidated `CATALOG_*` reference or old-to-new name table exists.
8. No procedure shows how to inspect, pin, or roll back a generation.
9. No Kubernetes example shows the pair together.
10. The three-credential separation rule appears only in architecture text.
11. No alerting guidance covers catalog freshness.
12. No document shows a topology diagram.

## Documents

CAT9.1 adds or updates these documents.

| Repository | Document | Content |
| --- | --- | --- |
| Starport | `docs/DEPLOYMENT-TOPOLOGIES.md` | the four topologies, one diagram each, a decision table, and the request budget arithmetic |
| Starport | `docs/OPERATOR-GUIDE.md` | the catalog configuration reference, the workspace layout, the generation procedures, and the alert rules |
| Starport | `README.md` | one paragraph and one diagram that name the central Starmap server topology |
| Starport | `.env.example` | every `STARPORT_CATALOG_*` name from the canonical contract with its default |
| Starmap | `docs/ENTERPRISE_CATALOG_SERVER.md` | the central server runbook |
| Starmap | `README.md` | one paragraph that links the runbook from the HTTP server section |
| Starmap | `docs/DOCKER.md` | the Kubernetes pair example |

## Topologies

The topology guide describes four valid designs. Each design has one mermaid
diagram, its settings, its request budget, its freshness age, and its failure
behavior.

1. Single Starport with direct GitHub. No catalog settings. The gateway
   follows `catalog-latest` with conditional polling and runs its own
   acquisition.
2. Starport fleet with direct GitHub. Each replica polls GitHub with a shared
   token. The guide gives the header-driven budget arithmetic and the point at
   which a fleet needs a central server.
3. Central Starmap server with replica acquisition. One Starmap server follows
   GitHub and serves the fleet. Each replica sets the source URL and keeps its
   own acquisition on.
4. Central-only acquisition. Only the central server collects provider
   observations. Replicas disable acquisition and never reach GitHub or a
   provider. This is the air-gapped and restricted-egress design, with an
   optional OCI mirror.

## Central server runbook

The Starmap runbook covers, in order:

1. Start `starmap serve --auth` with a persistent store and a health probe.
2. Provide acquisition credentials and separate them from the server API key
   and from inference credentials.
3. Set the acquisition interval and the source policy.
4. Size the server for the fleet with the connection count and the SSE
   heartbeat.
5. Point each Starport at the server with the source URL and the API key.
6. Rotate the API key and read the health and readiness routes.
7. Run the pair on Kubernetes with the example manifests.

## Configuration reference

The operator guide gets one table with every `STARPORT_CATALOG_*` name, its
default, its valid values, and its interactions. A second table maps each
removed name to its replacement. The reference states that the removed names
have no runtime alias.

## Verification

- CAT-V53 checks that the topology guide names the four topologies and holds
  four diagrams. The Starport link check must also pass.
- CAT-V54 checks that the Starport README names the central Starmap server
  topology and links the guide.
- CAT-V55 checks that the operator guide and `.env.example` document every
  canonical `STARPORT_CATALOG_*` name and no removed name.
- CAT-V56 checks that the Starmap runbook exists with its seven sections and
  that the Starmap README links it.

Documentation conditions are declarative because prose has no runtime. The
verifier states this in its comment.
