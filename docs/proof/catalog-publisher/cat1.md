# CAT1 secure publisher proof

Date: 2026-09-01

Work commits: `cf3124c0`, `9017b83b`

## Repository secret state

The Starmap repository now stores these Actions secrets:

- `ANTHROPIC_API_KEY`
- `CEREBRAS_API_KEY`
- `DASHSCOPE_API_KEY`
- `DEEPSEEK_API_KEY`
- `FIREWORKS_API_KEY`
- `GOOGLE_API_KEY`
- `GROQ_API_KEY`
- `HETZNER_API_KEY`
- `MISTRAL_API_KEY`
- `MOONSHOT_API_KEY`
- `OPENAI_API_KEY`

The workflow exposes these credentials only to the catalog refresh step.
DeepInfra uses its public catalog endpoint. Azure OpenAI and Google Vertex stay
unconfigured because no local acquisition credentials exist.

## Live provider result

The secret-safe provider verifier reported success for 12 catalog providers:

- Alibaba Cloud Model Studio
- Anthropic
- Cerebras
- DeepInfra
- DeepSeek
- Fireworks AI
- Google AI Studio
- Groq
- Hetzner
- Mistral AI
- Moonshot AI
- OpenAI

Azure OpenAI and Google Vertex reported unavailable credentials. The verifier
returned status 0 and confirmed that its output contained no credential value.

## Workflow result

The workflow requests `17 */4 * * *`. The owner changed the cadence from six
hours to four hours on 2026-09-01. This cadence supports the six-hour
end-to-end freshness objective. The planned consumer poll runs every hour. The
refresh step binds the 11 stored credentials. A regression test checks cadence,
the exact credential roster, and refresh-step scope.

## Verification

- Fail-before: the four-hour cadence test failed while the workflow still used
  `17 */6 * * *`.
- `go test ./internal/ciworkflow -count=1`: passed after commit `9017b83b`.
- `actionlint .github/workflows/catalog-generation.yaml`: passed.
- `bash scripts/verify-action-pins.sh`: passed with 8 references.
- Technical-writing lint: passed for 4 files with 0 diagnostics.
- `make technical-writing-check`: passed for 655 files with 0 diagnostics.
- `go tool ago -stale-ignores -format json ./...`: passed with 0 findings.
- `git diff --check`: passed.
