# Constructor network probe

Run `python3 scripts/constructor_network.py` from the repository to verify passive library construction.
The command requires Go and a Linux AMD64 or ARM64 Docker engine with seccomp support.
It builds the current source with Go `1.25.12`, without cgo, for the engine's native architecture.
The temporary image starts from `scratch` and contains only the probe binary.

The probe calls `New`, `NewContext`, and `NewContext` with memory or empty filesystem storage.
Each constructor must return the same nonempty embedded catalog and create no files in its writable home.
The container receives synthetic source settings and a dummy provider key. It receives no host credentials.

The container uses Docker's `none` network driver and a temporary seccomp profile.
The profile terminates the process on socket operations or network access through `io_uring`.
A positive control announces its socket attempt and must terminate with `SIGSYS`.
An ordinary network error, missing result, resource failure, or missing Docker engine cannot qualify the check.

The report includes command results, container states, the binary hash, toolchain details, and cleanup results.
The command removes its containers and image after both controls, including after failures.
Docker can retain its normal build cache. The command does not remove unrelated cache entries.

This probe supplies Linux component evidence for `A02.constructor_network_silence`.
It does not qualify a release or test remote storage reads.
Caller-supplied remote storage can read over the network under the approved constructor contract.
The separate worker tests cover delayed background activity. Application startup and cold offline persistence have separate acceptance cases.

The temporary seccomp profile belongs only to this verification process.
Docker documents the [custom profile mechanism](https://docs.docker.com/engine/security/seccomp/)
and the [none network driver](https://docs.docker.com/engine/network/drivers/none/).
The [OCI specification](https://github.com/opencontainers/runtime-spec/blob/main/config-linux.md#seccomp)
defines `SCMP_ACT_KILL_PROCESS`.

The Pull Request workflow includes a separate Ubuntu job for this probe.
It retains the complete JSON report as `constructor-network-linux-amd64`.
