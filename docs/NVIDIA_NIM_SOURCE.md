# NVIDIA API Catalog and NIM Source

Status: P13.20 implementation evidence

## Product boundary

NVIDIA exposes two different catalog products that must not be collapsed:

- NVIDIA-hosted API Catalog models are public provider offerings reached at
  `https://integrate.api.nvidia.com` for development and prototyping.
- NIM containers are caller-hosted deployments. Their base URL, served-name
  overrides, infrastructure, account, region, and aliases are contextual
  offering facts, not publicly publishable catalog facts.

Primary evidence:

- [NIM for developers](https://developer.nvidia.com/nim) distinguishes hosted
  trial endpoints from self-hosted NIM microservices.
- [NVIDIA API Catalog](https://build.nvidia.com/models) supplies the current
  public catalog; its OpenAI list endpoint is `GET
  https://integrate.api.nvidia.com/v1/models`.
- [NIM API reference](https://docs.nvidia.com/nim/large-language-models/latest/reference/api-reference.html)
  documents customer deployment `/v1/models`, chat, completions, responses,
  messages, metadata, manifest, readiness, and metrics contracts.
- [NIM architecture](https://docs.nvidia.com/nim/large-language-models/latest/reference/architecture.html)
  documents the container-local proxy/runtime and deployment observability.
- [NIM quickstart](https://docs.nvidia.com/nim/large-language-models/2.0.1/get-started/quickstart.html)
  states that the served name is customer-configurable with
  `NIM_SERVED_MODEL_NAME`.

## Public catalog mapping

The current public list is unauthenticated and returned 121 records in live
proof on 2026-07-12. It contains exact model ID and owner but mixes chat,
embedding, safety, parsing, video, and other API families without identifying
the invocation contract in each list record. Starmap therefore publishes these
as NVIDIA-hosted, discoverable-only offerings. It does not guess that every
record is a chat model. A future first-party typed inventory may make individual
offerings routable without changing their identities.

The optional `NVIDIA_API_KEY` is an invocation credential only. No account,
rate-limit, trial, key, or customer deployment detail is serialized.

An isolated credential-configured repeat on 2026-07-12 resolved the key without
printing it and fetched the same 121 public records. Reconciliation in a
temporary catalog reported 121 additions while preserving the public records as
discoverable-only; the temporary state was removed. This is live inventory and
credential-resolution evidence, not proof that every listed model supports the
same invocation API.

## Contextual NIM mapping

NIM discovery is a separate optional credential-scoped source and calls the configured deployment's own
`/v1/models`. Because `NIM_SERVED_MODEL_NAME` can replace the model name, the
caller must provide an explicit served-name-to-canonical-definition mapping;
unknown names fail closed. The resulting ordinary canonical catalog retains
account, deployment, URL, region, and aliases for the current caller context.
Its credential-scoped observation is rejected before every public write.

Fixtures prove strict list envelopes, public non-routability, author/provider
separation, private endpoint and scope retention, explicit mapping, missing
mapping failure, and absence of private identity from the public model. Shared
transport fixtures cover bounded retry, cancellation, terminal authorization,
rate-limit, and transient failure behavior.
