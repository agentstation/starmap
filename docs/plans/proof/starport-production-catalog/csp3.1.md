# CSP3.1 offering lookup allocations

CSP3.1 passes its local acceptance criteria. The two assigned A44 subcases pass in the product verifier.
The complete A44 case and released-pair acceptance remain UNVERIFIED. Publication still requires the pending pre-PR review.

`Catalog.Offering` now resolves provider identity through an immutable index, then copies only the selected offering.
Both catalog constructors build the index after identity validation. Canonical IDs and aliases retain the same missing-record errors.
Empty providers remain distinguishable from unknown providers. Observation catalogs still expose no consumer offerings.

The [allocation regression](csp3.1/allocation-red.jsonl) failed before the correction.
Lookup allocations grew from 31 with one model to 16,019 with 1,000 models for both canonical and alias identities.
The new regression requires allocations to remain independent of unrelated provider models.

Canonical lookup measurements below are medians of three runs on the local macOS host:

| Models in provider | Time before / after | Bytes before / after | Allocations before / after |
| --- | --- | --- | --- |
| 1 | 1.558 µs / 0.565 µs | 3,264 / 1,104 | 31 / 11 |
| 100 | 80.577 µs / 0.570 µs | 174,457 / 1,104 | 1,617 / 11 |
| 10,000 | 10.767 ms / 0.608 µs | 17,398,242 / 1,104 | 160,047 / 11 |

Alias lookup at 10,000 models fell from 10.618 ms to 0.603 µs, with the same allocation reduction.
The API still copies returned mutable values. These measurements cover one catalog lookup, not total Starport request overhead.
The [verification record](csp3.1/verification.json) includes all six benchmark cases, exact artifact digests, and source inputs.

| Check | Result |
| --- | --- |
| Catalog package race tests | 843 test events across nine package suites pass. |
| Identity and ownership checks | Canonical and alias equality, missing records, empty providers, and concurrent mutations of returned values pass. |
| Full catalog benchmark command | Passes with allocation reporting. |
| A44 task checks | `A44.canonical_alias_lookup` and `A44.caller_owned_values` pass. |
| Verifier tests | All 30 tests pass. |
| Repository lint and ago | Pass without findings or incomplete errors. |
| Generated documentation | Passes after regenerating source links in the catalog package README. |

The first fixture lacked its authored model's primary author. The corrected fixture declared its primary author before the allocation regression ran.
The documentation check initially found shifted source links. Its original output remains beside the corrected check.
CSP10.1 owns Starport integration and its remaining A44 subcases. CSP0.4 and CSP10.1 retain the full-request performance contract.
