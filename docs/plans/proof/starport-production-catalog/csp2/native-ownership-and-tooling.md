# Native Linux tooling and workspace ownership

The native tooling rerun passes all 72 Models.dev test events without failures or skips.
The workspace ownership fixture passes 12 scenarios across atomic and journaled replacement.
Its 15 passing events include the two mode groups and their parent test. No test skips occurred.
These checks extend CSP2 component evidence. They do not qualify the released product pair.

## Tooling environment

The [tooling image record](native-linux-tooling-image.json) binds the local image, package versions, and build commands.
The image adds Bash, Git, curl, and CA certificates to a pinned Ubuntu base.
The build used an empty context. It copied no repository files or credentials into the image.
Tests ran without network access, as UID/GID 65532, with all capabilities removed and the root filesystem read-only.

The [tooling rerun](native-linux-modelsdev-tooling/internal-sources-modelsdev.json) records all 72 passing events and the test binary digest.
It closes the three missing-executable failures recorded by the earlier minimal-image run.
The container exited successfully, and the runner removed its container and temporary volume.

## Ownership contract

`TestWorkspaceForeignOwnership` runs only with the `starmap_ownership_test` build tag and the explicit fixture selector.
The fixture uses a private Docker volume. The host repository and executable mounts are read-only.

The parent has only CHOWN, FOWNER, DAC_OVERRIDE, SETUID, and SETGID capabilities.
Every service child verifies UID/GID 65532 and zero effective capabilities before it calls workspace operations.
Children also verify their supplementary groups. The fixture never changes ownership in the host repository.

The twelve scenarios cover foreign file ownership, file group, root ownership, root group, managed-directory ownership, and an operator note.
Each scenario runs through both atomic and forced journaled replacement.
An unprivileged child first creates the workspace and its coordination files.
The fixture parent then assigns UID or GID 65533 to the selected entry.

Without the required ownership rights, replacement must return EPERM and preserve the original tree identity, content, and access digest.
The fixture also verifies the old catalog and completed replacement state after refusal.
For the group scenarios, a child with supplementary group 65533 successfully publishes an intermediate catalog without capabilities.
The authorized parent then publishes the final catalog and preserves every retained entry's ownership, mode, and native access digest.
The operator note retains its original bytes.

## Application verification

The [verification record](foreign-ownership-checks.json) reports 639 passing macOS race events across bootstrap, application, runtime, and workspace packages.
There were no failures or skips. All recorded workspace, module, and runner input hashes remained unchanged during verification.
Whole-module ago and Linux ARM64 vet with the ownership test tag also passed.

## Evidence and limits

The [initial run](native-linux-foreign-ownership-initial/internal-catalog-workspace.json) failed during fixture setup.
Go's nested temporary directory had an inaccessible parent, so the service child could not create the workspace.
The fixture now creates one temporary directory directly beneath the isolated volume and gives that directory to the service account.
This was a fixture defect. It did not establish a production failure.

The [ownership run](native-linux-foreign-ownership/internal-catalog-workspace.json) passed all 15 events.
The [final group-aware run](native-linux-foreign-ownership-groups/internal-catalog-workspace.json) also passed all 15 events.
Both records include the commands, binary digests, terminal container state, and successful container and volume removal.
The fixture found no production defect and changed no runtime policy.

Reproduce the final fixture from the Starmap worktree:

```sh
python3 docs/plans/proof/starport-production-catalog/csp2/native-linux-suites.py \
  --workspace-ownership --output /tmp/starmap-ownership-proof
```

The runner requires the cached native Linux ARM64 image identified in its source.
These executions use CGO-disabled binaries without race instrumentation. They do not simulate power loss or qualify other filesystems.

Foreign-owner behavior on macOS and Windows remains UNVERIFIED. Native Windows execution, service configuration, and legacy journal procedures remain open.
CSP5 owns abandoned preparation directories and errors after publication. CSP8 owns Starport adoption, and CSP12 owns engine qualification.

The [task gate](native-ownership-documents-and-task.json) retains two passing subcases and twenty UNVERIFIED subcases.
All 50 primary cases remain UNVERIFIED. The unchanged acceptance roster still requires released-pair qualification.
