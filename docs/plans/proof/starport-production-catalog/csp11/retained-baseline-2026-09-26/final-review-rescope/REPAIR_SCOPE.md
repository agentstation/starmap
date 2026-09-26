# CSP11 review convergence checkpoint

Producer: `6fefd6d422c214520342485b9da2141e441d8911`.
Consumer: `509044a005323d75ce46830cf46494b04c0c0fc9`.
Status: pause the patch loop after two review-triggered repair cycles.
Neither branch received a push or merge during this review.
The approved D41 policy remains unchanged. No product decision remains open.

## Accepted P2: optional providers block ownership

`runtime.checkFleetAcquisition` puts every catalog provider in the implicit
required-provider list.

`providerSourceObserver.CheckFleetAcquisition` returns
credential resolution errors for each member. `auth.Resolver.ResolveCatalog`
returns an authentication error when a required credential profile is absent.
Ordinary provider acquisition instead classifies that provider as not configured.

Thus a catalog provider without local acquisition credentials can prevent lease
acquisition or renewal. Catalog membership does not establish a required
acquisition scope. This can occur on startup or after upstream catalog growth.

Repair ownership: acquisition owns credential capability classification.
Runtime owns required retained scopes and explicit deployment bindings.
Separate optional discovery candidates from required acquisition scopes at this
boundary. Do not ignore failure for retained scopes or explicit bindings.
Keep capability checks free of provider inventory requests and secret digests.

Required regression evidence: an upstream update adds an unconfigured provider
without blocking ownership or later refresh. A lost required credential still
refuses ownership. Test both paths with the actual acquisition resolver.

## Accepted P1: live takeover omits publication

`execute` can get a new grant for an existing follower. An unchanged source
read then returns success without publishing. The shared head still carries the
previous owner's grant. Startup and provider acquisition already check whether
ownership requires publication. The unchanged source path does not.

Starport's `FleetStore.AcceptPublication` requires the original publication grant
when accepting a new candidate. Thus a candidate that was not accepted before
takeover cannot advance through route acceptance under the new owner's grant.
An already accepted head has an idempotent early return and is not affected by
this acceptance failure. Do not claim that every existing request fails.

Repair ownership: runtime owns publication under the current original grant.
Apply this invariant to every successful refresh operation, including unchanged
source data. Keep the native store's grant, predecessor, and acceptance checks.
Do not weaken those checks to make the retained head acceptable.

Required regression evidence: keep a follower running, lose its leader, get
ownership, receive unchanged source data, and publish the retained content under
the new grant. Verify that native storage accepts the new publication and rejects a delayed
old-owner commit. Preserve pins, recovery inputs, and authority permission limits.

## Next checkpoint

Before another code patch, define the two operation contracts and register their
behavioral tests in CSP11. Capture both failures, then repair their owners.
Run the eleven task subcases with native storage, affected race suites, and
required repository checks. Commit and repeat the complete pre-PR review.
Publish only after convergence. Pin and qualify the published consumer module,
then merge Starmap PR185 before Starport PR385.

The autoreview skill requires this pause when two repair cycles do not converge.
This checkpoint does not request new implementation or merge permission.
