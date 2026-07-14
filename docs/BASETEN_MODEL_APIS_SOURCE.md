# Baseten Model APIs Source

Status: P13.33 implementation evidence

Baseten's [Model APIs overview](https://docs.baseten.co/inference/model-apis/overview)
defines bearer-authenticated `GET https://inference.baseten.co/v1/models` as
the current shared catalog, including pricing, context, and feature metadata.
The same shared serverless models support OpenAI Chat Completions and Anthropic
Messages. Pricing is USD per million uncached input, cached input, and output
tokens; all models support tool calling.

Starmap uses the shared OpenAI-compatible decoder with Baseten-specific current
defaults: active serverless `model-api` deployment, both chat and messages
contracts, tool support, and the exact shared inference base. Owner-qualified
slugs preserve DeepSeek, Moonshot, NVIDIA, OpenAI, and Zhipu authors separately
from Baseten. Returned context/output limits, modalities, reasoning/structured
output features, cache pricing, and unknown-field evidence remain attached to
the offering. Missing prices remain absent and malformed numeric values fail
decoding.

Baseten's separate [management models](https://docs.baseten.co/reference/management-api/models/gets-all-models)
and [deployment API](https://docs.baseten.co/reference/management-api/deployments/gets-all-deployments-of-a-model)
contain workspace model/deployment IDs, environment names, hardware, scaling,
and dedicated subdomains. The public provider calls only the shared
`inference.baseten.co` catalog and never the customer management API; no Truss,
chain, deployment, environment, team, or custom endpoint identity can enter
public generation.

Fixtures prove bearer auth, exact author identity, limits, modalities, tool,
reasoning and structured-output features, per-million input/output/cache price,
dual invocation contracts, serverless deployment, and unknown-field evidence.
The shared OpenAI client and transport suites cover strict envelopes and bounded
terminal/transient faults. Live discovery is explicitly skipped when
`BASETEN_API_KEY` is absent.
