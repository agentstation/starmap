# Acquisition credential preparation

This read-only inspection records the next credential task. CSP7 remains unstarted and depends on CSP1 and CSP3.
The inspected Starmap source is `a0cfc63e9b1c81702b2197f33dcfe11251a52973`.

`internal/auth/resolver.go:resolveAmbientField` tries catalog-declared conventional names before the derived `STARMAP` name.
It skips explicit empty values and can select another candidate or a field default.
Section 7.2 of the engineering specification requires the derived name first and terminal explicit-empty selection.
The change also needs the specified migration comparison before persistent installations activate a different credential identity.

The resolver already has reference policies, environment and file sources, direct secret backends, cloud chains, cached source material, and coalesced reads.
Preserve those contracts. Verify complete-version rotation and terminal reference failures before changing the selection order.

`internal/auth/types.go:ProfileDetails` reports the selected profile and primitive. It does not report selected field origins.
CSP7 must report origins without values and redact sensitive reference paths.
No credential resolution or secret lookup ran during this inspection.

CSP4 precedes CSP7 in the ledger. The active CSP3 work and required CI remain unchanged.
