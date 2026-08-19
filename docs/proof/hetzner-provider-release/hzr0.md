# HZR0 baseline and live contract

- Starmap baseline: `main` at `b24d6589`.
- Starport baseline: `main` at `9f3888f`.
- Starmap worktree: clean, one primary worktree, only `main` before work.
- Starport worktree: clean, one primary worktree.
- Fail-before: `internal/embedded/catalog/providers.yaml` has no provider with
  ID `hetzner`.
- Public documentation reports an OpenAI-compatible base URL at
  `https://inference.hetzner.com/api/v1`, bearer authentication, model listing
  at `/models`, and chat completions at `/chat/completions`.
- Current documentation names two definitive models:
  `Qwen/Qwen3.6-35B-A3B-FP8` and `Qwen3.8-27B`. Both have a 262,144-token
  context and accept text and image input.
- Live authenticated contract: blocked until the user confirms the final
  persistent-token creation action.
- Branch cleanup: removed 23 remote branches tied to merged pull requests and
  four integrated local branches. Preserved five remote branches with unique
  commits and no pull request. Also preserved seven local branches with unique
  work.
