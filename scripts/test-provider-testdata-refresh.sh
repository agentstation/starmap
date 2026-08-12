#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REFRESH="$ROOT/scripts/refresh-provider-testdata.sh"
WORKSPACE="$(mktemp -d)"
trap 'rm -rf "$WORKSPACE"' EXIT

fixture_root="$WORKSPACE/internal/providers/openai/testdata/providers"
mkdir -p "$fixture_root/alpha" "$fixture_root/beta"
for provider in alpha beta; do
	printf '{"data":[{"id":"%s"}]}\n' "$provider" >"$fixture_root/$provider/models_list.json"
	printf '{"provider":"%s"}\n' "$provider" >"$fixture_root/$provider/models_list.metadata.json"
done

fake_go="$WORKSPACE/fake-go"
cat >"$fake_go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >"$FAKE_GO_ARGS"
case "$FAKE_GO_MODE" in
	fail) exit 42 ;;
	noop) exit 0 ;;
	update)
		printf '\n' >>"internal/providers/openai/testdata/providers/$STARMAP_PROVIDER_FIXTURE/models_list.json"
		printf '\n' >>"internal/providers/openai/testdata/providers/$STARMAP_PROVIDER_FIXTURE/models_list.metadata.json"
		;;
	*) exit 97 ;;
esac
EOF
chmod +x "$fake_go"

expect_status() {
	expected="$1"
	shift
	set +e
	"$@" >/dev/null 2>&1
	actual=$?
	set -e
	if [[ "$actual" != "$expected" ]]; then
		printf 'command status = %s, want %s: %s\n' "$actual" "$expected" "$*" >&2
		exit 1
	fi
}

expect_status 0 env STARMAP_ROOT="$WORKSPACE" "$REFRESH" --help
expect_status 2 env STARMAP_ROOT="$WORKSPACE" "$REFRESH"
expect_status 2 env STARMAP_ROOT="$WORKSPACE" "$REFRESH" ../alpha
expect_status 3 env STARMAP_ROOT="$WORKSPACE" "$REFRESH" missing
expect_status 42 env STARMAP_ROOT="$WORKSPACE" STARMAP_GO_TEST_BIN="$fake_go" \
	FAKE_GO_MODE=fail FAKE_GO_ARGS="$WORKSPACE/args" "$REFRESH" alpha
expect_status 4 env STARMAP_ROOT="$WORKSPACE" STARMAP_GO_TEST_BIN="$fake_go" \
	FAKE_GO_MODE=noop FAKE_GO_ARGS="$WORKSPACE/args" "$REFRESH" alpha

beta_before="$(cksum "$fixture_root/beta/models_list.json" "$fixture_root/beta/models_list.metadata.json")"
expect_status 0 env STARMAP_ROOT="$WORKSPACE" STARMAP_GO_TEST_BIN="$fake_go" \
	FAKE_GO_MODE=update FAKE_GO_ARGS="$WORKSPACE/args" "$REFRESH" alpha
beta_after="$(cksum "$fixture_root/beta/models_list.json" "$fixture_root/beta/models_list.metadata.json")"
if [[ "$beta_before" != "$beta_after" ]]; then
	printf 'selective alpha refresh changed beta fixture bytes\n' >&2
	exit 1
fi
if ! grep -Fq "test ./internal/providers/openai -run ^TestRefreshOpenAICompatibleProviderFixture$ -count=1 -update" "$WORKSPACE/args"; then
	printf 'refresh did not invoke the exact OpenAI-compatible fixture test\n' >&2
	exit 1
fi

printf 'provider testdata refresh contract passed\n'
