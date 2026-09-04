# CAT9.1 proof: the Starport operator documentation

CAT9.1 owns the Starport topology guide, the catalog configuration reference,
the README paragraph, and the environment example. The work lives in the
Starport repository, not in Starmap. This file records where the work is, what
the owner review found, and what the gates reported.

CAT9 and CAT9.2 own the Starmap text. CAT9.3 owns the source maximum age
wiring that this review found.

## Location

The work starts from the CAT8 head `9081ca4` on the Starport branch
`worktree-agent-af98dedf087bbebc4`. The owner moved the four commits onto the
CAT8.1 branch `worktree-agent-ad6e217c4e16a7520`, where they sit above
`8da2c6c`. The full proof is `docs/proof/catalog-publisher/cat9.1.md` at that
branch.

| Original | Moved | Subject |
| --- | --- | --- |
| `1a74aca` | `47991d8` | docs(catalog): add the Starport deployment topology guide |
| `404c07b` | `1f66327` | docs(catalog): document every canonical catalog setting |
| `4949296` | `4fc4929` | docs(proof): record the CAT9.1 operator documentation |
| `4d608ff` | `d75dbe2` | docs(catalog): correct seven catalog claims against the code |

## Owner review

The review at `4949296` traced every setting claim to the Starmap and
Starport code and returned seven factual defects. The agent repaired every one
in `4d608ff`.

1. The `starmap` and `file` kinds require `SOURCE_URL`. The `github` kind
   does not.
2. `SIGNER_WORKFLOW` defaults to `.github/workflows/catalog-generation.yaml`.
3. `require_source` reads the source once at open and fails startup when that
   read fails. It never waits.
4. `SOURCE_MAX_AGE` moves no freshness threshold today. The thresholds stay
   at six hours for `warn` and ten hours for `critical`.
5. The workspace wording named the wrong layer.
6. `STATE_DIR` holds the retained layer store, the instance identity seed, and
   the source discovery record.
7. `REFRESH_TIMEOUT` of zero adds no cap.

The fourth defect is a product gap. The runtime validates the source maximum
age and never reads it again. CAT9.3 owns the wiring in Starmap.

## Gates at `d75dbe2`

| Command | Result |
| --- | --- |
| `bash scripts/verify-doc-links.sh` | pass |
| `bash scripts/verify-developer-experience.sh` | 47 passed, 0 failed |
| `bash scripts/verify-readme-quickstart.sh` | pass |
| `technical-writing lint docs/DEPLOYMENT-TOPOLOGIES.md` | 0 diagnostics |
| `technical-writing lint docs/OPERATOR-GUIDE.md` | 48 diagnostics, the same count as the base |

Every remaining operator guide diagnostic sits outside the new catalog
section.

## Verifier

The run uses `scripts/verify-catalog-distribution.sh` at plan head `7129b237`.
`CATALOG_DISTRIBUTION_STARPORT_ROOT` points at the Starport worktree at
`d75dbe2`. The result is 68 passed, 0 failed, and 0 unverified of 68. CAT-V56,
CAT-V57, and CAT-V58 pass.
