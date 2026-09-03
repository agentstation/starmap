# Starmap glossary

Use these terms in developer documentation and source comments. Code
identifiers, command names, API fields, and protocol values remain exact.

| Term | Definition | Avoid | Status | Evidence |
|---|---|---|---|---|
| acquisition | An explicit operation that gets catalog evidence from configured external sources. |  | approved | `acquisition/syncer.go` |
| AI | Artificial intelligence. Starmap catalogs models that AI services offer. |  | approved | `README.md` |
| API | An application programming interface that software uses to interact with Starmap. |  | approved | `docs/REST_API.md` |
| API key | A secret value that authenticates a client to an API. | api-key in prose | approved | `docs/DOCKER.md` |
| API_KEY | The environment name of the server credential that `starmap serve --auth` requires. |  | approved | `docs/REST_API.md` |
| author | A person or organization that creates a model definition. |  | approved | `docs/CATALOG_IDENTITY.md` |
| authority policy | The field order and merge rule that select a source value for one catalog fact. |  | approved | `docs/CATALOG_AUTHORITY_POLICY.md` |
| AWS | Amazon Web Services, which provides the S3 storage service. |  | approved | `docs/CATALOG_STORE_CONTRACT.md` |
| caller-owned | Controlled by the code that calls an API, not by the called component. |  | approved | `AGENTS.md` |
| catalog | The immutable read model that contains definitions, offerings, and author membership. |  | approved | `pkg/catalogs/catalog.go` |
| catalog artifact | The archive and detached statement that distribute one validated catalog generation. |  | approved | `docs/CATALOG_ARTIFACT_FORMAT.md` |
| catalog digest | The SHA-256 digest that names one immutable catalog release tag. |  | approved | `pkg/catalogs/artifact/channel.go` |
| catalog fact | One value in an authored record, provider record, or canonical read model. |  | approved | `docs/CATALOG_AUTHORITY_POLICY.md` |
| catalog generation | A validated manifest and catalog payload with one immutable generation ID. |  | approved | `pkg/catalogs/generation_manifest.go` |
| catalog payload | The canonical JSON bytes for one catalog generation. |  | approved | `pkg/catalogs/payload.go` |
| catalog source | The configured upstream that supplies verified catalog generations to a connected runtime. |  | approved | `internal/catalog/settings/settings.go` |
| catalog store | The caller-supplied interface that persists and reads catalog generations. | generation store | approved | `pkg/catalogs/storage/store.go` |
| connected runtime | The process state that `starmap.Open` returns. It reads a catalog source on a schedule. |  | approved | `runtime.go` |
| CAS | The common abbreviation for compare-and-swap. |  | approved | `docs/CATALOG_STORE_CONTRACT.md` |
| catch-up | The process that applies events which a subscriber missed. |  | approved | `docs/REMOTE_CATALOG_PROTOCOL.md` |
| CLI | The command-line interface for Starmap operations. |  | approved | `docs/CLI.md` |
| compare-and-swap | An atomic update that succeeds only when the stored value matches an expected value. |  | approved | `docs/CATALOG_STORE_CONTRACT.md` |
| CORS | Cross-Origin Resource Sharing, an HTTP mechanism that permits selected cross-origin requests. |  | approved | `docs/DOCKER.md` |
| embedded catalog | The verified catalog baseline that Starmap compiles into its binary. |  | approved | `internal/sources/embedded/source.go` |
| effective catalog | The immutable catalog that a connected runtime builds from its retained layers. |  | approved | `runtime_layers.go` |
| egress | The outbound network traffic that a Starmap process starts. |  | approved | `docs/ARCHITECTURE.md` |
| endpoint projection | The generated view that joins model definitions to provider offerings. |  | approved | `pkg/catalogs/projection` |
| field-level | Applied to one field instead of a complete record. |  | approved | `docs/CATALOG_AUTHORITY_POLICY.md` |
| generation ID | The immutable logical identifier for one catalog generation. |  | approved | `docs/CATALOG_ARTIFACT_FORMAT.md` |
| generation_id | The JSON field that carries one catalog generation ID. |  | approved | `docs/REMOTE_CATALOG_PROTOCOL.md` |
| GET | The HTTP request method that retrieves a resource without changing it. |  | approved | `docs/REST_API.md` |
| GitHub | The service that hosts the Starmap repository and its release assets. |  | approved | `CONTRIBUTING.md` |
| human catalog workspace | The human-editable YAML tree that contains author and provider records. | human workspace | approved | `README.md` |
| human-readable | Written for direct use by a person. |  | approved | `README.md` |
| hop | One Starmap runtime inside a source chain. |  | approved | `pkg/catalogs/remote/chain.go` |
| HTTP | The application protocol that Starmap uses for its REST API and remote catalog service. |  | approved | `docs/REST_API.md` |
| ID | A value that uniquely identifies a catalog entity or generation in its scope. |  | approved | `docs/CATALOG_IDENTITY.md` |
| JSON | JavaScript Object Notation, the data format for Starmap API and catalog payloads. |  | approved | `docs/REST_API.md` |
| last-known-good | The most recent catalog generation that completed validation and activation. |  | approved | `docs/CATALOG_DISTRIBUTION_TRUST.md` |
| Kubernetes | The container orchestration system that runs the Starmap deployment example. |  | approved | `docs/DOCKER.md` |
| lease | The shared-store claim that permits exactly one replica to refresh. |  | approved | `runtime_lease.go` |
| manifest | The record that binds payload bytes, schema, compatibility, and source observations. |  | approved | `pkg/catalogs/generation_manifest.go` |
| model definition | The provider-independent identity and intrinsic facts for one model. |  | approved | `docs/CATALOG_IDENTITY.md` |
| models.dev | An external catalog source that supplies provider and model metadata. | models dev | approved | `internal/sources/modelsdev` |
| OpenAI-compatible | Implements the relevant OpenAI API contract for compatible clients. |  | approved | `docs/REST_API.md` |
| OpenAPI | The machine-readable specification for the Starmap REST API. |  | approved | `docs/openapi.yaml` |
| OpenRouter | An external model-routing service and optional Starmap data source. |  | approved | `internal/sources/openrouter` |
| opt-in | Enabled only after an explicit configuration or API choice. |  | approved | `docs/REMOTE_CATALOG_PROTOCOL.md` |
| payload digest | The SHA-256 digest of the exact catalog payload bytes. |  | approved | `docs/CATALOG_ARTIFACT_FORMAT.md` |
| projection | A derived view that does not own independent catalog authority. |  | approved | `docs/CATALOG_IDENTITY.md` |
| provider | A service that offers one or more models for inference. |  | approved | `pkg/catalogs/provider.go` |
| provider offering | One provider's service contract for a model definition. |  | approved | `pkg/catalogs/provider_offering.go` |
| provenance | Evidence that records the source and selection history of a catalog fact. |  | approved | `pkg/provenance/tracking.go` |
| readiness | The reported state that says whether a process serves a usable catalog now. |  | approved | `docs/REST_API.md` |
| reconciliation | The deterministic selection of catalog facts under the authority policy. |  | approved | `internal/catalog/reconciler` |
| read-only | Permits reads but does not permit changes. |  | approved | `docs/DOCKER.md` |
| remote subscriber | A client that follows remote events and activates validated catalog generations. |  | approved | `remote/subscriber.go` |
| repository | The version-controlled Starmap project tree. | repo | approved | `AGENTS.md` |
| review candidate | A provider offering that publication excludes until a person resolves its evidence. |  | approved | `pkg/catalogs/evidence/review_candidate.go` |
| runbook | An ordered operator procedure for one deployment task. |  | approved | `docs/ENTERPRISE_CATALOG_SERVER.md` |
| semantic digest | The SHA-256 digest of normalized catalog facts. |  | approved | `docs/CATALOG_IDENTITY.md` |
| S3 | The AWS object-storage service. |  | approved | `docs/CATALOG_STORE_CONTRACT.md` |
| S3-compatible | Implements the S3 operations that the catalog store requires. |  | approved | `docs/CATALOG_STORE_CONTRACT.md` |
| SHA-256 | The cryptographic hash function for Starmap payload and semantic digests. |  | approved | `docs/CATALOG_ARTIFACT_FORMAT.md` |
| Sigstore | The public signing project whose trusted root Starmap compiles into its binary. |  | approved | `internal/attestation/trustedroot.go` |
| source | A named origin that supplies catalog evidence. |  | approved | `pkg/sources/source.go` |
| source chain | The ordered hop list from one runtime to the origin of its catalog. |  | approved | `pkg/catalogs/remote/chain.go` |
| source observation | One source's identity, revision, time, checksum, and catalog evidence. |  | approved | `pkg/sources/observation.go` |
| SSE | Server-Sent Events, the HTTP stream format for remote catalog notifications. |  | approved | `docs/REMOTE_CATALOG_PROTOCOL.md` |
| Starmap | The Go library, command-line application, and server in this repository. | Star Map | approved | `README.md` |
| startup policy | The rule that decides what a runtime serves before its first upstream reply. |  | approved | `runtime_policy.go` |
| trusted root | The Sigstore document that names the keys and authorities a verifier accepts. |  | approved | `internal/attestation/trustedroot.go` |
| TUF | The Update Framework, the protocol that distributes a Sigstore trusted root. |  | approved | `internal/attestation/trustedroot.go` |
| URL | A uniform resource locator that names one network endpoint. |  | approved | `docs/REST_API.md` |
| YAML | A human-readable data format for Starmap catalog and configuration files. |  | approved | `README.md` |
