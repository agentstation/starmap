# HZR3 Starport catalog update proof

Work commit: `269a70b` (`deps: consume Starmap Hetzner catalog`).

Pull request: [agentstation/starport#116](https://github.com/agentstation/starport/pull/116).

## Contract

- Starport now consumes Starmap `v0.6.0`.
- One catalog-boundary test projects the two exact Hetzner model IDs to their
  canonical definitions and OpenAI-compatible chat endpoint.
- One credential-boundary test proves that
  `STARPORT_HETZNER_API_KEY` resolves from Starmap's credential declaration.
- Starport adds no provider roster, endpoint table, model list, price, or
  provider-specific transport switch.
- The Starmap catalog-acquisition token remains only in Starmap's ignored
  `.env`. Starport has no copy of that token.

## Fail-before and checks

- Before the dependency update, the focused route test failed to compile
  because Starmap `v0.5.0` did not define `ProviderIDHetzner`.
- After the update, both focused Hetzner tests passed.
- The ownership suite passed 12 of 12 assertions.
- The v1 architecture suite passed 12 of 12 assertions.
- The catalog-driven provider suite passed 19 of 19 assertions.
- Package-layout and README quickstart checks passed.
- `go test ./...` and `go vet ./...` passed.
- `make lint` passed with zero issues, and `make build` passed.
- Raw HTTP chat, streaming, model, and embedding smoke checks passed.
- The Python, TypeScript, and Go OpenRouter SDK smoke checks passed.
- Module verification and `git diff --check` passed.
- The required pre-PR autoreview reported no actionable findings. GitHub
  verifies the commit signature.
