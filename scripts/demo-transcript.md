# Catalog server preflight transcript

This transcript contains the preflight commands and output.
The transcript omits terminal color and screen controls.
The recording adds chapter titles, typing, prompt waits, and pauses.
Titles precede each action and remain above the commands.
The runner waits for server readiness before the client reads a model.
The ending identifies the observed model and verified generation.

The catalog combines retained provider observations with a reviewed GPT-6.1 Sol definition.
The recording evidence binds the exact working-tree catalog inputs and generation metadata.

The checksum check verifies payload integrity.
Consumer configuration, authentication, authority checks, and subscriber activation require separate setup.
See [Go consumer configuration](../remote/README.md#Config) or the [Starport central-server guide](https://github.com/agentstation/starport/blob/main/docs/site/operate-starmap/central-server.md#connect-replicas-to-a-central-server).

```text
$ source setup.sh
$ printf '\033[2J\033[H\033[1;36mStarmap / Serve a model catalog\033[0m\nGoal: read model metadata and verify a catalog generation over HTTP.\nLocal catalog data. No provider API keys or inference requests.\n\n\033[1;36m1 / Start the catalog server\033[0m\n\n'
Starmap / Serve a model catalog
Goal: read model metadata and verify a catalog generation over HTTP.
Local catalog data. No provider API keys or inference requests.

1 / Start the catalog server

$ starmap serve --host 127.0.0.1 --port 18991 >server.log 2>&1 &
$ url=http://127.0.0.1:18991/api/v1
$ curl -fsS --retry 20 --retry-connrefused --retry-max-time 10 --max-time 2 "$url/ready" >/dev/null 2>&1
$ printf '\033[2A\033[J'
$ curl -fsS --max-time 5 "$url/ready" | tee ready.json | jq '.data | {status}'
{
  "status": "ready"
}
$ printf '\033[2J\033[H\033[1;36m2 / Read a model over HTTP\033[0m\nA client asks for the canonical model openai/gpt-6.1-sol.\n\n'
2 / Read a model over HTTP
A client asks for the canonical model openai/gpt-6.1-sol.

$ curl -fsS --max-time 5 "$url/models/openai/gpt-6.1-sol" >model.json
$ jq '.data | {id, name, author_ids}' model.json
{
  "id": "openai/gpt-6.1-sol",
  "name": "GPT-6.1 Sol",
  "author_ids": [
    "openai"
  ]
}
$ printf '\033[2J\033[H\033[1;36m3 / Fetch and verify a generation\033[0m\nFetch the manifest, then verify its immutable catalog payload.\n\n'
3 / Fetch and verify a generation
Fetch the manifest, then verify its immutable catalog payload.

$ curl -fsS --max-time 5 "$url/catalog/manifest" >manifest.json
$ jq '{generation_id, schema_version, validation: .validation.status}' manifest.json
{
  "generation_id": "bindings-53fdce277e501591d72a1160857d31787d386754e0775b862a2a22704eb60a36.local.c70aef7a19dd",
  "schema_version": 19,
  "validation": "passed"
}
$ id=$(jq -r .generation_id manifest.json)
$ curl -fsS --max-time 5 -D payload.headers "$url/catalog/generations/$id/payload" >catalog.json
$ jq -r '.payload.checksum | sub("^sha256:"; "") + "  catalog.json"' manifest.json | shasum -a 256 -c
catalog.json: OK
$ jq -n --arg model "$(jq -r .data.id model.json)" --arg generation "$id" '{model_id:$model, generation_id:$generation, checksum:"verified"}' >outcome.json
$ printf '\033[2J\033[H\033[1;36mResult / Model read and payload verified\033[0m\n\nModel: %s\nGeneration: %s\nPayload checksum: OK\n\nNext: configure a catalog subscriber.\nGo: remote/README.md   Starport: central-server guide\n' "$(jq -r .model_id outcome.json)" "$(jq -r '.generation_id | .[:24] + "..." + .[-12:]' outcome.json)"
Result / Model read and payload verified

Model: openai/gpt-6.1-sol
Generation: bindings-53fdce277e50159...c70aef7a19dd
Payload checksum: OK

Next: configure a catalog subscriber.
Go: remote/README.md   Starport: central-server guide
$ touch complete
```
