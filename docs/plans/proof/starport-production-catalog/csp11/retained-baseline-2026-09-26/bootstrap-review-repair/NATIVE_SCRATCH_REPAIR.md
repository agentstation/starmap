# Native scratch creation repair

Both Windows archive jobs fail on consumer `aab7dd7c` before development readiness.
The generated console is present. Startup fails because `os.MkdirTemp` inherits a DACL that permits access beyond the private-directory policy.
The current code then calls `productfiles.ExistingDirectory`, which correctly refuses that ACL.

CSP11 final-pair qualification owns this integration repair. The native private-file concept remains in Starmap.
Add `productfiles.CreateDirectory(path)` for exclusive creation beneath an existing trusted parent.
It returns a bound private directory and an existence error for any pre-existing target.
It must preserve existing directories, files, links, permissions, and contents.
Creation uses the existing native primitive through an open parent handle, with parent identity and ancestor validation.

Windows sets the protected private DACL during creation. POSIX creates mode 0700.
Failure after creation preserves the empty private entry rather than deleting an uncertain path.

Starport generates a bounded random scratch name and calls this API before writing session records.
Its direct-child scratch layout, recovery schema, exclusive session lock, and identity-bound cleanup remain unchanged.
No permission repair applies to an existing directory. No access-policy relaxation or fallback applies.

Required evidence includes exclusive-creation collisions, trusted shared-read parents, native private child access, and scratch lifecycle tests.
Preserve both failed Windows logs as before evidence. Qualify the candidate binaries on Windows amd64 and arm64 after the repair.
The module must publish before consumer qualification. Repeat required checks and review for the changed commits.
