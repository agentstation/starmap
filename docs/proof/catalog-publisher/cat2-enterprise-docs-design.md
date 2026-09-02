# Enterprise deployment documentation design

Date: 2026-09-02. Owner tasks: CAT9.1 for Starport and CAT9.2 for Starmap.
Decision: CAT-D20. The third review split the tasks by repository and
separated restricted egress from air-gapped operation on 2026-09-02.

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
5. No procedure covers restricted-egress or air-gapped operation, and no text
   separates the two.
6. The workspace catalog has two sentences and no file layout.
7. No consolidated `CATALOG_*` reference or old-to-new name table exists.
8. No procedure shows how to inspect, pin, or roll back a generation.
9. No Kubernetes example shows the pair together.
10. The three-credential separation rule appears only in architecture text.
11. No alerting guidance covers catalog freshness.
12. No document shows a topology diagram.
13. No text explains a replicated central server, its lease, or the store
    that active-active servers require.

## Documents

Each task owns one repository, so each task maps to one pull request.

| Task | Repository | Document | Content |
| --- | --- | --- | --- |
| CAT9.1 | Starport | `docs/DEPLOYMENT-TOPOLOGIES.md` | the five topologies, the replicated variant, one diagram each, a decision table, and the request budget arithmetic |
| CAT9.1 | Starport | `docs/OPERATOR-GUIDE.md` | the catalog configuration reference, the workspace layout, the generation procedures, and the alert rules |
| CAT9.1 | Starport | `README.md` | one paragraph and one diagram that name the central Starmap server topology |
| CAT9.1 | Starport | `.env.example` | every `STARPORT_CATALOG_*` name from the canonical contract with its default |
| CAT9.2 | Starmap | `docs/ENTERPRISE_CATALOG_SERVER.md` | the central server runbook |
| CAT9.2 | Starmap | `README.md` | one paragraph that links the runbook from the HTTP server section |
| CAT9.2 | Starmap | `docs/DOCKER.md` | the Kubernetes pair example |

## Topologies

The topology guide describes five valid designs and one replicated variant.
Each design has one mermaid diagram, its settings, its request budget, its
freshness age, its egress, and its failure behavior.

1. Single Starport with direct GitHub. No catalog settings. The gateway
   follows `catalog-latest` with conditional polling and runs its own
   acquisition.
2. Starport fleet with direct GitHub. Each replica polls GitHub with a shared
   token. The guide gives the header-driven budget arithmetic and the point at
   which a fleet needs a central server.
3. Central Starmap server with replica acquisition. One Starmap server follows
   GitHub and serves the fleet. Each replica sets the source URL and keeps its
   own acquisition on.
4. Restricted replica egress. The central server keeps its egress to GitHub
   and to the providers. Each replica reaches only the central server and
   disables acquisition. This design is not air-gapped, because the central
   server still reaches the internet.
5. Air-gapped mirror. No host inside the boundary reaches GitHub or a
   provider. The runtime has no OCI source, so an external process outside
   the boundary pulls the artifact and its verification bundle. The
   operator transfers both across the boundary on a schedule. The operator
   then verifies the bundle offline with the CAT4 verification command and
   exposes the artifact through the supported `file` source. Every host
   disables acquisition, and the freshness age follows the transfer cadence.

The replicated variant applies to the three central-server designs. Two or
more central servers run active-active behind one load balancer only on a
lease-capable shared store. That store provides the CAT-D18 lease and a conditional
compare-and-swap on the generation record. A plain shared filesystem volume
provides neither, so the guide limits it to the single-server design and to
the active and passive form. In that form one standby starts only
after the active server stops. The fleet subscribes through the balancer and reconnects
on failover.

## Central server runbook

The Starmap runbook covers, in order:

1. Select the store. A single server uses a persistent volume. Active-active
   servers use a store with the CAT-D18 lease and conditional writes.
2. Start `starmap serve --auth` with the store and a health probe.
3. Provide acquisition credentials and separate them from the server API key
   and from inference credentials.
4. Set the acquisition interval and the source policy.
5. Size the server for the fleet with the connection count and the SSE
   heartbeat.
6. Point each Starport at the server with the source URL and the API key.
7. Rotate the API key and read the health and readiness routes.
8. Run the pair on Kubernetes with the example manifests in `docs/DOCKER.md`.

The Kubernetes example holds two Deployments and one Service. The Starmap
Deployment mounts a persistent volume and runs `starmap serve --auth`. The
Starport Deployment sets `STARPORT_CATALOG_SOURCE_URL` to the Starmap
Service. The example is a single-server design, and the runbook says so.

## Configuration reference

The operator guide gets one table with every `STARPORT_CATALOG_*` name, its
default, its valid values, and its interactions. A second table maps each
removed name to its replacement. The reference states that the removed names
have no runtime alias.

## Verification

- CAT-V56 checks that the topology guide names the five topologies and the
  replicated variant, and holds six diagrams. The Starport link check must
  also pass.
- CAT-V57 checks that the Starport README names the central Starmap server
  topology and links the guide.
- CAT-V58 checks that the operator guide and `.env.example` document every
  canonical `STARPORT_CATALOG_*` name and no removed name.
- CAT-V59 checks that the Starmap runbook exists with its eight sections and
  that the Starmap README links it.
- CAT-V64 parses the Kubernetes YAML in `docs/DOCKER.md` and checks the
  structure. The check requires two Deployments with different names, one
  Service, and a Starport container whose source URL names that Service. A
  string count is not enough.

Documentation conditions are declarative because prose has no runtime. The
verifier states this in its comment.
