# CSP2 native Windows service qualification fixture

Commit `fb54f39e` adds `TestServiceConfigurationWindowsAdministratorOwnerAndDeniedRead` to the existing native CI suite.
Windows AMD64 and ARM64 compilation pass. Native execution remains UNVERIFIED, with zero new native test results.

The fixture assigns its temporary configuration file to the Administrators group. The service reader must return the original bytes.
The owner-only reader must refuse that different owner. A deny entry for file data must then prevent the service read.
After explicit permission correction, the service reader must return the unchanged file again.

Metadata inspection must observe the administrator owner and classify the service descriptor as compatible.
It must report the private-policy conflict and retain an unverified effective-access status, including when the data read fails.
This tests the difference between policy compatibility and actual read access.

The test changes only its temporary fixture. Cleanup restores the original owner and permissions.
A local account that cannot assign administrator ownership skips the fixture with an explicit reason.
GitHub Actions must fail in that situation instead of silently skipping the owner check.

The [verification record](windows-service-qualification/verification.json) binds source, workflow, compiled artifacts, and static-check captures.
Windows-targeted lint reports zero issues. Ago reports no findings, stale ignores, or errors.
The maintained-prose check passes 1,137 files. No product behavior changed in this commit.

The native Windows matrix must still run after required review and publication. No primary acceptance case gains credit.
