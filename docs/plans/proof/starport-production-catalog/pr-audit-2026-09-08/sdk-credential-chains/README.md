# SDK credential-chain compatibility

The new tests call the production AWS and Azure secret-source factories.
They use real SDK credential selection, token exchange, request signing, and response decoding.
Local TLS servers provide the remote service replies.
The fixtures replace neither factory nor SDK read function.

| Backend | Tested flow | Required observation |
| --- | --- | --- |
| AWS | Environment credentials | The secret request selects the environment access key and session token. |
| AWS | Shared credentials file | The secret request selects the file's access key. |
| AWS | Environment precedence | Environment credentials take precedence over the default shared-file credentials. |
| AWS | Web identity | STS receives the configured assertion and role, then Secrets Manager receives the returned credentials. |
| AWS | Missing credentials | Resolution returns a typed unavailable error without a service request or secret value. |
| AWS | Denied secret read | Resolution returns a typed denied error without a secret value. |
| Azure | Environment credentials | The token endpoint receives the configured client secret and vault scope. |
| Azure | Workload identity | The token endpoint receives the file assertion without a client secret. |
| Azure | Denied secret read | The authenticated vault reply produces a typed denied error without a secret value. |

Every authenticated Azure vault read uses the returned bearer token and selected secret version.
The AWS requests use SigV4 with the expected region and service scope.
The full authentication suite passes 78 test events and one package result on each checked dependency and toolchain combination.
Every run reports zero failures and skips.

## Checked combinations

The canonical catalog branch retains its existing SDK dependencies at base `5ccae0a4`.
The isolated branch contains the four proposed dependency updates and consumer checksum repair at base `9575418e`.
Their production factory files are identical.
Both combinations pass the full authentication race suite with Go 1.26.6.

The updated dependencies also pass that suite with Go 1.25.12.

Package lint reports zero issues for the updated dependencies.
Ago reports zero findings, stale ignores, and errors in both worktrees.
The [verification record](verification.json) binds the final tests, factories, module files, and raw results to their exact bytes.

## Fixture correction

The first Azure fixture expected only the vault scope.
Both MSAL versions append the standard `openid`, `offline_access`, and `profile` scopes.
The corrected fixture checks that exact scope set and preserves the vault audience.
The initial failures describe an incorrect fixture expectation, not a production defect.
The initial Azure source and diagnostic captures remain available.
The initial AWS source snapshot was not retained, so that draft capture provides no qualification evidence.

## Isolation and limits

The tests clear inherited AWS and Azure settings and use temporary fixture credentials and files.
Their serial HTTP transport permits only the named fixture hosts and sends every allowed connection to a local TLS server.
Unexpected hosts or service routes fail the test.
The fixture trusts only its test certificate and keeps TLS certificate verification enabled.
No real cloud credential, secret, or provider call occurs.

These checks cover the listed credential paths only.
They do not qualify live cloud IAM policy, SSO, device login, EC2 or ECS identity, or Azure managed identity services.
They do not prove production secret-manager availability or the released Starmap and Starport pair.
Default-branch merges and releases still require owner authorization.
