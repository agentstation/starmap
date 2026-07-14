# Microsoft Foundry and Azure OpenAI Source Contract

Status: P13.11 `DONE`  
Reviewed: 2026-07-12  
Identity: Azure SDK for Go `azidentity` v1.14.0

## Product boundaries

`microsoft-foundry` is a serving channel, not a model author. Azure Resource
Manager exposes available models under a subscription and location, while it
exposes deployments under a specific Cognitive Services/Foundry account. Those
scopes are meaningful:

- Location `models` results are sanitized into restricted public offerings. They
  establish that a model/version and deployment SKU were available to the
  observed subscription in that location, but they do not establish universal
  Azure availability.
- A public offering is discoverable-only. Azure inference addresses a customer
  deployment name, not the catalog model/version, so Starmap does not invent a
  globally routable alias or endpoint.
- Account `deployments` results are part of the configured credential-scoped
  source. Subscription, resource group, account, deployment alias, and account
  endpoint remain ordinary contextual offering facts and make the complete
  observation ineligible for public publication.
- Microsoft Foundry/Azure OpenAI remains distinct from the model author. The
  returned model `publisher` supplies authorship evidence; `format` is only a
  fallback when publisher is absent. An OpenAI-compatible format never turns a
  Mistral, Cohere, Meta, or Microsoft model into an OpenAI-authored definition.
  The offering remains owned by `microsoft-foundry`.
- Model version is part of the offering identity (`name@version`). SKU names are
  named deployment modes, not separate canonical definitions. When a deployment
  legally omits its model version, it resolves only through the one model-list
  record explicitly marked `isDefaultVersion`; multiple default claims fail
  closed.
- Commercial Azure and Azure Government have separate ARM endpoints, Microsoft
  Entra authority hosts, token audiences, and realm IDs. Results never merge the
  two realms implicitly. Azure Retail Prices currently covers commercial cloud
  only, so the Government source does not inherit commercial rates.

The optional default source reads `AZURE_SUBSCRIPTION_ID`,
`AZURE_RESOURCE_GROUP`, `AZURE_FOUNDRY_ACCOUNT`,
`AZURE_FOUNDRY_LOCATION`, and (for private inventory only)
`AZURE_FOUNDRY_ENDPOINT`. Missing configuration or credentials produces a typed,
degraded empty observation and does not make credential-free catalog generation
fail.

## Primary evidence

| Fact | Primary source | Contract consequence |
| --- | --- | --- |
| Control-plane versions and purpose | [Azure OpenAI REST API reference](https://learn.microsoft.com/en-us/azure/foundry/openai/reference) | Use a stable ARM control plane for account model/deployment inventory; do not confuse it with inference/data-plane model listing |
| Location model inventory | [Models - List](https://learn.microsoft.com/en-us/rest/api/aiservices/accountmanagement/models/list?view=rest-aiservices-accountmanagement-2024-10-01) | Call the subscription/location route and treat nested publisher, model name, version/default marker, format, lifecycle, and SKUs as scoped availability evidence |
| Account deployment inventory | [Deployments - List](https://learn.microsoft.com/en-us/rest/api/aiservices/accountmanagement/deployments/list?view=rest-aiservices-accountmanagement-2024-10-01) | Keep deployment name, scale/SKU, resource endpoint, and alias in the credential-scoped catalog observation |
| Inference aliases | [Azure OpenAI REST reference](https://learn.microsoft.com/en-us/azure/foundry/openai/reference) | Inference paths use `deployments/{deployment-id}`; a base model/version alone is not a routable Azure target |
| Foundry v1 model surface | [Azure OpenAI models](https://learn.microsoft.com/en-us/rest/api/microsoft-foundry/azureopenai/models) | The endpoint model list is authenticated and endpoint-scoped; it is not a credential-free global catalog |
| Microsoft Entra chain | [Credential chains in Azure Identity for Go](https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/credential-chains) | Use `DefaultAzureCredential` with injected API fixtures and serialize no token or credential value |
| Physically separate government cloud | [Azure Government documentation](https://learn.microsoft.com/en-us/azure/azure-government/) | Preserve a separate realm, ARM endpoint, authority, and audience rather than treating Government as another commercial region |
| Public retail-price semantics | [Azure Retail Prices REST API](https://learn.microsoft.com/en-us/rest/api/cost-management/retail-prices/azure-retail-prices) | Use the unauthenticated, paginated feed; retain USD token rates by region/SKU mode; do not apply its commercial-only rates to sovereign realms |

## Discovery, failure, and freshness policy

The source uses `2024-10-01` for both location model and account deployment inventory.
ARM `nextLink` pagination is bounded at 32 pages and 10,000 records, rejects
repeated cursors, and rejects a cursor outside the configured ARM realm. The
shared Starmap retry policy owns 429/5xx retry and bounded jitter; the HTTP client
has the repository default timeout and all response bodies use the repository
source-payload limit.

A failed or partial account observation is not deletion evidence. Reconciliation
retains the last-known-good Foundry offerings and records degradation. Account
deployment observations have an independent contextual lifecycle and never enter
scheduled public publication.

The public Retail Prices importer filters `serviceName eq 'Foundry Models'`,
follows at most 32 first-party pages, accepts only USD `Consumption` token meters
with explicit `1K` or `1M` units, and converts them to exact per-token/per-million
rates. It retains rates in `retail/{region}/{sku-mode}` offering modes. Ambiguous
sessions, calls, images, capacity, and meters that cannot map uniquely to one
observed model are counted as ignored instead of guessed. Retrieval time is
reported independently as pricing freshness.

## P13.11 closeout gate

- deterministic model/deployment pagination and conversion fixtures;
- bounded retry and repeated-cursor fault fixtures;
- commercial/Government realm separation;
- zero account calls when required credential or account bindings are absent;
- public-payload absence checks for subscription, resource group, account,
  endpoint, and deployment alias;
- Retail Prices parsing and live public-feed evidence;
- credential-aware live account evidence or an explicit configuration/credential
  skip with no secret output;
- focused race, repository short race, vet, generated docs, catalog validation,
  and diff checks before advancing to P13.12.
