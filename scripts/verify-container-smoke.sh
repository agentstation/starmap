#!/usr/bin/env bash
# Container smoke check for the shipped deployment example.
#
# The check builds the server image the way the release builds it. It uses the
# same digest-pinned static base. It then runs the image with the Compose
# security settings. Those settings are a read-only root filesystem, an
# unprivileged user, no capability, and one writable state volume. The check
# finally reads the health endpoint.
#
# The check reports UNVERIFIED and exits zero when Docker is unavailable, so an
# offline workstation reports the gap instead of a false failure.
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 1

# BASE is the digest-pinned base image of the release configuration.
BASE="cgr.dev/chainguard/static@sha256:60582b2ae6074f641094af0f370d4ab241aab271858a66223dcde7eee9f51638"
IMAGE="starmap-container-smoke:local"
CONTAINER="starmap-container-smoke"
VOLUME="starmap-container-smoke-state"
STATE_PATH="/home/nonroot"
PORT="18080"

if ! command -v docker >/dev/null 2>&1 || ! docker info >/dev/null 2>&1; then
	printf 'UNVERIFIED the container smoke check needs Docker.\n'
	printf 'Run this exact command on a host with Docker: bash scripts/verify-container-smoke.sh\n'
	exit 0
fi

BUILD="$(mktemp -d "${TMPDIR:-/tmp}/starmap-container-smoke.XXXXXX")"
cleanup() {
	docker rm -f "$CONTAINER" >/dev/null 2>&1
	docker volume rm -f "$VOLUME" >/dev/null 2>&1
	docker image rm -f "$IMAGE" >/dev/null 2>&1
	rm -rf "$BUILD"
}
trap cleanup EXIT

printf 'Building the server binary for linux/amd64.\n'
if ! GOTOOLCHAIN=go1.26.6 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -trimpath -ldflags '-s -w' -o "$BUILD/starmap" ./cmd/starmap; then
	printf 'FAIL the server binary did not build.\n'
	exit 1
fi

cat >"$BUILD/Dockerfile" <<EOF
FROM $BASE
COPY starmap /ko-app/starmap
ENTRYPOINT ["/ko-app/starmap"]
EOF

printf 'Building the smoke image on the release base.\n'
if ! docker build --quiet --tag "$IMAGE" "$BUILD" >/dev/null; then
	printf 'FAIL the smoke image did not build.\n'
	exit 1
fi

printf 'Starting the server with a read-only root filesystem.\n'
if ! docker run --detach --name "$CONTAINER" \
	--read-only \
	--tmpfs /tmp \
	--cap-drop ALL \
	--security-opt no-new-privileges:true \
	--user 65532:65532 \
	--volume "$VOLUME:$STATE_PATH" \
	--publish "127.0.0.1:$PORT:8080" \
	--env "STARMAP_CATALOG_WORKSPACE_PATH=$STATE_PATH/.starmap/catalog" \
	--env "STARMAP_STATE_DIR=$STATE_PATH/.starmap/state/runtime" \
	--env "STARMAP_CATALOG_SOURCE=embedded" \
	--env "STARMAP_CATALOG_ACQUISITION_ENABLED=false" \
	"$IMAGE" serve --host 0.0.0.0 --port 8080 >/dev/null; then
	printf 'FAIL the container did not start.\n'
	exit 1
fi

printf 'Reading the health endpoint.\n'
healthy=1
for _ in $(seq 1 30); do
	if curl --silent --fail --max-time 2 "http://127.0.0.1:$PORT/health" >/dev/null; then
		healthy=0
		break
	fi
	sleep 1
done

if [ "$healthy" -ne 0 ]; then
	printf 'FAIL the server did not answer /health with a read-only root.\n'
	docker logs "$CONTAINER" 2>&1 | sed 's/^/  /'
	exit 1
fi

printf 'PASS the image serves with a read-only root and a writable state volume.\n'
