# CSP4 Starport consumer boundary

This inspection preserves the existing eight CSP4 task checks.
The Starmap source is `44a400a481748c5e7ae1268cecb63b260a91bdab`.
The Starport source is `d45bb30ce13774ca7a25e206a6c9a6168bbe8548`.
It provides source evidence, not consumer acceptance results.

## Existing composition

Starport already composes Starmap through `internal/catalog/runtime.go` and `internal/catalog/settings.go`.
It supplies generation stores, a candidate store, leases, source settings, and a separate acquisition credential resolver.
`internal/config/catalog.go` accepts only `prefer_source` and `require_source` as startup policy values.
Its settings translation does not supply authority identity, policy identity, or qualified clock evidence.
The current application therefore cannot select the new authority policy through its supported configuration.

## Required consumer checks

The acceptance map assigns four Starport checks to CSP4 as well as CSP10.
Starmap library tests cannot satisfy these consumer checks.

| CSP4 check | Consumer evidence |
| --- | --- |
| `A09.excluded_membership` | Starport preserves internal membership exclusions through its accepted catalog. |
| `A10.cold_owner_refusal` | A cold Starport owner refuses service without accepted authority permission. |
| `A10.warm_retained_authority` | Starport can use retained internal state only while the receipt and qualified clock permit it. |
| `A21.mixed_schema_permission_envelope` | Starport processes the permission requirement independently of catalog compatibility. |

CSP4 also owns all four A07 publication checks.
CSP10 owns the complete A09, A10, and A21 cases and the other gateway checks in its task roster.
Those later checks include every new route attempt, cache delivery, retry, and queued batch line.
The existing required subcases and their final qualification owners remain unchanged.

## Delivery sequence

First qualify, review, and merge the current Starmap subscriber delivery.
Then complete the remaining Starmap authority work in its own reviewed PR.
A separate Starport PR must pin a compatible published Starmap module and qualify the four consumer checks.
The owner must approve the module release under the plan's existing authority boundary.

The earlier two-PR description could not cover two Starmap deliveries and a separate Starport repository change.
The plan now permits that separate consumer PR without dropping checks or claiming task completion after a Starmap-only merge.
All eight CSP4 checks and the required implementation merges must precede CSP4 completion.
