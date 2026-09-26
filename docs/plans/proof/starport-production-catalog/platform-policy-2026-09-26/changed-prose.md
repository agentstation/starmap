Supported targets are macOS on Apple silicon, Linux on x86-64 and ARM64, and Windows on x86-64 and ARM64.
New releases do not support Intel Macs. Historical Intel Mac archives remain available.


2. builds Linux and Windows archives for amd64 and arm64, plus macOS archives for arm64, with `CGO_ENABLED=0`.
3. verifies cgo-disabled build metadata for all five binaries. It also verifies

Publication recovery, ingestion, and real Git acquisition run in separate native jobs on all five supported targets.
The verification job and all five native jobs require these tools. The

Promotion also requires successful native jobs for Ubuntu x64 and ARM, macOS ARM, and Windows x64 and ARM.

The command uses `gh` to read workflow metadata and download all five native artifacts.

| D39 | Both products support macOS only on Apple silicon. Linux and Windows retain x86-64 and ARM64 support. Preserve historical Intel Mac releases and evidence. | User confirmed on 2026-09-26. |

## Supported operating systems and processors

D39 defines five targets for both products:

| Operating system | Supported processors |
|---|---|
| macOS | Apple silicon (`arm64`) |
| Linux | x86-64 (`amd64`) and ARM64 (`arm64`) |
| Windows | x86-64 (`amd64`) and ARM64 (`arm64`) |

CI, native qualification, release archives, and installers must use this matrix.
New releases must omit `darwin/amd64`. Homebrew must reject Intel Macs without excluding Linux x86-64.
The catalog publisher must not require a retired Intel Mac check.
Keep race detection on supported targets where Go provides it. Preserve all distinct storage, recovery, pure-Go, capacity, and performance checks.

Historical Intel Mac releases and test evidence remain available. They do not define the current support boundary.
Go 1.27.1 remains the exact toolchain for every supported target.


## Apple silicon support decision, September 26

D39 removes Intel Mac support from both products. The previous release matrices included six targets, including `darwin/amd64`.
The revised matrices retain five targets. Linux and Windows retain both x86-64 and ARM64.
The implementation changes CI, release verification, native evidence selection, Homebrew requirements, and publisher promotion checks.

Historical six-target qualification remains evidence for its recorded commits. It does not require future Intel Mac runs.
The current plan records pending review, CI, and merge evidence.




Supported targets are macOS on Apple silicon, Linux on x86-64 and ARM64, and Windows on x86-64 and ARM64.
New releases do not support Intel Macs. Historical Intel Mac archives remain available.


The snapshot checks five target binaries, five archives, five SBOMs, shell

TypeScript, and Go SDKs. It also verifies five static binaries, five archives,
