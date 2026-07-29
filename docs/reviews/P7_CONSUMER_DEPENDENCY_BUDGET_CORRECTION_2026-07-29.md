# P7 Consumer Dependency Budget Correction

Date: 2026-07-29

PR: [#55](https://github.com/agentstation/starmap/pull/55)

Failed candidate:
`36bd6679a73c709cb860c35ca1fa5df7eefd20a9`

Status: `ACCEPTED` — the read-only consumer budget now measures the
platform-independent non-standard package closure while the complete closure
continues to drive the forbidden-dependency scan.

## Finding

The first hosted Verification Gate on the exact failed candidate completed all
ordinary tests and all four external consumer programs, then rejected the
read-only consumer because its total dependency closure was 163 packages
against a ceiling of 160. Security & Reliability passed on the same head.

- [Failed Verification Gate](https://github.com/agentstation/starmap/actions/runs/30477140009/job/90661641566)
- [Passing Security & Reliability](https://github.com/agentstation/starmap/actions/runs/30477140009/job/90661641826)

The total-package measurement included standard-library implementation
packages selected by the Go target and CGO mode. The identical consumer and
module graph measured:

| Target | CGO | Total packages | Non-standard packages |
| --- | ---: | ---: | ---: |
| `darwin/arm64` | enabled | 159 | 31 |
| `linux/amd64` | disabled | 162 | 31 |
| `linux/amd64` | enabled | 163 | 31 |
| `windows/amd64` | disabled | 163 | 31 |

The additional Linux and CGO packages are standard-library internals such as
Linux runtime/cgroup support and `runtime/cgo`; they are not Starmap
dependencies and do not represent library-composition growth.

## Correction

`scripts/verify-consumer-deps.sh` now:

1. compiles and executes the external read-only consumer;
2. retains the complete unique dependency closure for the existing
   forbidden-family scan;
3. independently selects packages for which Go reports `Standard == false`;
4. enforces a ceiling of 32 non-standard packages against the measured closure
   of 31; and
5. reports both the enforced non-standard count and the platform-local total
   for diagnostics.

The one-package margin remains deliberately narrow. Any additional Starmap or
third-party package consumes it regardless of operating system, architecture,
or CGO mode. The change does not relax the explicit rejection of acquisition,
provider, pipeline, remote, server, scheduler, GenAI, gRPC, OpenTelemetry,
WebSocket, SQLite, Cobra, or CLI implementation families.

`internal/ciworkflow/pr_workflow_test.go` structurally pins the non-standard
selection, the 32-package ceiling, use of the full closure for forbidden-family
matching, and absence of the obsolete 160-total-package rule.

## Verification

```text
bash -n scripts/verify-consumer-deps.sh
go test ./internal/ciworkflow \
  -run 'Test(ReadOnlyConsumerDependencyBudgetIsPlatformIndependent|ExternalReadOnlyConsumerUsesCanonicalCatalogDX|PullRequestWorkflowPinsToolchainActionsToolsAndRequiredJobs)$' \
  -race -count=1
./scripts/verify-consumer-deps.sh
```

The focused race test passed. The external consumer gate reported:

```text
read-only consumer dependency closure: 31/32 non-standard packages
  (159 total on this platform); forbidden families absent
store-only consumer: external compile and publication test passed
server-embed consumer dependency closure: 247/260 packages;
  acquisition families absent
remote-subscriber consumer dependency closure: 231/240 packages;
  forbidden families absent
```

Direct cross-target `go list` measurement reported 31 non-standard packages for
all four target/CGO combinations in the table.

This report appends a correction to the earlier P2/P6 measurements. Their
historical total-package counts remain valid evidence for the machines on which
they were taken; they are no longer the prescriptive CI budget.
