# P8 Production File Modularity Review

Date: 2026-07-29

Scope: P8.3 and F-021

## Decision standard

This review applies the repository file-size policy together with module depth,
conceptual locality, the deletion test, and the rule that an interface needs
real substitutability. A line-count reduction alone is not a reason to split a
cohesive module. An extraction is justified when it gives a named concept its
own invariant and test surface without adding a pass-through package, interface,
or public API.

The protected-main baseline named four production files for review:

| File | Protected-main lines | Reviewed-head lines | Disposition |
| --- | ---: | ---: | --- |
| `internal/providers/google/client.go` | 1,206 | 626 | Extract wire decoding and model normalization in the same package |
| `internal/providers/openai/client.go` | 1,183 | 558 | Extract wire decoding and configured mapping policy in the same package |
| `pkg/reconciler/merger.go` | 1,134 | 915 | Retain as one deep authority/provenance implementation |
| `pkg/differ/differ.go` | 1,000 | 978 | Retain as one catalog-difference algorithm pending the separate P8.6 deletion audit |

The higher current pre-review measurements of 1,255 lines for Google and 1,221
lines for OpenAI resulted from source-resilience work after the protected-main
baseline. Those additions made the conceptual boundaries clearer rather than
creating a new public abstraction.

## Google provider client

### Before

One 1,255-line file contained three distinct concerns:

1. Google AI Studio response envelopes, bounded record decoding, unknown-field
   evidence, and per-record quarantine;
2. client configuration, credentials, backend choice, network pagination, and
   lifecycle; and
3. provider model identity, feature inference, GenAI/REST conversion, Model
   Garden normalization, and merge behavior.

### Disposition

Keep one `google` package and its existing concrete `Client`, but split the
implementation by those concepts:

| File | Lines | Local invariant |
| --- | ---: | --- |
| `client.go` | 626 | Caller-visible client lifecycle, credentials, backend selection, and model-list acquisition |
| `wire.go` | 80 | Bounded AI Studio wire decoding with unknown-field and record-quarantine evidence |
| `models.go` | 570 | Deterministic conversion from Google records to Starmap model observations |

No interface, package, constructor, exported symbol, or dependency was added.
The provider factory remains the real adapter boundary. Keeping these files in
one package preserves access to the concrete client state while making wire
drift and normalization independently reviewable.

## OpenAI-compatible provider client

### Before

One 1,221-line file contained three distinct concerns:

1. OpenAI-compatible response DTOs, bounded decoding, unknown-field evidence,
   and numeric-string pricing parsing;
2. client configuration, transport, provider defaults, pricing, features, and
   source-extension projection; and
3. configured field mapping, author resolution, feature-rule application, and
   mapping validation.

### Disposition

Keep one `openai` package and its existing concrete `Client`, but split the
implementation by those concepts:

| File | Lines | Local invariant |
| --- | ---: | --- |
| `client.go` | 558 | OpenAI-compatible acquisition and conversion orchestration |
| `wire.go` | 203 | Bounded provider wire decoding and tolerant numeric pricing syntax |
| `mapping.go` | 476 | Validated provider-configured mapping, author, and feature policy |

No interface, package, constructor, exported symbol, or dependency was added.
The split does not manufacture separate clients for compatible providers and
does not move configuration policy out of the concrete adapter that owns it.

## Reconciler merger

`pkg/reconciler/merger.go` is now 915 lines. Earlier P4/P5 work already
extracted independent model policy, provider policy, source identity, presence,
and authority concepts into neighboring files. The remaining file is one deep
implementation over shared merger state:

- baseline and observation evidence;
- provider/model merge orchestration;
- authority selection and conflict history;
- reflection-backed field access; and
- defensive copy and supplemental merge helpers.

These operations share the same authority reader, observation evidence,
baseline, and provenance tracker. Splitting them again would either duplicate
that state or create a shallow pass-through seam. The file stays intact. P8.5
will still inspect its measured complexity, and P8.6 will still apply the
residual deletion test to exported or strategy behavior; this review does not
prejudge either gate.

## Catalog differ

`pkg/differ/differ.go` was exactly 1,000 lines at the protected baseline and is
978 lines on this review head. It remains one deterministic algorithm configured
by `Differ` options and producing the model, provider, author, and combined
changesets defined by the neighboring `changeset.go`.

The resource-specific comparisons are mutually similar parts of the same
catalog-difference operation, not independently composed adapters. Splitting
them would add navigation without introducing a stronger invariant or test
boundary. The file stays intact for P8.3. The package has one production caller,
and `Changeset.Filter` has no production caller; P8.6 therefore retains the
separate obligation to decide whether the package or dead exported behavior
passes the deletion test.

## Verification

The same-package extraction preserves public and package-level behavior:

```text
go test -race ./internal/providers/google ./internal/providers/openai -count=10
ok github.com/agentstation/starmap/internal/providers/google 1.347s
ok github.com/agentstation/starmap/internal/providers/openai 5.494s

make test-file-sizes
1036 internal/providers/openai/client_test.go
1446 pkg/reconciler/merger_test.go
Go file-size verification passed: review >1000, justify >1500, fail >=2000 lines

git diff --check
(no output)
```

Every production file named by P8.3 now has a terminal extract-or-retain
disposition. No production Go file exceeds 1,000 lines on this head. The two
remaining files listed by the executable gate are tests owned by P8.7.
