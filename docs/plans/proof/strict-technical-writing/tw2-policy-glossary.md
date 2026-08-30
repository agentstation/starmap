# TW2 policy and glossary proof

Date: 2026-08-30

Starmap now owns a strict technical-writing policy and a 57-term glossary. The
policy excludes only historical reviews and structured catalog data from prose
lint. Candidate discovery also ignores exact filenames and Go identifiers that
do not belong in the terminology table.

The repository has one `make technical-writing-check` command. Both `lint` and
`check` call it, and `scripts/verify.sh` includes it in repository verification.

## Evidence

- `glossary check` passed with 57 approved terms and no errors.
- `glossary update --check` found no missing candidate terms.
- Config reporting named `.agents/technical-writing.toml` after the user-level
  configuration file.
- Strict lint passed on the policy, glossary, agent instructions, plan index,
  active plan, proof files, verification script, and documentation index.
- A directory scan found no document under `docs/reviews/`.
- The three completed control planes now live in
  `docs/reviews/control-planes/` as historical records.
- The maintained `docs/` scan starts TW3 with 491 strict diagnostics.
