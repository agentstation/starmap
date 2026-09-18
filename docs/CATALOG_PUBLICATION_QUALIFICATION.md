# Catalog publication qualification

CSP6 requires a checked bot merge, publication recovery tests, and a hosted retry that preserves all five immutable assets.
The qualifier also verifies both channel attestations, checkpoint replay, and the current embedded catalog.
Missing evidence reports `UNVERIFIED`.

Use a clean checkout of the promoted source. The verifier permits later planning and design evidence changes.
Changes to code, tests, tools, or embedded catalog input require new qualification.

Capture the successful publication before its retry:

```sh
python3 scripts/catalog_publication_capture.py before \
  --directory docs/plans/proof/starport-production-catalog/csp6/hosted-publication \
  --pending "$PENDING_RECORD" --pull "$PROMOTION_PR" \
  --run "$COMPLETION_RUN" --attempt "$COMPLETION_ATTEMPT"
```

`PENDING_RECORD` is the retained `pending.json` from the preparation run.
Use a completion run that published both channels. A successful run that only awaits checks does not qualify.
The capture records required check identities and their completion before the bot merge.
It preserves release metadata and original channel bytes.

Rerun the original preparation workflow through GitHub Actions. That retry restores the run's retained publication inputs.
A new manual dispatch can build a new catalog and does not prove an identical-publication retry.
Wait for the retry to complete, then capture its exact run attempt:

```sh
python3 scripts/catalog_publication_capture.py after \
  --directory docs/plans/proof/starport-production-catalog/csp6/hosted-publication \
  --run "$RETRY_RUN" --attempt "$RETRY_ATTEMPT"

bash scripts/verify-catalog-product.sh --task CSP6 \
  --starport-root "$CSP_STARPORT_WORKTREE" --json
```

Qualification requires GitHub read access and Go 1.27.1.
It downloads the immutable assets and verifies five attestations.
It restores the checkpoint and compares the artifact with the current embedding.
Attestations must identify the expected publisher source and run. Later channel updates do not invalidate the captured, attested channel bytes.

The local recovery adapter injects failures into simulated GitHub transport while using real catalog tools and Git.
Those tests establish transition recovery. They cannot replace hosted publication evidence.
Both adapters must pass before CSP6 is complete.
