# Cursor Composer application channel

Status: P13.16 implementation evidence

## Current evidence

Primary Cursor sources establish that Composer is Cursor's own agentic coding
model and that Composer 2.5 is available inside Cursor with two current service
modes:

- [Composer 2.5 changelog](https://cursor.com/changelog/composer-2-5)
- [Original Composer architecture and product boundary](https://cursor.com/blog/composer)
- [Cursor pricing](https://cursor.com/pricing)
- [Cursor pricing policy](https://cursor.com/terms/pricing)

The May 18, 2026 announcement gives exact USD-per-million-token prices:
Standard is $0.50 input and $2.50 output; Fast (the default) is $3 input and
$15 output. It describes availability in Cursor, not a public server-to-server
Composer inference API.

## Catalog boundary

Starmap publishes one current `composer-2.5` definition and one Cursor
application offering. Standard and Fast are modes of that offering rather than
separate model definitions. The offering is `application_only` and
`discoverable_only`, has no invocation API, and cannot materialize as a
Starport route. The catalog intentionally does not invent a bearer credential,
base URL, request schema, or serverless inference deployment.

Cursor's documented routing of older Composer 2 selections to Composer 2.5 is
an operational behavior of the current Cursor application. It does not create
a second current model definition or a Starmap compatibility alias.
