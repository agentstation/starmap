# IBM watsonx.ai Source

Status: P13.24 implementation evidence

## Public regional inventory

- [IBM watsonx.ai REST API](https://cloud.ibm.com/apidocs/watsonx-ai)
  defines the dated regional `GET /ml/v1/foundation_model_specs` contract,
  opaque `next.href` pagination, foundation-model tasks and lifecycle records.
- [Supported foundation models](https://dataplatform.cloud.ibm.com/docs/content/wsj/analyze-data/fm-models.html?context=wx)
  distinguishes immediately available provided models from deploy-on-demand
  models and documents that availability varies by data-center region.
- [Foundation model deployment methods](https://dataplatform.cloud.ibm.com/docs/content/wsj/analyze-data/fm-model-deployment-methods.html?context=wx)
  defines provided, deploy-on-demand, custom, and prompt-tuned hosting and
  billing boundaries.

Starmap calls the configured regional origin with API version `2024-03-14`, a
200-record page size, bearer authentication, and same-origin opaque cursors.
Returned provided models become public `curated-multitenant` pay-per-token
offerings. The configured region is canonical offering geography. Text, chat,
and code tasks map to the native watsonx generation contract; embedding and
similarity tasks map to embeddings. Unknown tasks remain discoverable-only.
Lifecycle and model limits are source-native and malformed or duplicate records
fail the source rather than publishing a partial generation.

## Credential-scoped deployments

IBM's dated `GET /ml/v4/deployments` contract requires exactly one project or
deployment-space scope. Starmap exposes this only through opt-in
`FetchDeployments`; it never enters public source generation. Project/space ID,
deployment ID and name, serving name, regional invocation URL, asset ID, and
aliases become ordinary contextual offerings in the caller's catalog. Every
returned model or asset must have an explicit canonical-definition mapping,
preventing a scoped asset
name from silently becoming a public definition.

Deployment types remain distinct:

- `curated_foundation_model` is customer-exclusive on-demand dedicated hosting;
- `custom_foundation_model` is customer-managed dedicated hosting;
- `foundation_model` is a prompt-template deployment over provided multitenant
  hosting.

The credential-scoped path uses bounded same-origin pagination, validates exactly one
scope, rejects unmapped assets, and never serializes its token.

## Failure and live policy

Fixtures prove dated public requests, bearer auth, opaque pagination, task and
lifecycle mapping, regional projection, unknown-task non-routability,
cross-origin cursor rejection, invalid limits, scoped project pagination,
on-demand/custom separation, explicit mapping, and credential isolation. Live
public and credential-scoped discovery are explicitly skipped when no IBM regional base
URL, token, project/space, and mapping are configured. No generic global
endpoint or globally public deployment is inferred.
