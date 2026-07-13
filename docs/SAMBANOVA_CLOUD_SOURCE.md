# SambaNova Cloud Source

Status: P13.31 implementation evidence

SambaNova's [model-list API](https://docs.sambanova.ai/docs/api-reference/models/get-environments-available-model-list-metadata)
defines bearer-authenticated `GET /v1/models` and model ID, owner, context
length, maximum completion tokens, and exact per-token prompt/completion prices.
The [SambaCloud product](https://sambanova.ai/products/sambacloud) confirms the
OpenAI-compatible on-demand channel, while [plans](https://cloud.sambanova.ai/plans)
separate pay-as-you-go developer access from enterprise custom/BYOC capacity.

Starmap uses a native one-page decoder so the richer limits and prices are not
discarded by the generic OpenAI model shape. Model-family evidence preserves
Meta, DeepSeek, Qwen, Mistral, and OpenAI authors independently from SambaNova
as provider. Exact per-token decimal strings are retained and converted to USD
per million tokens. Unknown IDs receive SambaNova authorship rather than a
guessed upstream; absent prices remain absent. Duplicate IDs, null/wrong
envelopes, invalid objects or limits, and malformed/negative/non-finite prices
fail closed. Unknown fields remain source-drift evidence.

Fixtures prove bearer auth, author/provider separation, context/output limits,
exact price conversion, serverless on-demand routing, drift evidence, duplicate
and malformed-envelope failure, invalid pricing, and factory routing. The shared
transport suite supplies bounded transient/terminal retry proof. Live discovery
is explicitly skipped when `SAMBANOVA_API_KEY` is absent; custom/BYOC capacity
does not become a public model or invented deployment.
