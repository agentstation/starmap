# Record with scripts/record-demo.py. It supplies an isolated embedded catalog.
Output scripts/demo.svg
Output scripts/demo.gif

Set Shell "bash"
Set Width 1280
Set Height 720
Set FontFamily "Geist Mono"
Set FontSize 16
Set Theme "Dracula"
Set Padding 20
Set WindowBar Colorful
Set WindowBarTitle "Starmap · catalog server"
Set Framerate 20
Set TypingSpeed 35ms


Hide
Set TypingSpeed 0ms
Type `source setup.sh`
Enter
Wait+Line@10s /^[$]$/
# Opening: Serve a model catalog for applications and gateways
# Chapter: Start the catalog server

Hide
Set TypingSpeed 0ms
Type `printf '\033[2J\033[H\033[1;36mStarmap / Serve a model catalog\033[0m\nGoal: read model metadata and verify a catalog generation over HTTP.\nLocal catalog data. No provider API keys or inference requests.\n\n\033[1;36m1 / Start the catalog server\033[0m\n\n'`
Enter
Wait+Line@10s /^[$]$/
Show
Set TypingSpeed 35ms
Sleep 2.5s
Type `starmap serve --host 127.0.0.1 --port 18991 >server.log 2>&1 &`
Enter
Wait+Line@10s /^[$]$/
Type `url=http://127.0.0.1:18991/api/v1`
Enter
Wait+Line@10s /^[$]$/
Hide
Set TypingSpeed 0ms
# Readiness retries finish before the next visible command.
Type `curl -fsS --retry 20 --retry-connrefused --retry-max-time 10 --max-time 2 "$url/ready" >/dev/null 2>&1`
Enter
Wait+Line@15s /^[$]$/
Type `printf '\033[2A\033[J'`
Enter
Wait+Line@10s /^[$]$/
Show
Set TypingSpeed 35ms
Type `curl -fsS --max-time 5 "$url/ready" | tee ready.json | jq '.data | {status}'`
Enter
Wait+Line@10s /^[$]$/
Sleep 2s
# Chapter: Read a model over HTTP

Hide
Set TypingSpeed 0ms
Type `printf '\033[2J\033[H\033[1;36m2 / Read a model over HTTP\033[0m\nA client asks for the canonical model openai/gpt-6.1-sol.\n\n'`
Enter
Wait+Line@10s /^[$]$/
Show
Set TypingSpeed 35ms
Sleep 1.8s
Type `curl -fsS --max-time 5 "$url/models/openai/gpt-6.1-sol" >model.json`
Enter
Wait+Line@10s /^[$]$/
Type `jq '.data | {id, name, author_ids}' model.json`
Enter
Wait+Line@10s /^[$]$/
Sleep 3s
# Chapter: Fetch and verify a generation

Hide
Set TypingSpeed 0ms
Type `printf '\033[2J\033[H\033[1;36m3 / Fetch and verify a generation\033[0m\nFetch the manifest, then verify its immutable catalog payload.\n\n'`
Enter
Wait+Line@10s /^[$]$/
Show
Set TypingSpeed 35ms
Sleep 1.8s
Type `curl -fsS --max-time 5 "$url/catalog/manifest" >manifest.json`
Enter
Wait+Line@10s /^[$]$/
Type `jq '{generation_id, schema_version, validation: .validation.status}' manifest.json`
Enter
Wait+Line@10s /^[$]$/
Sleep 2s
Type `id=$(jq -r .generation_id manifest.json)`
Enter
Wait+Line@10s /^[$]$/
Type `curl -fsS --max-time 5 -D payload.headers "$url/catalog/generations/$id/payload" >catalog.json`
Enter
Wait+Line@10s /^[$]$/
Type `jq -r '.payload.checksum | sub("^sha256:"; "") + "  catalog.json"' manifest.json | shasum -a 256 -c`
Enter
Wait+Line@10s /^[$]$/
Sleep 3s

# Ending: Model read and generation payload verified
Hide
Set TypingSpeed 0ms
Type `jq -n --arg model "$(jq -r .data.id model.json)" --arg generation "$id" '{model_id:$model, generation_id:$generation, checksum:"verified"}' >outcome.json`
Enter
Wait+Line@10s /^[$]$/
Type `printf '\033[2J\033[H\033[1;36mResult / Model read and payload verified\033[0m\n\nModel: %s\nGeneration: %s\nPayload checksum: OK\n\nNext: configure a catalog subscriber.\nGo: remote/README.md   Starport: central-server guide\n' "$(jq -r .model_id outcome.json)" "$(jq -r '.generation_id | .[:24] + "..." + .[-12:]' outcome.json)"`
Enter
Wait+Line@10s /^[$]$/
Show
Sleep 5s
Hide
Type `touch complete`
Enter
Wait+Line@10s /^[$]$/
