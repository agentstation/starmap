# CSP1: Shared catalog settings contract

Status: done for the local task. Six acceptance subcases pass. Starport adoption and released-pair qualification remain with their later tasks.

## Initial evidence

The [initial checks](csp1/missing-public-contract.json) found no public settings package and no passing CSP1 acceptance registrations.
The [public API tests](csp1/public-contract-red.json) failed before implementation.
The [resolution tests](csp1/resolution-red.json) also failed before their API existed.

The inspected internal loader skipped every empty value.
Its lookup API could not enumerate or reject unknown input names.
The application YAML loader exposed only legacy remote-source fields.
Its dotenv loader read `.env` before `.env.local`, so the first file prevented the second file from overriding duplicates.
Service startup also loaded incidental working-directory dotenv files.
The application repairs below replace these inspected behaviors.

## Local implementation

`pkg/catalogs/config` now owns the 22 existing catalog parsers and their runtime option conversion.
Its public descriptors identify grammar, defaults, scope, sensitivity, applicability, source binding, mutability, schema introduction, compatibility, and documentation anchors.
`internal/catalog/settings` retains application composition and flags. It delegates parsing to the public package.

`Parse` rejects unknown canonical names and retains supplied false, zero, and permitted empty values.
`CanonicalValues` maps file keys and semantic IDs to canonical names. It rejects conflicting aliases.
`Load` accepts caller-supplied lookups. It cannot enumerate unknown names, so complete input maps must use `Parse`.
No public parser reads the environment, filesystem, or network.

`Resolve` selects values from ordered, named authorities.
An explicit source identity selects a complete source group from its own or higher layers.
It excludes lower source credentials and clears previously composed source credentials before applying the selected group.
Independent refresh settings retain normal precedence.
Origins and ignored-value diagnostics contain no credential values.
The hosting application must select eligible authorities before calling this resolver.

The [cascade regression](csp1/cascade-zero-red.json) reproduced loss of explicit zero startup spread.
The composition adapter now maps explicit zero to the subscriber's documented negative disable value.
Absence still selects the subscriber default. The public value remains zero.

One old parser fixture used the unsupported startup policy `embedded`.
It now uses `prefer_source`, which the connected runtime supports.
The test still requires one option for every supported setting.

## Application and reference

The command now resolves flags, process environment, explicit dotenv files, and canonical YAML through the public resolver.
All 22 catalog settings retain their values through the YAML loader.
Unknown keys fail. Command construction defers validation until explicit flags can override lower scalar values.

Dotenv files require `--env-file`. Later files replace earlier file values, while process values retain precedence.
Catalog values stay in named file layers. Other values supply the existing acquisition environment only when absent.
Legacy source aliases produce redacted migration diagnostics.
Malformed or duplicate files fail before environment mutation.

The [dotenv regression](csp1/dotenv-provenance-red.json) reproduced credential promotion during a repeated command.
The repaired command preserves file authority and excludes old transport credentials from a replacement source.
The [base-policy regression](csp1/source-base-policy-red.json) reproduced loss of a host's node aliases during source replacement.
The resolver now resets only source-bound defaults and credentials.

The [standalone reference](../../../CATALOG_SETTINGS.md) and [descriptor export](../../../catalog-settings-schema.json) derive from the same descriptors.
The freshness test fails when either generated file differs from its descriptors.
The reference distinguishes current Starmap behavior from later Starport adoption and platform path work.

## Verification

The task verifier covers six CSP1 subcases through named behavior checks.
The [task result](csp1/task-verification.json) records each fresh invocation and its result.
The [application checks](csp1/application-final-checks.json) retain race, vet, lint, and writing results.
The [minimum-version check](csp1/go125-compatibility.json) passed with Go 1.25.12 across the public contract, settings adapter, and command application.
The [architecture checks](csp1/architecture-checks.json) passed package layout, catalog ownership, dependency direction, and 30 verifier regression tests.

The full task race run passed 217 test and subtest events with no skips.
Go vet, targeted golangci-lint, and whole-repository ago passed.
Generated reference prose and the maintained source passed their targeted writing check.
The [final writing check](csp1/writing-final.json) still reports three diagnostics in the unchanged historical storage-revision captures.
No policy or historical evidence changed to hide these diagnostics.

ER01 moved three premature path checks.

CSP8 retains those checks after CSP2.
All 50 primary cases and 324 final required subcases remain mandatory.
CSP1 does not qualify full A24, full A25, native platform behavior, or a released product pair.
No commit, push, pull request, or release exists for this work.
