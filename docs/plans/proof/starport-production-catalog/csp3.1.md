# CSP3.1 offering lookup allocations

CSP3.1 is in progress. Its CSP0.4 dependency is complete.
The immutable catalog currently calls `Providers.Resolve` for every offering lookup.
That call copies the complete provider, including all its models, before selecting one offering.

Capture canonical and alias lookup allocations across increasing provider model counts.
Then build an immutable provider identity index and preserve caller-owned results, aliases, empty providers, and missing-record errors.
The task requires package race tests, allocation benchmarks, and its product verifier checks.

CSP3 awaits the owner decision on account-specific removals and explicit inference scope links.
This independent task changes lookup cost, not catalog membership or source authority.
