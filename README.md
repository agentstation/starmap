# Catalog channel v1

This branch is a machine-written discovery pointer. The catalog
generation workflow is its only writer.

The file `channel.json` carries the media type
`application/vnd.agentstation.starmap.catalog-channel.v1+json`. It
names the one immutable `catalog-<digest>` release that a client
reads, and it carries build provenance that the client verifies
before it decodes the document.

Do not edit this branch by hand. A hand-written commit breaks the
sequence that every client uses to reject a replayed pointer.
