# Origin runtime integration inspection

This inspection uses Starmap commit `8a71193bc7587a025231be0d72f2002327fd67be`.
It changes no implementation or acceptance status.

The runtime stages an input-publication journal before committing the serving generation.
The journal names the candidate generation ID and exact payload checksum.
See `runtime/publication_retention.go:99` and its later call to `r.commit` at line 117.

Origin preparation derives a different authority generation ID from the ordinary source manifest and publication sequence.
An origin transform inserted after journal staging would therefore record the wrong intended generation.
Origin integration must prepare the final authority identity before staging that journal, then preserve conditional publication and recovery.

The client currently constructs ordinary manifests inside its private `newGeneration` method.
That method assigns a synchronization-run ID and publication time. See `generation.go:224`.
A preparation boundary must preserve those exact values through journal staging, retries, store publication, and serving-client activation.

The runtime also uses layer-derived identities to skip unchanged work and recover retained inputs.
Origin integration must distinguish source-input identity from the authority publication identity without inventing a new generation after an unchanged restart.
The required checks must cover failed publication, failed activation, crash recovery, concurrent writers, unchanged restart, and explicit authority bootstrap.

The current `PublishCatalog` library combines preparation and commit.
The runtime needs access to the prepared final identity before its journal write.
Select that boundary after inspecting every commit caller and input-publication recovery path.
Preserve the existing source-policy guard, lease fence, passive construction contract, and independent current-head receipt reader.
