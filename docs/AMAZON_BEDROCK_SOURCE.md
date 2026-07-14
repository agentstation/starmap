# Amazon Bedrock Source Contract

Status: P13.10 `DONE`  
Reviewed: 2026-07-12  
SDK: AWS SDK for Go v2, `service/bedrock` v1.65.0

## Product boundaries

`amazon-bedrock` is a provider channel, not a model author. Foundation-model
records become canonical definitions attributed to the returned `providerName`
and Bedrock provider offerings keyed by the exact `modelId`.

- `ListFoundationModels` is swept per configured source region. Equal model IDs
  merge into one offering with a stable region set.
- The default commercial sweep covers all 32 `bedrock-runtime` regions in AWS's
  reviewed regional endpoint table. The two GovCloud regions are exposed as a
  separate inventory because they use a distinct realm and access boundary.
- `SYSTEM_DEFINED` inference profiles are public provider offerings. Their exact
  profile ID is routable, their source regions are the regions where the profile
  was observed, and destination regions come from model ARNs.
- `APPLICATION` inference profiles are account-owned cost-attribution resources.
  The configured credential-scoped source returns them as ordinary canonical
  offerings in the caller's contextual catalog. Their account ARN and profile
  ID make the complete observation ineligible for public publication.
- Regional, geographic cross-region, global cross-region, on-demand, and
  provisioned distinctions are provider-offering facts. They never modify a
  provider-independent model definition.
- `ListFoundationModels` supplies modalities, streaming support, and inference
  types. The native source maps those exact fields, routes on-demand records
  through `InvokeModel`, and leaves provisioned-only base records discoverable
  until customer provisioned-throughput inventory supplies an invocable ARN.
  It does not infer Converse, Messages, Chat Completions, or Responses support
  from a model name.

## Primary evidence

| Fact | Primary source | Contract consequence |
| --- | --- | --- |
| Foundation model response fields and lifecycle | [ListFoundationModels API](https://docs.aws.amazon.com/bedrock/latest/APIReference/API_ListFoundationModels.html) | Require model ID, name, provider name, inference types, and lifecycle; treat the response as region-scoped |
| Profile pagination and type boundary | [ListInferenceProfiles API](https://docs.aws.amazon.com/bedrock/latest/APIReference/API_ListInferenceProfiles.html) | Bound `nextToken` pagination at 32 pages/10,000 records; query system and application profiles separately |
| Source and destination region semantics | [Supported regions and models for inference profiles](https://docs.aws.amazon.com/bedrock/latest/userguide/inference-profiles-support.html) | Source region is the control-plane region; destination regions are parsed from `Get/ListInferenceProfiles` model ARNs |
| Geographic residency | [Geographic cross-Region inference](https://docs.aws.amazon.com/bedrock/latest/userguide/geographic-cross-region-inference.html) | Keep geography scope and destination regions explicit; never imply single-region residency |
| Global routing mutability | [Global cross-Region inference](https://docs.aws.amazon.com/bedrock/latest/userguide/global-cross-region-inference.html) | Global destinations are observations, not a permanent closed set |
| Application profile ownership | [Application inference profiles](https://docs.aws.amazon.com/bedrock/latest/userguide/inference-profiles.html) | Application profiles are credential-scoped cost-attribution offerings, not publicly publishable offerings |
| Credential resolution | [Configure the AWS SDK for Go v2](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html) | Use `config.LoadDefaultConfig`, its concurrency-safe credential cache, and injected client factories; serialize no credentials |
| Complete runtime region inventory | [Regional availability by endpoints](https://docs.aws.amazon.com/bedrock/latest/userguide/endpoints-region-availability.html) | Sweep 32 commercial runtime regions by default and expose the two GovCloud regions as a separate realm-specific sweep |
| Endpoint/API compatibility is model-specific | [Get list of models](https://docs.aws.amazon.com/bedrock/latest/userguide/models-get-info.html) and [Endpoints supported by Bedrock](https://docs.aws.amazon.com/bedrock/latest/userguide/endpoints.html) | Treat native `InvokeModel` as the discovered runtime contract; add Converse/OpenAI/Anthropic-compatible APIs only from explicit model-level evidence |
| Pricing authority | [Amazon Bedrock pricing](https://aws.amazon.com/bedrock/pricing/) | Pricing is region/model/mode-specific first-party evidence and must be curated with retrieval time; the model-list API does not supply prices |
| Machine-readable pricing | [AWS Price List concepts and endpoints](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/price-changes.html) | Fetch the public `AmazonBedrockFoundationModels` bulk offer, bind its version/publication date/ETag, and parse bounded on-demand token SKUs |

## Failure and freshness policy

The SDK's internal retry count is set to one. Starmap's shared provider policy
owns bounded retry/jitter and retries 429/5xx failures; shared cursor pagination
rejects repeated cursors and page/record overflow. A failed regional sweep is not
evidence that offerings were deleted. Publication must retain the last-known-good
Bedrock observation and mark provider/region coverage degraded.

Public regional discovery uses the regional-control-plane freshness budget.
Application profiles use the credential-scoped observation budget and are never
part of scheduled public generation. Pricing has its own observation timestamp
and is stale independently of model availability.

The pricing importer keeps rates in named per-region modes (`regional`,
`global`, `global_batch`, `batch`, `latency_optimized`, and one-hour cache
variants). It never copies a region-dependent rate into the offering-wide
`Pricing` field. Unsupported non-token and provisioned-capacity SKUs are counted
as ignored rather than guessed into token units. The 2026-07-12 live proof read
Price List version `20260703085857`, publication time
`2026-07-03T08:58:57Z`, accepted 3,487 token SKUs, and explicitly ignored 1,396
other SKUs.

## P13.10 closeout

Focused race, repository-wide short race, vet, generated docs, catalog generation,
public live pricing, and credential-aware live-or-explicit-skip evidence pass. The
durable evidence record is in the P13 control-plane log.
