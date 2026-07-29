# P5 Author-Model Corpus Disposition

Date: 2026-07-28  
Baseline: `9609f4f4a74281a7f9692a97cc4926df5978d754`

## Finding

The embedded catalog persisted the same model concept in provider model files
and in 322 author model files. Author files were not an independent source:
they copied provider/model data, became stale separately, and made a
provider-independent definition appear authoritative without preserving the
provider offering that supplied it.

The baseline corpus contained:

- 322 author model records with 322 unique IDs;
- 200 IDs with an exact surviving provider-model record;
- 122 author memberships that cannot be reproduced as exact provider-backed
  membership;
- 311 additional memberships derived from current inline author metadata and
  explicit author attribution rules.

The 121 author-model IDs without an exact provider-model ID were reviewed as:

| Class | Count | Disposition |
| --- | ---: | --- |
| Alias/content overlap with a surviving provider record | 32 | Remove the duplicate; the provider record remains canonical |
| Present in the checked-in models.dev observation but absent from provider records | 25 | Do not promote into embedded truth; a current dynamic source may introduce the record during reconciliation |
| No surviving provider overlap or checked-in models.dev evidence | 64 | Remove as stale orphan data |

Eight of the 25 models.dev-only IDs name configured providers:
`deepseek-chat`, `deepseek-reasoner`, `gpt-4`, `gpt-4-turbo`, `o1-pro`,
`o3-deep-research`, `o3-pro`, and `o4-mini-deep-research`. They are not copied
into provider YAML merely to preserve the old mirror. A provider API or
models.dev observation may add them through the normal authority and
reconciliation pipeline, with provider availability remaining unknown unless a
source supplies it.

## Decision

Literal equality with the former author-model tree is rejected: it would
preserve a second source of model truth and reintroduce the architecture P5 is
removing.

Author membership is instead derived from:

1. author IDs carried by canonical provider model records; and
2. explicit `authors.yaml` attribution rules, including provider-scoped and
   global patterns.

`Provider.Catalog.Authors` remains acquisition scope and is never treated as
authorship evidence, even when it contains one author. Malformed attribution
patterns fail publication with a typed parse error.

Catalog schema version 2 therefore removes `Author.Models`, the
`author_models` payload collection, all embedded `authors/*/models/*.yaml`
files, and the prelaunch compatibility/migration adapters that depended on
them.

## Verification contract

- provider YAML and `provider_models` are the only persisted model records;
- build/save/load derives the same provider-backed attribution deterministically;
- author queries work through `Catalog.AuthorModels`;
- exact provider offerings retain provider-specific price, limits, lifecycle,
  endpoint, modes, and request values;
- unknown availability and lifecycle are not promoted to known facts;
- dynamic sources may add missing records through reconciliation, but the
  embedded catalog does not manufacture provider offerings from an old author
  mirror.
