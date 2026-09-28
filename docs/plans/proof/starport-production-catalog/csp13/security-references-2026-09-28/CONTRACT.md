# Captured gateway keys and accounting references

The gateway-key owner validates both hash-index directions and the collection count.
Stored expiration metadata cannot silently expire an authorization record.
Original key expiration and inactive status remain unchanged.
Missing accounts and teams remain diagnostics. A deleted initial key retains its setup marker.

The budget owner validates attempt identities, retained windows, history, correction references, and team initialization receipts.
Completed KV initialization requires the matching consumed SQL grant.
Retained SQL origins can outlive their teams.
A consumed grant without accounting history remains unknown, including an interrupted initialization.

Current account, key, and team policies require matching accounting history.
Missing or different history remains unknown. Verification cannot establish zero consumption or initialize a missing meter.
Uncertain attempts retain their reservations. Verification never settles, refunds, or repeats them.

The CLI reports missing references, held reservations, retained origins, and unknown history counts.
Private snapshot inspection does not open live stores or admit traffic.

The previous verifier accepts a missing API-key hash index in the retained regression.
Final Go 1.27.1 qualification passes 623 race results and 72 focused pure-Go results without failures or skips.
The race suite uses actual Badger, Valkey, SQLite, PostgreSQL, MySQL, filesystem, and object-store fixtures.

Reference checks do not prove aggregate accounting totals or complete correction ancestry.
Independent later history, restore activation, external fencing, native qualification, review, and merge remain open.
Fifteen CSP13 acceptance subcases remain UNVERIFIED.
