# CAT9.3 proof: the source maximum age sets the channel thresholds

CAT9.3 wires `source.MaxAge` into the channel freshness grade. The option
already existed, and the runtime already validated it, but no grade read it.

## The gap

`WithSourceMaxAge` stored `r.source.MaxAge`, and `SourcePolicy.Validate`
checked that the value is not negative. Nothing else in the runtime read it.

The channel grade in `runtime_status.go` read `r.config.freshness`, which held
`DefaultFreshnessPolicy()`. That policy warns at six hours and turns critical
at ten hours.

An operator who set `STARMAP_CATALOG_SOURCE_MAX_AGE=7d` still saw the channel
warn at six hours. The setting named a one-week tolerance, and the grade
ignored it.

`remote/source.go` already reads `SourceConfig.MaxAge` to degrade cascade
health, and `internal/catalog/settings/composition.go` already passes
`Config.SourceMaxAge` there. CAT9.3 changed neither path.

## The precedence rule

An explicit freshness policy wins. `runtimeOptions` gains the
`freshnessExplicit` field, and `WithFreshnessPolicy` sets it. `Open` calls
`runtimeOptions.resolve` once, after it applies every option and before it
validates them.

`resolve` derives the channel thresholds only when `freshnessExplicit` is
false and the maximum age is above zero. Option order does not change the
outcome, because the resolution runs after the whole option list.

## The derivation

The warning age becomes the maximum age. The critical age keeps the ratio that
the two defaults hold, which is ten hours over six hours, or five over three.

| Source maximum age | Channel warning age | Channel critical age |
| --- | --- | --- |
| 6h, the default | 6h | 10h |
| 7d | 168h | 280h |
| 0 | 6h, the default | 10h, the default |

`DefaultSourceMaxAge` stays at six hours, so the default runtime derives the
same pair that it held before. The default behavior does not change.

The scale divides before it multiplies, so a long age never overflows. A
derivation that cannot order the two thresholds returns the largest
representable duration as the critical age. The pair stays ordered, and
`FreshnessPolicy.Validate` still passes.

The source-check thresholds and the acquisition thresholds do not change.

## Files

| File | Change |
| --- | --- |
| `runtime_policy.go` | `FreshnessPolicy.withChannelMaxAge`, the ratio constants, and the constant comments |
| `runtime_options.go` | The `freshnessExplicit` field, `runtimeOptions.resolve`, and the two option comments |
| `runtime.go` | `Open` calls `resolve` before `validate` |
| `runtime_status_test.go` | `TestSourceMaxAgeSetsTheChannelThresholds` |

`runtime_status.go` needed no change. It reads the resolved policy, so the
derivation reaches every grade through one field.

## Tests

`TestSourceMaxAgeSetsTheChannelThresholds` holds six subtests. Five of them
open a runtime against a stub source, publish one generation at a chosen age,
and read the reported grade. An injected clock keeps every age exact.

| Subtest | Channel age | Options | Grade |
| --- | --- | --- | --- |
| a seven-day maximum age grades a two-day channel current | 2d | `WithSourceMaxAge(7d)` | `current` |
| a channel older than the maximum age warns | 8d | `WithSourceMaxAge(7d)` | `warn` |
| a channel older than the scaled critical age is critical | 281h | `WithSourceMaxAge(7d)` | `critical` |
| a zero maximum age keeps the defaults | 5h, 7h, 11h | `WithSourceMaxAge(0)` | `current`, `warn`, `critical` |
| the default maximum age reproduces the defaults | none | none | the default policy |
| an explicit freshness policy wins | 2d | `WithSourceMaxAge(7d)` and `WithFreshnessPolicy` | `critical` |

The explicit-policy subtest runs both option orders. Each order reports the
critical grade of the explicit policy, which warns at one hour.

The last subtest calls `withChannelMaxAge` with `DefaultSourceMaxAge` and
compares the result against `DefaultFreshnessPolicy()`. It guards the ratio
constants, so a later edit cannot move the default grade.

## Fail before

The removal of the single `resolve` call fails the new test:

```console
$ GOTOOLCHAIN=go1.26.6 go test ./ -run TestSourceMaxAgeSetsTheChannelThresholds -count=1
--- FAIL: TestSourceMaxAgeSetsTheChannelThresholds/a_seven-day_maximum_age_grades_a_two-day_channel_current
    runtime_status_test.go:192: channel freshness = "critical", want "current"
    runtime_status_test.go:196: catalog freshness = "critical", want "current"
--- FAIL: TestSourceMaxAgeSetsTheChannelThresholds/a_channel_older_than_the_maximum_age_warns
    runtime_status_test.go:204: channel freshness = "critical", want "warn"
FAIL
```

The two subtests that hold the defaults keep passing without the call. The
default grade stays exactly where it was, which CAT9.3 requires.

## Commands

| Command | Result |
| --- | --- |
| `GOTOOLCHAIN=go1.26.6 gofmt -l .` | one pre-existing file, `pkg/catalogs/artifact/channel_test.go` |
| `GOTOOLCHAIN=go1.26.6 go test ./ -run 'TestSourceMaxAgeSetsTheChannelThresholds|Freshness|Status' -count=1` | ok, 2 tests and 10 subtests passed |
| `GOTOOLCHAIN=go1.26.6 make test` | exit 0, 71 packages ok, 1190 tests passed, 0 failed |
| `GOTOOLCHAIN=go1.26.6 make lint` | exit 0, golangci-lint 0 issues, ago clean, technical writing PASS on 764 files |
| `GOTOOLCHAIN=go1.26.6 go tool ago -stale-ignores -format json ./...` | exit 0, no finding, no stale ignore, no error |

The `gofmt` finding predates CAT9.3. The file holds one trailing blank line at
the base commit `d2667375`. CAT9.3 owns neither that file nor its package. The
four changed files report no `gofmt` difference.
