# Pinned artifact consumer

This external module verifies a fixed catalog archive with a checked-in SHA-256 trust root.
It loads a fixed predecessor from caller-selected storage, activates the verified generation, and checks duplicate activation and restart.
Negative tests reject a different trust root and altered archive bytes.

The fixture contains one authored model, `fixture/pinned`, and no provider credentials.
Its archive and detached files are independent of the current embedded catalog and toolchain gzip output.
Base64 text stores the fixed archive bytes. A test compares its payload and manifest with the inspectable JSON source files.
Go 1.26.6 and `artifact.Build` produced the schema 8 fixture on 2026-09-10.
A fixture update requires a deliberate archive and trust-root change.

Run `make test-consumer-deps` from the repository root to verify all external compositions and dependency limits.
