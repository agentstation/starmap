# Final published pair

Starmap PR185 contains `b46c077a7`. Its complete Sol/Opus review reports no actionable findings.
All 122 fleet, acquisition, and pin race results pass. Full repository verification passes.
Native CI run 36282930189 remains active.

Starport commit `76a5687c6` selects the published Starmap module
`v0.16.6-0.20260927002942-b46c077a7039` without a local replacement.
All 34 repository commands pass with `GOWORK=off`.
All eleven selected CSP11 subcases pass with real Valkey and PostgreSQL.
This evidence does not qualify the other 48 primary plan cases.

The complete consumer review found two CSP11 defects beyond the prior acceptance coverage.
Both now have failing real-store regressions. The earlier passes remain historical evidence and do not clear those defects.
The consumer review disposition and repair scope are in `../consumer-review-rescope/REVIEW_RESOLUTION.md`.
Both PRs remain unmerged. Consumer publication awaits the witness contract repair and renewed qualification.
