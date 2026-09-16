# Private files for host applications

`productfiles` exposes the native private-file operations that Starmap uses.
Starport can use these operations without copying filesystem access code.
The package does no catalog acquisition and starts no background work.

The host selects its absolute paths through `productpaths` or explicit settings.
`NewDirectory` creates missing private directories and validates existing access.
`ExistingDirectory` validates an existing directory without creating paths.
Neither operation repairs existing permissions.

```go
directory, err := productfiles.NewDirectory(configDirectory)
if err != nil {
    return err
}
// A nil previous value requires an absent file.
return directory.CompareAndPublish(ctx, "config.env", nil, configBytes)
```

Each directory binding retains its original filesystem identity.
An operation refuses a replaced directory, an unsafe ancestor, or invalid native access.
Linux and macOS use ownership, modes, and the applicable ACL checks.
Windows uses the native owner and DACL checks.
New files have private access from creation.

`ReadFile` requires a byte limit and reads one regular private file.
`CompareAndPublish` requires matching previous bytes before it replaces a file.
A nil previous value means absence. An empty, non-nil value means an empty file.
`CompareAndRemove` also requires matching bytes and allows retries after removal.

Record publication reserves `.record-publications` for its writer lock and recovery receipts.
Writes and recovery preserve unknown entries and changed files.
`RecoverPublications` recovers abandoned record writes under the same lock.
It does not recover an application transaction across config, data, or state roots.
Record writes have a 64 MiB bound.

An `errors.PublicationError` means the change became visible, but durability remains unconfirmed.
The caller must retain the intended bytes and resolve that result before a different write.
It must not assume that the old file remains selected.

`Open` returns a checked `os.Root` that the caller must close.
Raw operations on that root follow the standard library contract.
They do not inherit the record publication or recovery contract.

`PublishDirectory` moves a direct child between open roots on the same filesystem.
It refuses any existing destination, including an empty directory.
The caller must sync staged contents before the move and both parent directories afterward.
The caller also owns recovery if its operation spans multiple roots.
`SyncDirectory` reports native synchronization errors without claiming hardware power-loss qualification.
