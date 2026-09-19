### Catalog acquisition credentials

Provider acquisition reads the checked configuration sources that the loader selects.
It checks `STARPORT_CATALOG_<PROVIDER>_<FIELD>`, `STARMAP_<PROVIDER>_<FIELD>`, `STARPORT_<PROVIDER>_<FIELD>`, then conventional names from the catalog.
An explicit empty or invalid selected value stops fallback.
Inference credentials and account BYOK records remain outside this resolver.

Select a secret source with `STARPORT_CATALOG_<PROVIDER>_<FIELD>_REFERENCE`.
For example, `STARPORT_CATALOG_OPENAI_API_KEY_REFERENCE=env:CATALOG_OPENAI_KEY` selects a deployment environment value.
A `file:` reference selects a private secret file.

Direct secret managers use the Starmap acquisition reference formats.
An unavailable selected reference stops acquisition for that provider.
The corresponding `_REFERENCE_FALLBACK_AMBIENT=true` setting permits fallback only when the source reports no configured value.
It does not permit fallback after authentication, validation, or revocation errors.

Persistent installations retain policy records under `<state root>/credentials/catalog/<instance ID>`.
The file inventory reports this directory as `credential-policy` with owner-only access.
On upgrade, different old and new credential profiles cause a provider-specific conflict.
An explicit catalog reference or removal of the conflicting variable resolves that choice.
The resolver retains acceptance across restart without storing credential values in policy records.
Development sessions use the current policy without persistent policy history.

