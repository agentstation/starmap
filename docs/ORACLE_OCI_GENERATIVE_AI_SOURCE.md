# Oracle OCI Generative AI Source

Status: P13.25 implementation evidence

## Regional discovery and invocation

- Oracle's [Generative AI Go SDK](https://docs.oracle.com/en-us/iaas/tools/go/latest/generativeai/index.html)
  defines native signed `ListModels` and `ListEndpoints` operations, OCI
  `opc-next-page` pagination, model capabilities/lifecycle/type, and dedicated
  endpoint identity. Starmap pins OCI Go SDK v65.120.0 (released July 7, 2026).
- [Generative AI Regions](https://docs.oracle.com/en-us/iaas/Content/generative-ai/regions.htm)
  defines OCI region identifiers and the OC1 commercial realm.
- [On-Demand and Dedicated Modes](https://docs.oracle.com/en-us/iaas/Content/generative-ai/modes.htm)
  distinguishes pay-as-you-go base-model invocation from customer-exclusive
  dedicated AI clusters.
- [OCI OpenAI-Compatible Endpoints](https://docs.oracle.com/en-us/iaas/Content/generative-ai/openai-compatible-api.htm)
  defines the regional `/openai/v1` base and OCI authentication. Starmap emits
  the Responses contract only for the exact currently documented Google,
  OpenAI gpt-oss, and xAI IDs; other chat/embed/rerank capabilities retain the
  native OCI inference contract.

The optional source requires `OCI_REGION`, `OCI_COMPARTMENT_ID`, and the OCI SDK
default credential chain. Each base model is a regional OC1 on-demand offering.
Capabilities become typed invocation contracts; inactive or retired models are
unavailable. Unknown capabilities remain discoverable-only. Model pages use the
shared retry and bounded pagination policy.

## Credential-scoped endpoint isolation

Fine-tuned custom model records do not become globally public definitions or
offerings. The configured credential-scoped source includes `ListEndpoints`
results as ordinary contextual offerings: compartment, endpoint, model,
dedicated-cluster and private-endpoint OCIDs plus display aliases remain in the
current caller context. Every endpoint must resolve to a model returned by the
same bounded observation, and the observation is rejected before public writes.

## Pricing

The current [Oracle Cloud PaaS and IaaS Global Price List](https://www.oracle.com/asean/a/ocom/docs/corporate/pricing/oracle-paas-and-iaas-global-price-list.pdf)
was text-extracted and visually verified on pages 9, 11, 13, and 14. Starmap
maps only exact token-priced IDs disclosed there:

- Google Gemini 2.5 Pro, Flash, and Flash Lite, including Pro's >200K tier;
- OpenAI gpt-oss-120b and gpt-oss-20b;
- xAI Grok 4.3 input, cached input, and output, including its >200K tier.

All values are USD per million tokens. Unknown model IDs remain unpriced.
Cohere transaction metrics and dedicated AI-unit/cluster-hour rates remain
source evidence only because they cannot be safely converted into per-request
or token prices. No unit conversion is invented.

## Failure and live policy

Fixtures prove multi-page model discovery, exact OpenAI versus native endpoint
selection, regional/realm projection, lifecycle, exact/tier/cache pricing,
custom-model exclusion, private endpoint isolation, no private calls during
public observation, explicit model resolution, duplicate/unmapped failure,
native failure propagation, and missing-configuration degradation. Live
discovery is explicitly skipped when OCI region, compartment, and SDK credential
configuration are absent.
