# CAT0 baseline proof

Date: 2026-09-01

Baseline: `main` at `42b610a42b2ca73893e4002d6bc17a63ce4b2e98`

## Credential inventory

The local Starmap environment file contains these non-empty catalog-acquisition
credentials:

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

The DeepInfra catalog endpoint is public. Azure OpenAI and Google Vertex
catalog credentials are unavailable. The Cohere credential is inference-only
and stays outside the catalog workflow.

The Starport environment file contains three acquisition credentials. Each
value equals its Starmap counterpart:

- `ANTHROPIC_API_KEY`
- `GROQ_API_KEY`
- `OPENAI_API_KEY`

The Starport worktree contains unrelated identity and limit changes. This plan
does not modify that worktree.

## GitHub baseline

The Starmap repository stores no provider Actions secrets. The catalog workflow
names eight provider secret references. Each reference therefore resolves to an
empty string in the scheduled job.

The workflow schedule is `17 3 * * *`, which requests one run each day. The
latest completed scheduled run succeeded on 2026-09-01.

The latest application release is `v0.15.0`. The latest catalog resource is the
immutable prerelease
`catalog-semantic-88dc537ac3ad5fef26490bbedd1c44d634f2c7101149bd8dee354242a9910312`.

## Consumer baseline

The catalog workflow publishes three immutable GitHub release assets. It does
not publish a mutable latest-catalog pointer.

The `remote` package consumes a versioned Starmap HTTP API and an SSE stream.
It does not discover or download GitHub catalog releases. The release import
path accepts locally supplied verified release assets.

## Commands

The inventory used these read-only sources:

- `internal/embedded/catalog/providers.yaml`
- `scripts/verify-live-providers.sh`
- `.github/workflows/catalog-generation.yaml`
- `gh secret list --repo agentstation/starmap --app actions`
- `gh release list --repo agentstation/starmap`
- `gh run list --repo agentstation/starmap --workflow catalog-generation.yaml`
- Local Starmap and Starport environment files, with values suppressed.
