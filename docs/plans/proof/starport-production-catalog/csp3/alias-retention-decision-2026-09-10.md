# Canonical rename alias retention

On 2026-09-10, the owner confirmed D30: retain an old canonical ID as an alias until explicit operator removal or replacement baseline removal.
This decision replaces the proposed 30-day automatic expiry.

Starmap owns the alias mapping and its source authority. Starport applies current admission rules to the resolved target.
Retained aliases cannot grant permission, restore unavailable routes, or redirect silently to a different model.
Provider omission and ordinary refresh cannot expire an alias.

CSP3 owns implementation and A08.alias_cycle_scope verification.
Tests must cover time beyond 30 days, restart, explicit removal, and replacement baseline removal.
They must also preserve cycle, ambiguity, operation, and authority checks.
CSP10 owns client protocol transitions through A18.

These requirements refine existing acceptance cases. The plan retains 38 tasks, 50 primary cases, and 324 required subcases.
Implementation, transport, Starport integration, and qualification remain incomplete.
