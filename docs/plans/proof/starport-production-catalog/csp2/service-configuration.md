# CSP2 service-managed primary configuration

The Starmap implementation passes local checks. Native Windows execution and the complete CSP2 qualification remain UNVERIFIED.
Starport adoption remains under CSP8. No primary acceptance case gains credit.

Work commit: `707d63db` on `codex/catalog-requirements`.
The [owner decision](owner-decisions-2026-09-06.md) permits administrator-owned primary inputs without widening private state access.

## Configuration contract

Select an explicit `--config` or `CONFIG` path and `--config-access=service-managed` or `STARMAP_CONFIG_ACCESS=service-managed`.
The flag overrides the environment. The default remains `owner-only`.
YAML contents cannot select their own access policy. Missing explicit files fail before parsing.

POSIX files must belong to root or the effective user and forbid group and other writes.
Windows files must have a trusted owner and forbid mutation grants to untrusted principals.
The trusted Windows set contains the process account, SYSTEM, Administrators, and TrustedInstaller.
Shared read grants remain valid. The bounded read must still succeed under the process identity.

The selected policy governs startup, migration configuration rereads, and file reporting.
The shared record reader preserves the 1 MiB input limit and checks file identity around the read.
Both policies check the selected symlink route and its target ancestors. Neither policy repairs the input.
Dotenv files and catalog state retain owner-only access.

Diagnostics classify policy conflicts but retain uncertainty about effective access and native ACLs they cannot establish.
Windows observations retain separate private and service-policy results. A shared read grant no longer appears as a service-policy conflict.

## Verification

The [fail-before capture](service-configuration/fail-before.log) shows that the original reader rejected an explicitly selected `0640` primary file.
The first corrected test used the wrong canonical settings-map key. The next fixture incorrectly paired an API key with an embedded source.
Those fixture errors remain in the first and second focused captures. The corrections preserve the access assertions and use valid catalog settings.

The application, file-access, path, and policy suite passes 394 race results across five packages.
The final focused suite passes 42 results across four packages, including migration reread and diagnostic regressions.
The shared runtime, storage, bootstrap, and root-library suite passes 686 race results across four packages.

The native Linux test runs in an isolated ARM64 Ubuntu container. Its child drops to UID and GID 65534.
It reads root-owned `0640` configuration through the file group. It rejects unreadable, shared-writable, and foreign-owned fixtures.
The original private reader rejects all four fixtures. The native CI workflow now runs this owner test on Linux.

Windows AMD64 compilation passes for the application and product-path tests. The retained build record contains both executable digests.
Compilation does not establish native ACL behavior. Native Windows administrator-owned and denied-read qualification remain open.

All six consumer compositions pass their existing dependency budgets and forbidden-dependency checks.
Final code lint reports zero issues. Ago reports no findings, stale ignores, or incomplete errors.
The initial code lint found a private reader wrapper with no production caller. The default path now calls that reader and retains its role check.

The workflow parses successfully. The final maintained-prose check passes after adding the ACL glossary definition and correcting paragraph boundaries.

The [verification manifest](service-configuration/verification.json) binds source and retained captures.
Required pre-PR review remains pending. These checks do not qualify the released Starmap and Starport pair.

## Native policy references

Linux maps the POSIX ACL mask to the group permission bits. This bounds named-user and group writes under the selected mode policy.
See the [Linux ACL contract](https://www.man7.org/linux/man-pages/man5/acl.5.html).

The macOS checker distinguishes read rights from mutation rights.
It uses the [Apple vnode rights](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/kauth.h).
The Windows checker uses the [Microsoft file-access rights](https://learn.microsoft.com/en-us/windows/win32/fileio/file-access-rights-constants).
Unknown rights do not gain an implicit read-only classification.
