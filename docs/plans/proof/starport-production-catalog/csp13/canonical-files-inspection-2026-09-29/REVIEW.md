# Passive canonical-file inspection

Worktree: `/private/tmp/starport-canonical-files-inspection-20260929`.
Branch: `codex/canonical-files-passive-inspection`.
Base: `f3a8a2848615d815ea46e334878c2cc4b5389088`.

## Contract

The publication owner records both the target directory identity and its parent identity in `FileTreeResult`.
Publication and reuse check that the same native parent remains selected through completion.
The app must retain the actual successful publication result before releasing any native component.
Historical results without the parent identity cannot establish the new passive proof.

`RestoreSource.InspectPublishedFileTree` checks the original successful result against the verified source selection and native target.
It returns a private `VerifiedFileTree` capability.
Its bounded record binds the source manifest, destination, exact selection, file count, original identities, and original publication result.
The record contains no selected file contents or validation time.

The app must seal `Record()` and `Digest()` in the immutable activation decision before the first component release.
`InspectRetainedFileTree` requires that original digest and record during restart.
It checks canonical record bytes, the source-derived selection, both original native identities, every selected file, and the app's semantic validator.
It refuses unknown records, changed selection, extra files, absent files, altered bytes, and unresolved publication journals.
Checks before and after the semantic validator preserve native identity and publication continuity.

The passive path calls no preparation, publication, sync, repair, permission grant, or clock owner.
It cannot recreate a missing original identity or renew historical evidence.
The owner validator must check component semantics without changing files.
External writer fencing remains the operator's responsibility.

## Ownership

Recovery owns exact tree selection and passive native checks.
App owns canonical roles, allowed dispositions, canonical locations, and the current semantic validators.
The app must retain the original role result and bind it to the operation and immutable activation decision.
A supplied digest or diagnostic result alone grants no activation authority.

The capability grants no current catalog permission, administrator access, SQL approval, or gateway admission.
The full coordinator must still verify those owners.
The implementation changes only the publication owner's new identity field and native checks, plus new passive inspection source and tests.

## Evidence

The tests exercise native local and shared backup fixtures.
They reopen original record bytes after a real KV component release and leave the SQL witness closed.
They detect a replaced parent even when the original leaf identity and file bytes remain unchanged.
They preserve unresolved publication journal bytes and reject forged records even with a recomputed supplied digest.
Existing publication and process-loss tests retain their coverage.

The evidence directory retains the initial compile failure and the initial complexity failure.
The initial app regression selector matched no tests. Its result provides no behavioral evidence.
The corrected selector tests the actual acquisition policy, runtime, baseline, and shared file publication owners.
Native Linux and Windows qualification remains with the parent integration and CI.
The parent owns structured review and publication.
