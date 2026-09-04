# CAT9.2 proof: the central server runbook and the Kubernetes pair

CAT9.2 owns three artifacts. The first is the enterprise catalog server
runbook. The second is the README link to it. The third is the Kubernetes pair
in the Docker document and the test that parses it.

CAT9 owns the rest of the maintained Starmap text.
`docs/proof/catalog-publisher/cat9.md` records CAT9.

## Fail before

The base commit is `ee65923c`. Neither artifact existed:

```console
$ git show ee65923c:docs/ENTERPRISE_CATALOG_SERVER.md
fatal: path 'docs/ENTERPRISE_CATALOG_SERVER.md' does not exist in 'ee65923c'
$ git show ee65923c:internal/deploymentdocs/doc.go
fatal: path 'internal/deploymentdocs/doc.go' does not exist in 'ee65923c'
```

The Kubernetes section of `docs/DOCKER.md` held one Deployment and no Service,
so no manifest wired a gateway to the catalog server.

The verifier reported both conditions as failures:

```console
$ GOTOOLCHAIN=go1.26.6 bash scripts/verify-catalog-distribution.sh
FAIL CAT-V59 the Starmap runbook has its eight sections and the README links it.
FAIL CAT-V64 the Docker document Kubernetes pair parses as two named Deployments and one Service whose selector and port match Starmap, and the Starport source URL names that Service.
Summary: 47 passed, 2 failed, 19 unverified.
```

## What CAT9.2 wrote

`docs/ENTERPRISE_CATALOG_SERVER.md` holds eight sections in the order of the
design record:

1. Select the store.
2. Start the server.
3. Provide credentials.
4. Set the interval and the policy.
5. Size the server for the fleet.
6. Point each Starport at the server.
7. Rotate the key and read the health routes.
8. Run the pair on Kubernetes.

The runbook names the single-server limit of the example. It also names the
active-active requirement. Two or more active servers need a lease-capable
store that supplies the CAT-D18 refresh lease and a conditional
compare-and-swap. A plain shared volume supplies neither, so it supports an
active and passive pair only.

`README.md` links the runbook from its HTTP server section.

`docs/DOCKER.md` gains a Service for the Starmap pods and a Starport Deployment
that reads that Service through `STARPORT_CATALOG_SOURCE_URL`. The section now
holds two Deployments and one Service. Each container keeps its own default
port 8080, because the two Deployments run separate pods.

`internal/deploymentdocs` is a test-only package. It adds no module. It uses
`github.com/goccy/go-yaml`, which the module already holds as a direct
dependency.

## The stale-name scan

The maintained text holds no removed name:

```console
$ git grep -n -E 'REFRESH_ON_START|REFRESH_INTERVAL|REMOTE_URL|REMOTE_API_KEY|REMOTE_ACTIVATION_INTERVAL' -- '*.md' ':(exclude)docs/proof' ':(exclude)docs/reviews'
```

The command prints nothing and exits 1. `cat9.md` records the unrestricted
scan.

## Tests

| Test | Package | Result |
| --- | --- | --- |
| `TestDockerDocumentKubernetesPairWiresStarportToStarmap` | `internal/deploymentdocs` | PASS with `-race` |

The test reads `docs/DOCKER.md`, extracts every YAML block under the
`## Kubernetes` heading, and parses each block with the module YAML dependency.
It then checks six properties. No ambient interpreter takes part.

## Mutation evidence

Each row mutates the document, runs the test, and restores the document. Every
mutation fails the test with the exit status 1.

| Property | Mutation | Diagnostic |
| --- | --- | --- |
| Two named Deployments | Rename the Starport Deployment to `starmap` | `the Kubernetes example has 1 named Deployment(s), want 2` |
| One named Service | Change the Service kind to `ConfigMap` | `the Kubernetes example has 0 named Service(s), want 1` |
| The selector matches the Starmap pod labels | Change the Service selector to `app: gateway` | `the Service selector app="gateway" does not match the Starmap pod label "starmap"` |
| The target port names a Starmap container port | Change the target port to `web` | `the Service target port "web" names no Starmap container port` |
| The Starmap container port equals the Service port | Change the container port to `9090` | `the Starmap container port 9090 does not match the Service port 8080` |
| The Starport source URL names the Service | Change the source URL to an external host | `STARPORT_CATALOG_SOURCE_URL is "http://catalog.example.com/api/v1", which does not name the Service host "starmap:8080"` |

A string count cannot prove any of the last four relations, because each one
compares two separate manifests.

## Commands

| Command | Result |
| --- | --- |
| `GOTOOLCHAIN=go1.26.6 go test ./internal/deploymentdocs -race` | ok |
| `GOTOOLCHAIN=go1.26.6 make lint` | 0 issues, ago clean, technical writing PASS |
| `GOTOOLCHAIN=go1.26.6 make test` | every package passed |
| `GOTOOLCHAIN=go1.26.6 go tool ago -stale-ignores -format json ./...` | no finding and no stale ignore |
| `GOTOOLCHAIN=go1.26.6 make technical-writing-check` | PASS, 758 files, 0 diagnostics |
| `GOTOOLCHAIN=go1.26.6 make docs-check` | all documentation is up to date |
| `bash scripts/verify-catalog-package-ownership.sh` | 13 passed, 0 failed |
| `shellcheck scripts/*.sh` | no diagnostic |
| `GOTOOLCHAIN=go1.26.6 bash scripts/verify-catalog-distribution.sh` | `PASS CAT-V59`, `PASS CAT-V64`, `Summary: 49 passed, 0 failed, 19 unverified.` |
