# README and demonstration brief

Updated: 2026-09-05. Status: proposed. This brief defines the early first-use tasks, CSP20, CSP21, and CSP24 in the
[canonical plan](../../starport-production-catalog-plan.html).
It does not record a new demonstration or authorize provider charges.

## Delivery stages

CSP0.2 improves the README using current commands and the current published release.
CSP0.3 records that release's first-use sequence with a real inference path.
Those tasks can finish before fleet and configuration implementation.
They must state current support limits and cannot advertise unfinished behavior.

CSP20 aligns the README with the final candidate.
CSP21 creates the recording tools and rehearses that immutable candidate.
CSP24 records the final demonstration after the release assets exist.
That order permits the final capture to use the actual advertised installation path.

Record the artifact hash, version, platform, installation method, and content revision at each stage.
A later setup, UI, inference, or installer change invalidates affected evidence.
A changed artifact or performance path also invalidates affected latency evidence.
The inference scene runs at real speed. Disclose cuts elsewhere without implying a gateway latency measurement.
Retain the earlier recording as historical evidence, not as proof of the final product.

## Intended result

A new developer installs Starport, sees the catalog, configures a provider, and
receives a streamed answer through the gateway. That sequence is the product
demonstration. Show the selected provider and route only when the running product
reports them.

The catalog works before provider credentials exist. Inference needs a supported
provider connection. Show those two states in order. Keep gateway authentication
separate from provider authentication.

## README sequence

1. State what Starport does and show a static demonstration preview.
2. Provide tested installation commands for each advertised platform.
3. Start the temporary development instance and inspect its catalog.
4. Configure one inference provider through the documented path.
5. Send a request with a gateway key and show the streamed answer.
6. Show the client base URL and the tested compatibility boundary.
7. Link persistent setup, team deployment, and enterprise deployment procedures.

Keep commands copyable outside the animation. Keep provider and gateway variable
names distinct. Generate catalog counts from the recorded release or omit them.
Move detailed performance and architecture material after first use.

Every latency claim must link the qualified profile, exact artifacts, workload, and timing boundaries from specification section 8.9.

The existing controller-only measurement cannot substantiate a complete gateway claim.
Early README work must qualify or remove that claim until complete measurements support it.
Explain that durable storage and valid request-serving memory have separate roles in the linked deployment guides.
Keep strict budget admission distinct from optional response caching.


Label `starport dev` as temporary. Do not imply that it saves persistent state.

The inspected release can retain explicit object-store and catalog-state settings during development.
Early documentation must disclose those exceptions. Remove persistent storage selectors from the recorded process environment before starting the temporary demonstration.
Record the absence of those selectors without recording their values.
Final capture must pass A43, which rejects conflicting persistent selections before opening stores.

Show `starport init` and `starport serve` in the persistent setup path, subject to
the final command contract. Explain the product directories in that path.

The persistent path must show the effective config file and data/state roots for the recorded platform.
Keep engine details in linked storage procedures. Explain that Badger holds durable KV records and Ristretto holds disposable memory caches.
Fleet links must name Valkey, PostgreSQL, shared blob bytes, and private replica state as one complete recipe.
A service selector or database connection alone must not imply that existing records migrated.

Test the actual Homebrew tap, release archives, and other advertised installation
methods. Do not infer operating-system support from the package name.

Align the Starmap README with the same ownership rules. Explain passive library
reads, persistent application startup, acquisition, publication, and server
subscription. The replacement GIF belongs to the Starport README.

## Proposed storyboard

The proposed pacing target is 35–45 seconds. Adjust scene boundaries after rehearsal.
A person can accept another duration with a recorded readability and pacing assessment.

| Time | Visible action | Evidence the viewer receives |
|---|---|---|
| 0–6 s | Install the selected release. | A short command produces a working executable. |
| 6–12 s | Start development mode and open the catalog. | Useful model information exists before provider credentials. |
| 12–18 s | Select the provider and show its configured state. | The operator knows where inference access comes from. |
| 18–30 s | Submit a short request and show the streamed answer. | A real request succeeds through Starport. |
| 30–37 s | Show the reported provider and client base URL. | The same gateway connects the client to the provider. |
| 37–42 s | Show the persistent setup link. | The developer has a clear next step after the temporary session. |

Use one provider unless the recording proves a reason to show more. Do not claim
fallback or routing across several providers from a single-provider recording.

Prepare credentials outside the captured region. Show the configuration step and
masked result without showing secret values, tickets, cookies, or private data.
The setup must use the documented inference credential path.

Use fixtures only for rehearsal. The published answer must come from a real
supported provider. A supported local provider is acceptable. Remote provider
calls require authorized credentials and a defined budget before recording.

Do not fabricate installation output, model counts, route data, answers, or
latency. Label each cut that shortens a wait. Preserve the complete
uncut run as evidence outside the published animation.

## Capture and accessibility requirements

- Capture at a source width of at least 1280 px.
- Keep the GIF below the 10 MiB project budget.
- Record the reviewer's pacing decision and any duration exception.
- Review the rendered asset at a 900 px display width.
- Keep essential instructions readable at an effective size of at least 14 px.
- Use two seconds as the proposed instruction-frame target. Verify actual reading time.
- Use zoom or a focused panel when the full console makes text too small.
- Provide a static poster, useful alternative text, and a text transcript.
- Check links, loading failures, both themes, and a narrow viewport.

Use pause controls and reduced-motion behavior where the renderer supports them.
Otherwise, use a poster that links to the animation. A raw GIF on GitHub does not
provide authored playback controls. Do not claim complete accessibility from
alternative text alone.

## Reproduction and acceptance

CSP21 creates `scripts/record-readme-demo.sh` and
`scripts/verify-readme-demo.sh` in Starport. Those scripts do not exist as part of
this planning change.

Keep the source capture, edit instructions, poster, transcript, GIF, and manifest.
The manifest records the release, both repository commits, catalog generation,
operating system, capture dimensions, tool versions, file hashes, and cut points.
Record the nonsecret provider path and the reviewed run result.
Record the persistence mode, resolved nonsecret paths, scratch ownership, and storage selector checks.
Verify temporary cleanup and persistent-installation preservation outside the captured scene.

The verifier records duration and checks dimensions, bytes, manifest completeness, and links.
Duration alone does not determine acceptance.
A person must review readability, scene order, secret exposure, and truthful
results. Record that review separately from raw tool output.

The [specification](../../../design/catalog-lifecycle/ENGINEERING_SPEC.md) defines cases A34 through A39.

The [PRD](../../../design/catalog-lifecycle/PRD.md) defines P27 and P28.

The [acceptance map](acceptance-map.json) assigns early, rehearsal, and final checks to their tasks.
Final A35 through A39 evidence requires the shipped artifact in CSP24.
