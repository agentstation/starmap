# CAT2 findings ledger

Date: 2026-09-02. Owner task: CAT2. The plan keeps a pointer to this record.
Each finding names the task that owns its repair. A `closed` classification
means that the campaign needs no further work on it. A `preserve`
classification names behavior that the campaign must keep.

| ID | Classification | Evidence | Owner |
| --- | --- | --- | --- |
| CAT-F1 | closed | Eleven repository secrets now cover 12 live provider catalogs. | CAT1 |
| CAT-F2 | naming | The live repository has 28 semantic tags, 17 payload tags, and no latest channel. | CAT3 |
| CAT-F3 | architecture | Release import, HTTP subscription, and root client lifecycles are separate. | CAT4-CAT7 |
| CAT-F4 | integration | The CLI parses remote server settings but rejects them during composition. | CAT6 |
| CAT-F5 | integration | Server and Docker start from durable or embedded state and follow no source. | CAT6 |
| CAT-F6 | preserve | Starport already separates remote head from accepted runtime state. | CAT8 |
| CAT-F7 | trust | A Go consumer needs native Sigstore verification instead of the `gh` executable. | CAT4 |
| CAT-F8 | dependency | GitHub CLI and the reviewed production consumers use `sigstore-go` as the engine and add domain policy around it. | CAT2.1 |
| CAT-F9 | DX | Starport remote `Sync` performs no network work, while local `Sync` runs provider acquisition. | CAT8 |
| CAT-F10 | architecture | A connected source and acquisition scheduler need one mutation owner. | CAT5 |
| CAT-F11 | integration | Starport selects local acquisition or remote generations and cannot combine them. | CAT8 |
| CAT-F12 | correctness | The runtime must retain input layers or a public refresh can overwrite operator evidence. | CAT5 |
| CAT-F13 | correctness | The provider source attempts every provider and treats missing credentials as aggregate degradation. | CAT5 |
| CAT-F14 | persistence | Effective generation storage cannot restore independent upstream and per-provider layers. | CAT5 |
| CAT-F15 | integration | Starport development mode disables catalog refresh and remote configuration forbids local acquisition. | CAT8 |
| CAT-F16 | security | Starport's Starmap resolver bypasses the loader lookuper used for process and `.env` values. | CAT8 |
| CAT-F17 | operations | Starport applies a one-minute request context to two-minute refresh and inference work, and to streams. | CAT8 |
| CAT-F18 | scale | GitHub permits 60 unauthenticated REST requests per hour per source IP. | CAT4-CAT8 |
| CAT-F19 | operations | The latest 20 successful publisher runs took 153-285 seconds; no job timeout is set. | CAT3 |
| CAT-F20 | operations | Starmap applies a 30-second total timeout to bodies allowed to reach 16 or 64 MiB. | CAT4-CAT7 |
| CAT-F21 | operations | Starport applies a two-minute remote body context and global elapsed request limits to long operations. | CAT8 |
| CAT-F22 | scale | The Starmap subscriber retries within 100 ms to five seconds and can synchronize a fleet after an outage. | CAT7 |
| CAT-F23 | closed | The runtime review and the final review stated two `Sync` signatures and two `Close` bounds. CAT-D14 reconciles them. | CAT2 |
| CAT-F24 | operations | Inactivity and size caps alone allow a slow-drip transfer to hold a refresh for hours. No per-transfer total bound exists. | CAT4-CAT5 |
| CAT-F25 | operations | `remote.NewClient` copies the caller's client, and the SSE open has no response-header bound. | CAT4, CAT7 |
| CAT-F26 | operations | Starport bounds a stream with the two-minute `MaxElapsed` budget and a 30-second connector header timeout outside the route middleware. | CAT8 |
| CAT-F27 | scale | Direct GitHub consumers spend a per-identity request budget that the rate-limit headers report. A fixed number is a ceiling, not a safe threshold. Each polling hop adds one interval to the freshness age. | CAT4, CAT7, CAT9 |
| CAT-F28 | architecture | The refresh lease has no TTL, renewal, or fencing token, so a lost lease can still commit. | CAT8 |
| CAT-F29 | dependency | Starport pins Starmap `v0.15.0`, and the ledger orders CAT8 before the CAT11 release. | CAT8, CAT11 |
| CAT-F30 | preserve | The Starport console reads `age_seconds` and `degradation_reasons` from the freshness document. | CAT8 |
| CAT-F31 | DX | The console spends a full-width row on Models and half the Overview grid on four catalog facts. No surface shows the source, the derivation chain, or the next update. | CAT8.1 |
| CAT-F32 | operations | No shipped document explains a central Starmap server, a Starport fleet, or the choice between direct, central, and embedded sources. Neither repository has a topology diagram. | CAT9.1, CAT9.2 |
| CAT-F33 | DX | No consolidated catalog configuration reference exists. `.env.example` is the only reference, and the Starport README is outside CAT9. | CAT9.1 |
| CAT-F34 | correctness | A freshness fallback to `generated_at` let a downstream hop or local acquisition hide a stale public channel. | CAT7 |
| CAT-F35 | architecture | The contract let completed provider layers advance, but no rule bounded the generations that a slow peer could cause. | CAT5 |
| CAT-F36 | architecture | A cloned scheduler identity in shared or copied state synchronized replica phases. | CAT5 |
| CAT-F37 | DX | The console lifecycle defined only the stop after `403`, and the Starport catalog queries had a stale time but no refetch interval. | CAT8.1 |
| CAT-F38 | verification | CAT-V03 accepted reversed or unrelated timeout values, and CAT-V64 depended on an ambient interpreter. | CAT2 |
