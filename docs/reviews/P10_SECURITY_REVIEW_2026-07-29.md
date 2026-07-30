# P10 Production Security Review

Date: 2026-07-29
Scope: P10.6 production candidate after P10.5
Status: PASS after remediation

## Outcome

The supported Starmap library, CLI, server, remote-consumer, generation-store,
artifact, acquisition, and release compositions pass the control-plane security
scope. Six concrete defects or hardening gaps were corrected in the same
coherent unit. No unresolved P0 or P1 security finding remains.

Starmap is not an authorization server, TLS terminator, secret manager, or
network sandbox. A deployment that binds beyond loopback still owns TLS,
network policy, secret injection, and the decision to enable API-key
authentication. Provider endpoints and optional S3 clients are trusted
operator configuration; no HTTP request can supply an arbitrary acquisition or
object-storage URL.

## Review matrix

| Boundary | Evidence and disposition |
| --- | --- |
| Credentials | Runtime provider values remain `json:"-" yaml:"-"`; durable source links omit diagnostic messages; server auth logs only whether a key was supplied; remote health exposes classified, secret-free state. Query-authenticated provider transport errors now discard the URL-bearing error graph, and recovery logs only the panic type. Repository secret-pattern scan found no production credential. |
| Redirects | Remote catalog redirects remain exact-origin only and HTTPS publisher responses require a verified certificate chain. Provider acquisition now refuses every redirect, so header or query credentials are never replayed to a redirect target. |
| SSRF | Remote publisher URLs reject credentials, queries, fragments, unsafe schemes, and non-loopback plaintext HTTP. Production models.dev HTTP/Git origins are fixed; provider endpoints and S3 endpoints are explicit operator/caller configuration; server update inputs select only registered provider/source identities and cannot provide a URL. |
| Authentication | Public health, readiness, and OpenAPI JSON paths are derived from the configured version prefix. Superseded default-prefix paths do not remain public. API-key comparison hashes both values before fixed-length constant-time comparison; an empty supplied key always fails. |
| Origins and rate limiting | CORS is off by default; an explicit allowlist reflects only an exact match and emits `Vary: Origin`; permissive `*` requires explicit enablement. Rate limiting trusts the socket peer rather than spoofable forwarded headers. TLS and trusted-proxy policy remain deployment-owned. |
| Request/body limits | The only server JSON request body is limited to 1 MiB, rejects unknown fields, and requires exactly one JSON object. Provider/source, remote manifest/payload, S3 object, and release-artifact reads remain bounded. SSE retains its 64 KiB line limit and now also has a 256 KiB cumulative frame limit; `Last-Event-ID` must be a positive integer before network I/O. |
| Decompression | Go HTTP automatic gzip decoding is bounded on the decoded reader. Catalog tar+gzip input is bounded in compressed form and through the decompressed tar stream, restricts members to three named regular files, and verifies every digest and descriptor binding before use. Incoming compressed search bodies are not implicitly decoded and fail strict JSON parsing. |
| Symlinks and paths | Human workspaces already reject symlink roots, writer locks, and unsafe machine overlap. Filesystem generation stores now reject symlinked roots, generation directories, current pointers, lock files, manifests, and payloads before read or mutation. Release staging rejects symlink lifecycle roots/generations and non-regular immutable assets. Digest-derived generation directories and fixed artifact member names prevent caller-controlled traversal. |
| Permissions | Catalog YAML, immutable catalog data, and public release assets contain no credentials and use the documented 0755 directory/0644 file policy. Sensitive test/operator files use 0600. Constructors do not create secret files. |
| Supply chain | Active actions are full-SHA pinned; Go/tool versions and the Chainguard base are exact; release binaries explicitly use `CGO_ENABLED=0`; checksums, GPG signatures, SBOMs, GitHub provenance, downloaded assets, and immutable release state are verified before/after publication. Release write/attestation/package/discussion permissions now exist only on the publishing job; test and Homebrew verification retain read-only defaults. |

## Remediated findings

### F-123 — configured-prefix authentication policy

The router registered health/readiness/OpenAPI paths under `PathPrefix`, but
authentication used a hard-coded `/api/v1` public list. A custom prefix could
make intended probes require a key while leaving nonexistent default-prefix
paths exempt. Middleware composition now derives the exact public paths from
the validated prefix. Real handler tests prove the four intended paths are
public, old-prefix paths are protected, and catalog routes still require a key.

### F-124 — bounded strict server and SSE inputs

Model search decoded an unbounded request body and accepted trailing JSON and
unknown fields. SSE limited each line but not the accumulated frame, permitting
unbounded memory growth from many legal-sized lines. Search now uses
`http.MaxBytesReader`, a 1 MiB implementation constant, strict field decoding,
and a required EOF after one value. SSE has a 256 KiB frame budget, retains its
64 KiB line budget, and validates resumption IDs before dialing.

### F-125 — provider credential redirect and error containment

The provider transport used the default redirect policy. Header credentials
could be replayed under net/http redirect rules, while query-authenticated
network errors could retain the secret-bearing URL in their unwrap graph.
Provider transport now returns the first 3xx response without following it.
Query-authenticated failures preserve caller cancellation but otherwise return
a typed, URL-free API error. Tests prove no redirect target is contacted and no
error or unwrap layer contains the credential.

### F-126 — authentication and panic-log hardening

Direct `subtle.ConstantTimeCompare` returned early for unequal-length keys, and
panic recovery logged the recovered value. Authentication now compares
fixed-length SHA-256 digests while retaining the explicit empty-input failure.
Recovery preserves type/method/path diagnostics without serializing an
arbitrary panic value; a secret-bearing panic regression proves absence.

### F-127 — machine-store and release symlink containment

The human workspace had explicit symlink defenses, but the filesystem
generation store and local release staging still followed selected lifecycle
entries during reads. Both boundaries now inspect owned roots and entries with
`Lstat`, reject symlink/non-regular substitutions with typed errors, and
preserve referenced operator files. Atomic generation CAS, immutable staging,
restart, and rollback behavior remain unchanged.

### F-128 — release workflow least privilege

Write-level contents/packages/attestation/discussion permissions were declared
at workflow scope, so the complete test job inherited publication authority.
The workflow default is now `contents: read`; only the publishing job receives
the five permissions required by GoReleaser, GHCR, provenance, immutable
release publication, and release discussions. Structural workflow tests pin
that separation.

## Verification

The final P10.6 candidate passed:

- `go test -race` across middleware (`1.855s`), handlers (`69.606s`), server
  (`170.923s`), transport (`3.231s`), catalogremote (`2.941s`), catalogstore
  (`3.799s`), catalogartifact (`2.893s`), workspace (`8.437s`), and workflow
  fixtures (`2.985s`);
- affected-package `go vet`, `actionlint .github/workflows/*.yaml`,
  `make goreleaser-check`, regenerated GoDoc, `make docs-check`, and
  `git diff --check`;
- `go mod verify` (`all modules verified`) and
  `go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...`;
- a production-file secret-pattern scan excluding tests and testdata, with no
  matches; and
- `go test ./... -count=1`, including the root (`83.541s`), server
  (`60.812s`), catalog (`30.119s`), remote (`19.692s`), and every other
  repository package.

`govulncheck` reports zero called or imported-package vulnerabilities. It also
reports GO-2026-5932 in the required `golang.org/x/crypto` module because the
deprecated `openpgp` package exists in that module; Starmap neither imports nor
calls that package, and the advisory has no fixed module version. This is
recorded rather than misrepresented as an entirely advisory-free module graph.

## Residual deployment obligations

- Enable API-key authentication and terminate trusted TLS before exposing the
  server outside a controlled network.
- Configure trusted-proxy handling outside Starmap; Starmap deliberately uses
  the socket peer for its local rate-limit identity.
- Treat provider YAML endpoints and injected S3 clients as privileged
  configuration, with network egress policy and secret-manager injection.
- Scan and verify the exact released image/binaries and attestations. A pinned
  base image is not a permanent “zero vulnerabilities” promise.
- Starport-owned relational adapters remain responsible for their driver,
  credentials, migrations, transaction/CAS correctness, backups, and database
  network policy.
