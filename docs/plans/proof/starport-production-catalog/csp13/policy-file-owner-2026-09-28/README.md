# Inference policy files during restore

Starmap now exposes its native check for pending file publications.
The check leaves files unchanged and requires the caller to fence writers.
It does not recover a journal or approve a deployment.

The Starport candidate validates retained inference policy through its credential owner.
It checks the recorded product, deployment, instance, schema, provider filename, and canonical JSON.
It rejects pending journals, unknown files, missing defaults, and another replica's policy.
Invalid files remain unchanged.

The file-tree test preserves accepted provider policy and the legacy default through publication and exact retry.
The candidate uses a temporary local module file.
Final checks require a published Starmap module.

The full producer check started before the additive inspection API commit.
The proof separates that run from the repeated checks for the changed package.
Native checks, review, publication, and complete deployment recovery remain open.
