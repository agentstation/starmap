All documents, artifacts, and screenshots are reviewed, plus spot-checks of the Starport budget middleware, config package, deployment topologies doc, both READMEs, AGENTS.md, and glossaries. Here is the complete review.

---

# Fable review: Starmap and Starport catalog lifecycle design and plan

Reviewed on 2026-09-04 against the Starmap worktree (`codex/catalog-requirements`, planning artifacts uncommitted) and the Starport worktree (`codex/catalog-lifecycle`, clean). Baselines: Starmap `4780dfea`, Starport `042eb978`.

## 1. Verdict

The product direction is right and the documents are unusually rigorous. The ownership split (Starmap owns facts, Starport owns inference), the "catalog works before credentials" first-use moment, the membership-versus-routability distinction, and the refusal to let company size select a storage engine are all correct product calls. The repository findings and audit are honest about what is proven versus proposed, which is rare and valuable.

The documents are **not ready for activation as a single campaign**, for three reasons:

1. **Sequencing buries the highest-leverage user value.** The README rewrite, demo, and docs readability fixes — the cheapest, most product-changing work — sit in phase 5 behind the entire configuration, storage, and fleet architecture campaign. The current README actively contradicts the confirmed product moment (it demands a provider key before `starport dev`), and that stays shipped until ~20 tasks complete.
2. **Six contracts must settle before implementation** — the audit's AUD01–AUD06 (I agree with all of them), plus two new gaps found here: the inference-credential precedence flip has no defined migration behavior, and there is no client-facing model-identity lifecycle contract (renames, aliases, removals).
3. **Several proposals should shrink.** D6 baseline materialization, the D12 local-edit API surface, the fourth update-control knob, Redis/MySQL alternative qualification runs, and the numeric GIF release gates all add cost without a named user who needs them in v1.

The PRD and spec are close to implementation-ready for the catalog/authority core (sections 2–6, 8, 9). The configuration-management sections (7.5, 7.7) and the plan's phase structure need revision first.

## 2. Prioritized findings table

New findings only; positions on the prior audit are in section 3.6. Personas: LD = local developer, SU = startup, EN = enterprise operator.

| ID | Severity | Persona | Location | User consequence |
| --- | --- | --- | --- | --- |
| FBL-01 | High | LD, SU | Plan phases (plan.html:79–88), CSP18/CSP20 dependencies (plan.html:322, 350) | The broken first-use story (key-before-catalog README, tour-only GIF, 14 px auth-gated docs) stays shipped for the entire architecture campaign; phase exits measure case counts, not user increments. |
| FBL-02 | High | SU, LD | Spec §7.2 (ENGINEERING_SPEC.md:419–439) vs current behavior (F08; starport `internal/config/README.md:70–74`) | The target precedence flips conventional-name-first to STARPORT-first for inference. A user with both `OPENAI_API_KEY` and `STARPORT_OPENAI_API_KEY` set silently changes which key pays after upgrade. The spec forbids this outcome but defines no mechanism. |
| FBL-03 | High | SU | PRD.md:126–146; spec §10 (892–922) | No contract for model-ID renames, alias retirement, or removal timing across generations. Clients hardcode model IDs; behavior when an ID disappears mid-fleet is undefined, so a routine catalog update can break production callers unpredictably. |
| FBL-04 | Medium | All | Spec §10.4 (1071–1074), A38 (1128), demo brief | A 33-second or 48-second excellent demo fails the 39-case release gate. Numeric duration bounds in release acceptance turn editorial judgment into a false blocker. |
| FBL-05 | Medium | EN | D8 (PRD.md:27), spec §10.2 (995–998), CSP23 (plan.html:395–409, and "no hosted sites" at line 95) | The public docs site with stable version URLs is a confirmed promise with no named hosting mechanism, infrastructure task, or operational owner. |
| FBL-06 | Medium | All | D6 (PRD.md:26), spec §3.1 (84–118) | Persisting the embedded baseline payload on every fresh start adds idempotency, CAS-seeding, migration, and "not-a-second-authority" complexity, while the same bytes are always available in the binary. No named user problem requires the payload copy. |
| FBL-07 | Medium | LD, SU | Spec §6.2 (355–393) | Four overlapping update-control knobs (`SOURCE_REFRESH_MODE`, `ACQUISITION_ENABLED`, `NETWORK_MODE`, pin) plus startup policy. `SOURCE_REFRESH_MODE=manual` is the only one without a named persona need; every §6.2 intent is reachable with the other three. |
| FBL-08 | Medium | EN, LD | D11/D12 (PRD.md:30–31), spec §7.5 and §7.7 (532–653) | The local-edit configuration API (revisions, drafts, ETags, idempotency keys, atomic activation, PATCH) is a product inside the product. Most operator value is in the read-only effective report, validation, and diff export. |
| FBL-09 | Low | Contributors, EN | D3 (PRD.md:22), spec §5 (246–287), AUD09 | *Labeled challenge to a confirmed decision.* Four-hour default-branch promotion costs serialized bot PRs, gate identity work, revalidation, and ~6 bot commits/day for a modest benefit: fresh checkouts at most 4 h stale instead of at most one promotion cycle stale. Runtime consumers get updates via the channel either way. |
| FBL-10 | Medium | All | CSP2 acceptance ("Native qualification remains in CSP22", plan.html:135) vs CSP22 | The path/migration contract ships in phase 1 but Windows/Linux native evidence arrives only at final qualification. A Windows migration defect found at CSP22 forces rework across the whole campaign. |
| FBL-11 | Low | LD | Starport README.md:158–160; demo brief:63–67 | The local-provider (Ollama) path — the only genuinely credential-free inference story — requires hand-editing a Starmap workspace and setting `STARPORT_CATALOG_WORKSPACE_PATH`. The weakest step in the strongest local-dev narrative. |

## 3. Detailed findings

### FBL-01 · Plan ordering defers the product's first impression to last

**Evidence.** The current README requires `export OPENAI_API_KEY` *before* `starport dev` (starport `README.md:70–74`), inverting the confirmed catalog-before-credentials moment (PRD P27/P28). The GIF is an 11-frame console tour with no installation or request (readme-baseline.json, planning.md:63–65). The embedded docs are auth-gated with 14 px prose and no anchors (G18, U01–U05; confirmed in the desktop screenshots — the h2 headings render *smaller* than the panel intro text, an inverted hierarchy). Yet CSP20 depends on CSP17–CSP19, CSP18 depends on CSP16–CSP17, and the phase table measures exits in cumulative case counts. Fixing the README's order of operations, recording a truthful demo of the *current* release, adding heading anchors, and lifting static docs out of the credential guard require none of the shared-settings contract, fleet fencing, or the config-revision API.

**Proposed correction.** Split into two releasable tracks before activation. Track A: README reorder (catalog first, provider second — current commands already support this), demo re-record against the current release, docs typography/anchors/contrast, static docs outside the auth guard. Track B: the architecture campaign as planned. Accept that Track A's demo may need one re-record after Track B's UI changes (AUD10); a 45-second re-capture is cheaper than months of a broken first impression. Where Track A text depends on unsettled command contracts (`starport init` semantics), keep the current documented commands.

**Acceptance evidence.** A Starport release ships with the reordered README, a demo meeting A36–A37's scene requirements, and publicly reachable static docs — before A13–A15 fleet evidence exists.

**Owning tasks.** CSP18 (split), CSP20, CSP21; plan phase table.

### FBL-02 · Credential precedence flip has no migration mechanism

**Evidence.** Today Starport inference checks conventional names before `STARPORT_` names (F08; starport `internal/config/README.md:73–74`: "checks its conventional environment names first. It then checks the derived `STARPORT_<PROVIDER>_<FIELD>` value"). Spec §7.2 targets the reverse: `STARPORT_<PROVIDER>_<FIELD>`, then conventional names. The spec says "the target behavior must not silently change the charged inference identity" and "migration must report which variable wins" (ENGINEERING_SPEC.md:436–439) — but reporting is not preventing. A user upgrading with both variables set, holding different keys, silently starts paying with the other one.

**Proposed correction.** Define the migration rule: when the old and new precedence would select different values for the same provider field, refuse to activate that provider (or refuse startup for it) with a diagnostic naming both variables, until the operator removes one or sets an explicit selection. Never resolve the conflict silently in either direction.

**Acceptance evidence.** A test sets both variables to distinct values, upgrades the resolver, and proves no request is charged to the newly preferred key without explicit operator action. Extend A11.

**Owning tasks.** CSP7 (Starmap side), CSP9 (Starport side).

### FBL-03 · Model identity needs a client-facing lifecycle contract

**Evidence.** The PRD's merge section (PRD.md:126–146) governs how the *catalog* handles identity, deletion, and tombstones. The compatibility section (spec §10) governs routes, fields, and streaming. Neither states what a *gateway client* experiences when a generation renames `author/slug`, retires an alias, or removes a model: error code, error body, grace period, or successor pointer. OpenRouter-migrating startups hardcode model IDs in deployed code; D3's four-hour update cadence means this can change under them at any time. "Preserve exact model identifiers" (A17's shape tests) covers stable IDs but not the transition events.

**Proposed correction.** Add a model-identity lifecycle contract to the PRD and the compatibility matrix: IDs are stable; a rename ships as an alias with deprecation metadata and a stated minimum overlap (one generation at minimum, ideally a time window); a removal returns a defined 404/410 whose body names the removal and any successor; pricing changes take effect only at generation boundaries and never mid-request (the one-runtime-per-request rule already covers the second half — state the first half).

**Acceptance evidence.** A test swaps in a generation that renames one model and removes another while requests are in flight; in-flight requests complete under the old identity, new requests to the renamed ID succeed via the alias with a deprecation signal, and new requests to the removed ID receive the defined error. Extend A18.

**Owning tasks.** CSP3 (alias/tombstone semantics in Starmap), CSP10 (routing behavior), CSP18 (documented contract).

### FBL-04 · GIF numeric bounds don't belong in the release gate

**Evidence.** A38 makes "35 to 45 seconds" a pass/fail condition in the same summary that gates production claims ("Summary: 39 passed, 0 failed", plan.html:78). The size bound (10 MiB) is a real renderer constraint; the duration and two-second hold rules are editorial guidance.

**Proposed correction.** Keep 10 MiB and 900 px readability as measured checks. Move duration and hold times into the brief's human-review checklist, which already exists (readme-demo-brief.md:98–100).

**Acceptance evidence.** A38 reworded; the verifier checks size, dimensions, manifest completeness; the recorded human review covers pacing.

**Owning tasks.** CSP21; spec §10.4.

### FBL-05 · The public docs site has no owner

**Evidence.** D8 is user-confirmed: full docs ship offline *and on a public site* from the same content. Spec §10.2 requires stable version URLs and a version selector (995–998). CSP23 says "publish… the matching public documentation" but the plan's verification section explicitly authorizes no hosted sites, and no task names the hosting mechanism, domain, deployment pipeline, or the operator of that pipeline. This is a confirmed production promise without a necessary dependency.

**Proposed correction.** Before activation, name the hosting mechanism (a GitHub Pages deployment from the docs build is the least-cost fit for versioned static content) and its owner in CSP23, or narrow D8's v1 claim to "public documentation published as versioned release artifacts" until hosting authority exists.

**Acceptance evidence.** A29's public-side check runs against a real public URL for the release pair.

**Owning tasks.** CSP18 (build output), CSP23 (publication), release owner.

### FBL-06 · D6's payload materialization needs a named user problem

*Labeled challenge to a proposal (D6 is marked "Proposed interpretation").*

**Evidence.** Spec §3.1 requires every fresh persistent startup to write the embedded baseline manifest *and payload* into the catalog directory, with idempotency rules, fleet compare-and-swap seeding, "not a second authority" caveats, and migration handling (84–118). The payoff listed is inspectability and a durable baseline identity. But the identical bytes are compiled into the binary forever (D4), and A01's real user need — same digest served across restarts with no network — holds with or without a payload copy on disk.

**Proposed correction.** Persist a small generation *record* (identity, digest, schema, timestamp) for the accepted-head machinery and diagnostics; make the full payload export an explicit `starmap catalog export` / `starport catalog export` command for the tooling use case. If the payload copy stays, the PRD's outcomes list should name who reads that file and why.

**Acceptance evidence.** A01 rewritten around digest stability and record presence; export command test covers the inspectability need.

**Owning tasks.** CSP2, CSP8.

### FBL-07 · Drop the fourth update-control knob for v1

**Evidence.** Spec §6.2's intent table (364–372) is excellent — every row names a real operator intent. But the intents are coverable by three controls: `NETWORK_MODE=offline` (prohibit catalog egress), `ACQUISITION_ENABLED` (provider observations), and pin (freeze the effective catalog). `SOURCE_REFRESH_MODE=manual` ("stop automatic source changes but allow explicit refresh") is the only proposal without a persona who cannot use pin-plus-explicit-refresh instead.

**Proposed correction.** Defer `SOURCE_REFRESH_MODE`. Keep the intent table and map its "stop automatic source changes" row to pin semantics. Revisit if an operator asks for unpinned-but-manual behavior.

**Acceptance evidence.** A22/A23 rescoped to three controls; the UI's combined-effect display (§6.2 requirement) covers three knobs instead of four.

**Owning tasks.** CSP5, CSP8, CSP17.

### FBL-08 · Narrow D12 to read, validate, and export for v1

*Labeled recommendation on pending proposals D11/D12.*

**Evidence.** Sections 7.5 and 7.7 specify management modes, revision preconditions (412/422/428), idempotency keys, draft semantics, operation receipts, activation-failure recovery, and audit — roughly a config-management product. Meanwhile the current console shows read-only facts with no origins at all (G17), and the replicated recipe already *requires* `external` mode, where the deliverable is a validated diff, not a write path. The read-only effective report with origins (P20), `POST /validate`, and diff export deliver most of the operator value for every persona; the PATCH/local-save path serves only the single-host local operator, who can edit `config.env` today.

**Proposed correction.** Confirm D11 (it is nearly forced by the ownership model). Split D12: v1 ships `GET /schema`, `GET /effective`, `POST /validate`, and export; defer `PATCH`, operation receipts, and UI save. This removes the hardest half of CSP16 and shrinks CSP17.

**Acceptance evidence.** A26/A27 rescoped: concurrent-edit and stale-revision cases apply to validate-against-revision, not to writes; locked-field and redaction cases remain.

**Owning tasks.** CSP16, CSP17; D11/D12 resolution.

### FBL-09 · Reconsider four-hour default-branch promotion cadence

*Labeled proposal to reconsider a confirmed decision (D3). Not silently rewritten; the channel cadence and release-embedding invariants stay.*

**Evidence.** Spec §5 is among the most operationally complex sections: serialized bot PRs, gate configuration, revalidation after base-branch changes, and AUD09's unresolved bot-identity and CI-trigger work — six times per day. The user-visible benefit over a daily promotion is that a fresh `git clone` is at most ~4 h stale instead of at most ~24 h stale. Runtime consumers are unaffected either way (they follow the channel); release consumers are unaffected (releases embed the promoted input at tag time either way).

**Proposed correction.** Keep the four-hour channel publication exactly as confirmed. Promote to the default branch once daily (or at release preparation), preserving D3's invariant that new releases embed the latest accepted catalog. If the four-hour promotion stands, accept AUD09's cost knowingly.

**Acceptance evidence.** A05/A06 unchanged in substance; only the promotion schedule constant differs.

**Owning tasks.** CSP6; product owner sign-off since D3 is confirmed.

### FBL-10 · Pull native platform evidence forward

**Evidence.** CSP2 explicitly defers native qualification to CSP22 (plan.html:135), yet CSP2/CSP8 ship the path contract and the conflict-aware migration that touches every existing installation, including Windows rename/ACL behavior the spec itself flags as risky (§4.3, 234–239).

**Proposed correction.** Add native Windows and Linux CI jobs for path resolution and interrupted migration as exit criteria of CSP2 and CSP8. Keep full release qualification at CSP22.

**Acceptance evidence.** A03/A04 subcases green on native runners at phase 1–2 exit, re-run at CSP22.

**Owning tasks.** CSP2, CSP8, CSP22.

### FBL-11 · Make local-provider enrollment first-class (design suggestion)

**Evidence.** The demo brief correctly allows a local provider as the published inference path (readme-demo-brief.md:63–67), and Ollama is the only credential-free story. But enrolling Ollama today means hand-authoring a Starmap workspace and exporting `STARPORT_CATALOG_WORKSPACE_PATH` (starport README.md:158–160) — a catalog-authoring task pushed onto the newest user.

**Proposed correction (suggestion, defer-able).** A sanctioned local-provider discovery that surfaces installed Ollama models as a scoped local observation fits the spec's existing scope machinery (§6.1's provider-scoped observations) without violating Starmap's identity ownership. Even v1 could ship a guided `starport` subcommand that generates the workspace entries.

**Owning tasks.** New scope; would extend CSP3/CSP10 if accepted. Do not gate v1 on it.

### 3.6 Position on the prior audit

**Agreements (all ten findings).** AUD01 (withdrawal enforcement contract), AUD03 (recovery cannot infer post-backup changes), AUD04 (no owned all-source acquisition task), AUD05 (catalog trust must not silently authorize new credential destinations), AUD06 (publisher partial-failure admission rules), AUD07 (ID coverage is not behavior coverage; CSP20's commands don't install or infer), AUD08 (Starmap admin bootstrap/audit persistence), AUD09 (bot identity), AUD10 (demo artifact handoff) — all stand up to independent inspection. AUD05 deserves emphasis: it is the sharpest trust-boundary gap in the design, because a catalog update is exactly the mechanism an internal authority uses, and endpoint repointing plus credential placement is what it controls.

**Extension to AUD02 (budget outage).** Reading `internal/server/budget.go:156–178` adds a fact that materially de-risks the recommended fail-closed policy: usage reads happen *per configured budget rule*. A key, account, or team with no budget rule triggers no read and is untouched by any outage policy. Fail-closed therefore affects exactly the holders who asked for enforcement — local developers without budgets never see it. This strengthens the audit's recommendation; see question Q1.

**Extension to AUD07.** The phase exit criteria themselves ("17 cumulative primary cases pass") measure verifier counts, not user journeys. Even with honest subcases, a phase can exit without any persona journey improving. Pair the subcase roster with per-phase journey statements.

**No disagreements.** On AUD10 I would bind the recording to the qualified release-candidate artifact with defined re-record triggers rather than moving recording after release, but both resolve the finding.

## 4. Journey review

### Local developer

**First result.** Target state is strong: install via brew, `starport dev`, see a real catalog with prices before any credential — few choices, clear prerequisites. Today's README inverts it (key first), and the settings screenshot shows the consequence of the current console philosophy: facts like `unstamped build` / `unavailable` with gray environment-variable names at 2.56:1 contrast as the only path to action. The offline catalog promise is honestly bounded (PRD explicitly separates catalog access from inference compute), so there is no offline-inference oversell.

**Failure paths.** Unwritable state directory and ephemeral-vs-persistent confusion are both specified (PRD startup table; spec §3.1's "must not silently switch to ephemeral"). The gap is FBL-02: an upgrade silently changing which env var pays. Recovery docs currently require the console to be up and authenticated (G18) — the exact situation where they are needed is the one where they are unreachable. CSP18 fixes this; FBL-01 argues it should not wait.

**Verdict.** The designed journey is right. The shipped journey stays wrong for the whole campaign under the current ordering.

### Startup (one instance → fleet)

**Growth path.** The migration-trap analysis is genuinely good: G07 (Valkey with replica-local SQLite silently accepted today) is caught, CSP12 makes T4 refuse incomplete recipes, and §8.6 forbids presenting a backend dropdown as a migration. The KV+SQL+files-move-together rule and the quiesce/export/verify/switch order are the correct guardrails. Provided CSP12/CSP13 land as specified, a startup can grow T2→T3→T4 without a data trap.

**Remaining exposure.** FBL-03 is the startup's biggest unpriced risk: hardcoded model IDs meeting a four-hour catalog cadence with no rename/removal contract. Second: the OpenRouter contract's real evidence (A17, raw HTTP + official SDKs) arrives only at CSP22; until then the 17-condition structural guard is the only proof, and the audit correctly notes it mostly checks source structure. The base-URL-swap examples exist and the PRD's honest scope limit ("does not imply identical commercial services") is the right posture.

**Failure paths.** Budget-store outage behavior is the open decision (Q1). Lease loss, missed events, and follower recovery from the durable head are well specified (§8.2).

### Enterprise operator

**Authority, egress, outage.** This is the strongest part of the design. D1/D2/D13, no implicit public fallback, catalog-egress-versus-inference-egress separation (fixing G11), the authority identity (`CATALOG_AUTHORITY_ID`) surviving URL migration, and the retained-catalog-with-freshness-warnings default are all coherent and explainable. The T1–T7 target table with explicit "do not infer" columns is exactly what an enterprise evaluator needs.

**Cold-start recovery.** D2's first-boot block is correct, but it is also the enterprise's first impression: a pilot gateway pointed at an internal Starmap that has not yet approved a generation is a gateway that refuses inference. The spec keeps diagnostics available and the docs plan includes a "no accepted authority" troubleshooting entry; make that the single most polished recovery page, and make the console's blocked state name the approving action, not just the condition.

**Gaps.** AUD03 (restore cannot reconstruct post-backup revocations) and AUD05 (credential destinations) are the two contracts an enterprise security review will ask about first; both must settle before implementation. AUD08 (admin bootstrap/audit persistence on a no-SQL T1 server) is the third. FBL-05 (docs hosting) affects the evaluation phase: D8 promises a public site that no task builds.

## 5. Keep, simplify, defer, reject

**Keep.**
- The ownership boundary table and "no second roster in Starport" (PRD product boundaries) — this is the product's spine.
- D1, D2, D13, D5, D9, D10; the layer order and complete-snapshot-replaces-base rule (a union would resurrect deletions — PRD.md:141–143 states the reason precisely).
- Credential role separation and the source-bound-credential rule ("an inherited token cannot follow a new URL", spec §7.4) — cheap to state, prevents a real leak class.
- One runtime generation per request; membership/routability/credential/availability as separate reported states (P16, §10 status list) — this answers the distinguishability question well.
- The storage matrix with its "do not infer" column; XDG/platform path contract; P27/P28 and the demo brief's honesty rules (no fabricated results, labeled cuts).
- Typography and WCAG 2.2 AA targets (§10.3) — U01/U02/U08 are confirmed by the screenshots and measurements; these targets are corrective, not gold-plating.

**Simplify.**
- D6 → persist a generation record, add an explicit export command (FBL-06).
- §6.2 → three update controls, not four (FBL-07).
- A38 → 10 MiB and readability measured; duration reviewed (FBL-04).
- P19's superset already has the right escape hatch ("state a versioned restriction; silent omission is invalid") — use it aggressively for aliases/coalescing rather than surfacing them in Starport's UI.

**Defer.**
- D12's PATCH/local-save surface → v1 is read/validate/export (FBL-08).
- Redis and MySQL alternative qualification *runs* in CSP15 → declare unsupported in v1 with the existing honest wording; the spec already forbids inherited qualification, so skipping the runs costs nothing but the support claim nobody has asked to make yet.
- Valkey Cluster support beyond a hash-slot layout guard.
- Command-palette docs-search integration (U06's "may share the index") → ship docs search itself; palette integration later.
- FBL-11's local-provider enrollment → good v1.x candidate.

**Reject (for v1).**
- Any Starport-mediated administration of upstream Starmap (spec §7.6 already leans this way — make it a stated non-goal rather than an open decision).
- GIF duration as a release gate (FBL-04).
- Any supported-recipe claim for shared Badger/SQLite files or untested backend combinations — the documents already reject these; keep them rejected under launch pressure.

## 6. First-release boundaries and ordering changes (proposal only; the plan is not activated or rewritten here)

**Release A — first use (small, fast, against the current product):** README reorder, demo re-record, docs typography/anchors/contrast, static docs outside the auth guard, read-only effective-config origins. Delivers P24 (partially), P27, P28 and the U01–U08 corrections.

**Release B — catalog and identity core (Starmap-led):** shared settings contract, baseline record and paths (with native CI — FBL-10), field-authority reconciliation with the identity lifecycle contract (FBL-03), retained-authority startup, update controls (three knobs), credential precedence with migration guard (FBL-02), publisher admission rules (AUD06). Completes the T2/T3 story.

**Release C — fleet and enterprise:** atomic fencing, storage recipes and migration, recovery reconciliation (AUD03), destination policy (AUD05), Starmap admin split (AUD08), Valkey+PostgreSQL qualification, T4/T5 recipes, config validate/export, full docs platform and hosting (FBL-05).

Ordering changes to the existing ledger, in place: cut CSP20/CSP21's dependencies on CSP17/CSP19 down to a CSP18-lite (static docs + anchors); pull native CI into CSP2/CSP8 exits; add owned tasks for the budget-outage policy (AUD02), all-source acquisition (AUD04), and docs hosting (FBL-05); add per-phase journey statements beside the case counts (AUD07 extension).

## 7. Product questions (five, each materially design-changing)

**Q1 — Budget-read outage: fail open or closed?** (The pending decision.) **Recommend: fail closed, with a 503-class retryable refusal and diagnostics, applied to account, key, and team budgets including batch admission.** The decisive fact from `budget.go:156–178`: reads occur only per configured budget rule, so unbudgeted traffic — all default local development — is untouched. Fail-closed affects exactly the tenants who asked for enforcement. Tradeoff: a shared-store blip becomes a hard outage for budgeted tenants; if that is unacceptable for some deployments, add one explicit, logged `degraded=allow` opt-out rather than making permissive the default. This aligns the code with the PRD's shared-store-outage row instead of amending the PRD to match the code.

**Q2 — D12 scope: does v1 ship configuration writes at all?** **Recommend: no — read, validate, export only (FBL-08).** Tradeoff: single-host operators keep editing `config.env` by hand, which they do today; in exchange, CSP16/CSP17 shrink by roughly half and the concurrency/revision surface (the riskiest new API) disappears from v1.

**Q3 — D6: record or payload?** **Recommend: persist the generation record; make payload export an explicit command (FBL-06).** Tradeoff: no ambient inspectable payload file on disk; anyone who needs one runs one command. Removes the CAS-seeding and baseline-migration complexity from every cold start.

**Q4 — D3 promotion cadence (reconsideration of a confirmed decision).** **Recommend: keep the four-hour channel, promote the default branch daily or at release preparation (FBL-09).** Tradeoff: fresh source checkouts can be up to ~24 h stale versus ~4 h; in exchange, the bot-PR serialization, identity, and revalidation machinery (spec §5 steps 7–8, AUD09) runs 1× daily instead of 6×, and main's history stays readable. If the answer is "keep 4 h," AUD09's cost is accepted knowingly and nothing else changes.

**Q5 — Model identity lifecycle: what does a client get to rely on?** **Recommend: stable IDs; renames ship as aliases with deprecation metadata and at least a one-generation overlap; removals return a defined error naming any successor; price changes bind to generation boundaries (FBL-03).** Tradeoff: Starmap carries alias/deprecation metadata and the publisher must enforce the overlap, which slightly constrains catalog editing; in exchange, the four-hour cadence stops being a standing threat to deployed client code — which is the difference between "drop-in replacement" as a claim and as a contract.

## 8. Coverage record

**Read in full:** `docs/design/catalog-lifecycle/PRD.md`, `ENGINEERING_SPEC.md`, `REPOSITORY_FINDINGS.md`; `docs/plans/starport-production-catalog-plan.html`; `docs/plans/proof/starport-production-catalog/readme-demo-brief.md`, `planning.md`; `audit-2026-09-04/AUDIT.md`; `acceptance-map.json`, `readme-baseline.json`, `evidence/measurements.json`, `audit-2026-09-04/test-summary.json`; `docs/plans/README.md`; Starport `README.md`, `GLOSSARY.md`, `internal/config/README.md`; Starmap `GLOSSARY.md`, `AGENTS.md`.

**Read in part:** Starmap `README.md` (first 150 lines), Starport `AGENTS.md` (first 200 lines), Starport `docs/DEPLOYMENT-TOPOLOGIES.md` (first 120 lines), Starport `internal/server/budget.go:130–179`.

**Screenshots inspected (as sampled states, not proof of interactive accessibility):** `starport-docs-desktop-dark.png`, `starport-docs-desktop-light.png`, `starport-docs-mobile-dark.png`, `starport-docs-build-320.png`, `starport-settings-light.png`. They corroborate U01/U02/U08 (dense 14 px prose, h2 below body size, low-contrast right-aligned setting names) and U03 (no horizontal overflow at 320 px). They include dev-tool overlays and fixture data ("Gateway unreachable", "unstamped build"), as the findings disclose.

**Not verified in this review:** no commands were run; no tests executed; GitHub-hosted evidence links (workflow runs, channel document, pinned source URLs) were taken as recorded in the findings and audit rather than fetched; screenshot hashes were not recomputed; the Starport operator guide, `.env.example`, enterprise runbook, and OpenRouter parity script were assessed through the findings' citations (G11, G12, F14) rather than full reads; the prior `fable-review-2026-09-04/` response was deliberately not read to avoid anchoring. Test results in `test-summary.json` prove current behavior at the inspected revisions only — per the review brief, they do not certify any proposed production requirement, and no external backend, native Windows/Linux, SDK, or provider evidence exists yet for any claim gated on it.
