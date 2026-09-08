# Composite catalog provenance

Metadata, service modes, and extension fields retain separate source evidence for each accepted contribution. Explicit null map values remain distinct from omitted keys. Mode pricing remains one commercial record. Quoted map keys prevent ambiguity between namespaces, mode names, and field names.

Present empty records also retain evidence. Presence entries cover metadata, architecture, modes, provider overrides, and extension namespaces. A denied carried record cannot preserve an empty wrapper. An explicit operator edit can supply an accepted field under the existing projection policy.

Baseline merges preserve omitted mode headers and body fields. Existing tags occur once. Stale fallback data cannot replace accepted baseline facts or claim their summary receipt. Returned nested values remain independent of source objects.

Pricing comparison accepts JSON and YAML spellings for top-level and mode pricing. Arbitrary request-body and extension keys retain their original meaning. Dynamic JSON numbers preserve numeric types and large integer precision. A numeric value remains distinct from a string with the same characters.

The runtime test covers 42 retention and replacement scenarios. It checks publication, filesystem persistence, restart, source refresh, exact contributing receipts, generation degradation, and unchanged observation history. Cases include false and unknown values, dates, architecture facts, tags, free and paid mode pricing, headers, request bodies, nested extensions, and dotted keys.

The payload and YAML workspace test checks seven contribution receipts in each format. Separate tests cover stale fallback, projection refusal, explicit edits, baseline retention, caller ownership, price-path boundaries, and numeric identity. The final minimum-Go checks pass 83 events. Ago reports zero findings, stale ignores, and errors. Package lint reports zero issues.

## Failure evidence

| Check | Observed failure and repair |
| --- | --- |
| Original composite recovery | Old production code records 36 failure events against the first 34 scenarios. Separate field receipts repair the lost attribution. |
| Empty records | An intermediate candidate records eight failure events for six empty-record scenarios. Explicit presence receipts retain those records. |
| Stale fallback | Two failure events expose replacement of accepted metadata, modes, and extensions. The repair preserves the baseline and rejects false summary attribution. |
| Baseline fill | Two failure events expose duplicate tags and missing prior mode overrides. The repair preserves omitted fields and deduplicates tags. |
| Payload mode pricing | Four failure events expose a mode price that becomes local evidence. The YAML cases pass. Mode-pricing alias normalization repairs the payload path. |
| Numeric aliases | Five failure events expose JSON numbers that become strings during comparison. Dynamic JSON normalization preserves numeric identity. |

The [verification report](verification.json) retains event counts and package results. Counts include parent tests and package events. Intermediate compiler diagnostics remain separate from behavioral failures. The paid-price runtime probe passes before the alias repair. That result did not cover the payload-to-local projection that the workspace test exposed.

The earlier broad suites pass 912, 921, 922, and 925 events at their recorded stages. These runs precede the final alias repair. The 925-event run has no complete source snapshot. The final broad suite passes 957 events with no failures or skips. All three packages pass on the final source. Earlier results do not qualify later source revisions.

## Remaining CSP3 work

Authorship still records one aggregate receipt for several contributions. Its publication probe retains an author but drops the contributing receipt and clears degradation. An expanded probe records four failure events across retention and replacement cases. It confirms missing per-author receipts through filesystem restart and source refresh.

The architecture codec probe records six failure events and four passes. Missing or null architecture flags become false. Description and whole-record null failures also remain open. The [authorship candidate review](authorship-candidate-review.md) records an unapplied prototype and its remaining selection defect.

CSP3 still requires complete field presence, authorship evidence, scoped membership, removal review, application integration, and released-pair acceptance. No component or released-pair acceptance credit follows from these focused checks.
