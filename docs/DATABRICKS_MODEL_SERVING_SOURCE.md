# Databricks Mosaic AI Model Serving Source

Status: P13.22 implementation evidence

## Product boundary

Databricks exposes a public support matrix and workspace-scoped serving
products. Starmap keeps them separate:

- [Supported foundation models](https://docs.databricks.com/aws/en/machine-learning/model-serving/foundation-model-overview)
  is the current first-party region/support authority for pay-per-token,
  AI Functions, and provisioned-throughput model families.
- [Foundation Model APIs](https://docs.databricks.com/aws/en/machine-learning/foundation-model-apis)
  documents Databricks-hosted pay-per-token and provisioned-throughput modes.
- [Foundation model REST API](https://docs.databricks.com/aws/en/machine-learning/foundation-model-apis/api-reference)
  documents the workspace endpoint contract and OpenAI-compatible request
  formats.
- [External models](https://docs.databricks.com/aws/en/machine-learning/foundation-models/external-models)
  documents third-party upstream provider/model configuration and required
  multi-model traffic configuration.
- [Multiple served models](https://docs.databricks.com/gcp/en/machine-learning/model-serving/serve-multiple-models-to-serving-endpoint)
  documents served-entity aliases, traffic percentages, and direct entity
  invocation independently from the endpoint alias.

## Public availability

The source extracts only exact `databricks-*` endpoint IDs from the current
first-party support matrix, requires a non-trivial result, sorts and deduplicates
it, and maps model authors by documented family. These are discoverable
foundation offerings. Databricks invocation URLs are workspace-specific, so the
public support list never receives an invented global endpoint or customer
workspace URL.

## Private workspace inventory

Workspace discovery is opt-in and calls the paginated
`GET /api/2.0/serving-endpoints` contract with a private bearer token. The
workspace ID, host, endpoint name, served-entity name/version, external upstream
provider/model, traffic percentage, region, and invocation URL remain only in
`CustomerInventory`. External model provider names and traffic aliases are not
canonical model IDs.

Because workspace entities can be custom, fine-tuned, versioned, or external,
the caller must supply an explicit entity-to-definition mapping. An unknown
entity, repeated page cursor, page-limit breach, null endpoint array, insecure
non-loopback host, or malformed response fails closed. Tokens and hosts have
explicit non-serializable configuration fields.

Fixtures prove public support parsing/non-routability, pagination, bearer auth,
external and managed entity separation, traffic-alias retention, workspace
isolation, explicit mapping, missing mapping failure, and absence of private
identity from public records. No workspace live call is attempted without host,
token, workspace ID, and mappings.
