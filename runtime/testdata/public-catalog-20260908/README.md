# Signed public catalog fixture

These files come from the [September 8 catalog release](https://github.com/agentstation/starmap/releases/tag/catalog-dcc4e4539c74a181c1789a8f275bf16893ce9698ea9644bf18a89cf10a067571).
The capture record lists the source commits and file checksums.
The archive retains its published bytes and schema 6 payload. Current Starmap supports this legacy schema.

Run `python3 scripts/prepare_public_catalog_fixture.py` from the repository root before public-catalog acceptance tests.
The explicit setup downloads the immutable archive and checks its captured size and checksum. It reuses a verified local copy.
The archive stays outside version control. The tests embed the prepared archive.
Their catalog requests stay on the local HTTP fixture.

Ordinary tests skip these cases when the archive is absent. The product verifier treats those skipped cases as UNVERIFIED.
CI prepares the archive first and requires it through `STARMAP_PUBLIC_FIXTURE_REQUIRED=1`.
The setup refuses an invalid local copy. Remove that test artifact before preparing a replacement.

The channel documents carry sequences 19 and 18. Each document has its own published Sigstore bundle.
The archive bundle binds the archive digest to the Starmap catalog workflow.
GitHub CLI verified all three bundles during capture on September 11, 2026.

The tests serve these files through a local HTTP fixture.
Starmap uses its default signature verifier, compiled trust root, and publisher policy.
The fixture checks that catalog requests carry no credentials.

The rejection tests damage channel and archive signatures, archive bytes, and reported asset sizes.
They also serve the signed previous channel to test replay rejection.
Each rejection preserves the accepted catalog. A restart restores that catalog while the source is unavailable.

The artifact package separately checks unsupported schema refusal before it returns a generation.
The fixture does not create signing keys or alter the compiled trust policy.
