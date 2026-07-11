# Legacy catalog format v0

This fixture freezes the directory format shipped before transactional catalog
generations:

- `providers.yaml` and `authors.yaml` contain record indexes;
- provider models live at `providers/<provider>/models/<model>.yaml`;
- author model views live at `authors/<author>/models/<model>.yaml`;
- `provenance.yaml` is optional.

The format has no generation ID, sync-run ID, generated timestamp, validation
report, payload checksum, or source-observation identity. Migration callers must
supply those values explicitly through `LegacyMigrationOptions`; the migrator
never invents time or identity metadata and never mutates this source directory.
