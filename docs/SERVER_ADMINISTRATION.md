# Standalone server administration

Starmap keeps catalog subscriber credentials separate from administrator credentials.
A subscriber can read its configured catalog audience. It cannot refresh the catalog,
change identities, or read administrator reports. Provider acquisition credentials and
Starport inference credentials do not grant administrator access.

A default server without administration state serves the public catalog with its
configured reader authentication. Its administrator routes reject requests.
An internal catalog requires initialized administration state before the CLI starts
its listener. Initialization creates no default network credential.

## Initialize one server

Select the product roots and catalog authority configuration first. Run:

```sh
starmap admin init --id operator
```

The command writes one administrator credential to standard output as JSON.
Store that credential privately. The state file stores its digest, not the credential.
The command refuses existing identities and a live administration writer.

Set `STARMAP_ADMIN_TOKEN` for explicit local administration commands. This variable
does not enable network administrator access by itself.

```sh
starmap admin create --id gateway --role subscriber
starmap serve
```

Configure Starport's catalog transport with the subscriber credential. Keep its
provider inference credentials separate. The server validates each incoming
credential against its configured audience and active identity state.

The authority origin ID selects the audience when the server enables origin issuance.
A configured internal source authority ID selects it for a relay.
Otherwise, the deployment ID selects it. Changing the audience requires an explicit
state migration. Existing credentials cannot cross that boundary.

Embedding hosts
must stop and reconstruct the server when they change the catalog authority.
The Go server constructor also requires managed identities for internal catalogs.

## Operate a running server

Use an administrator bearer credential for these routes:

| Method and route | Effect |
| --- | --- |
| `POST /admin/identities` | Create an identity from `id` and `role`. Returns the credential once. |
| `POST /admin/identities/{id}/rotate` | Rotate a credential with an explicit duration such as `{"overlap":"10m"}`. |
| `DELETE /admin/identities/{id}` | Revoke the identity. The last active administrator cannot be revoked. |
| `GET /admin/status` | Read administration readiness, revision, and retained operation count. |
| `GET /admin/config/schema` | Read the canonical catalog setting descriptors. |
| `GET /admin/config/effective` | Read redacted catalog values, origins, presence, and managed paths. |
| `POST /api/v1/update` | Start an audited catalog acquisition operation. |
| `GET /api/v1/updates/{id}` | Read operation state, including retained recovery receipts. |
| `DELETE /api/v1/updates/{id}` | Request audited cancellation. Effects can already be partial. |

Rotate a credential with overlap to let subscribers change credentials while the
server runs. Overlap cannot exceed 24 hours. A second rotation cannot discard an
unexpired overlap. Revocation withdraws all keys for that identity. Active catalog
event streams end when their credential expires, an administrator revokes it, or the server closes.

Local `admin create`, `admin rotate`, and `admin revoke` commands require exclusive
state access. Stop the server before using them. Use the HTTP routes for online
changes. After losing an administrator credential, stop the server and run:

```sh
starmap admin recover --id operator
```

Recovery requires private filesystem ownership and valid audit history. It records
the replacement and immediately withdraws the lost credential. It cannot bypass
corrupt audit state. An interrupted first initialization can use the same recovery
command. Recovery retains its interrupted receipt and requires the original
administrator ID. A missing identity file after successful initialization requires
a consistent backup restore.

## Inspect configuration before catalog readiness

```sh
starmap config schema --format json
starmap config effective --format json
starmap config paths --output json
```

These commands do not open a catalog runtime or fetch sources. The CLI and
administrator API use the same configuration report content. Secret settings retain
their presence and origin but omit their values. API reports include an ETag and
prohibit shared caching. Subscriber credentials cannot read them.

Explicit `--host` and `--port` flags override listener environment variables.
Use `STARMAP_SERVER_HOST` and `STARMAP_SERVER_PORT`. Legacy `HTTP_HOST` and `HTTP_PORT`
produce migration diagnostics. Conflicting product and legacy variables fail unless
an explicit flag selects the value. Invalid selected ports fail before startup.

## State, audit, and recovery

The canonical state root owns these files:

| Location under the state root | Purpose |
| --- | --- |
| `admin/identities.json` | Audience, identity revisions, revocations, and credential digests |
| `admin/.owner.lock` | Exclusive administration writer ownership |
| `admin/audit/events.ndjson` | Ordered administrative intent and outcome events |
| `admin/operations/<operation-id-hash>.json` | Durable operation receipts |
| `admin/**/.record-publications/` | Private publication journals and ownership records |

The same private-file rules apply on Linux, macOS, and Windows. The server uses no
SQL database for this state. Authentication reads an immutable memory snapshot.
Administrative writes verify private paths and writer ownership.

An operation records durable intent before effects. It records its durable outcome
before reporting success. Audit-write failure rejects new mutations and preserves
authenticated diagnostics. A published revocation remains enforced if its audit
outcome fails. Restart can prove an identity outcome from its recorded content
digests. An unproved acquisition outcome becomes `interrupted` and never replays
automatically.

The update-status route preserves its operation response format after restart.
An interrupted receipt reports terminal `failed` status. The
`detail.retained_receipt` field preserves the durable outcome and its recorded times.
Live execution details, including cancellation classification, remain process-local.

Keep identities, audit history, receipts, and accepted catalog authority together
in a consistent offline backup. Do not copy a live administration directory between
writers or share it between processes.

The status reports audit bytes, operation count, and both limits. The limits are
64 MiB of audit history and 16,384 operations. Reaching the capacity rejects new mutations and preserves diagnostics.
No automatic eviction removes audit evidence. Retention maintenance must preserve
the deployment's required history and recovery evidence.
