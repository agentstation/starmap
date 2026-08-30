package anthropic_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/internal/providers/anthropic"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/internal/test/providerfixture"
	"github.com/agentstation/starmap/pkg/catalogs"
)

const anthropicFixtureRoot = "testdata/providers"

// TestRefreshAnthropicProviderFixture captures the live Anthropic catalog
// response. Anthropic serves a custom protocol, so it needs its own capture
// path: the OpenAI-compatible refresh cannot decode it.
// scripts/refresh-provider-testdata.sh anthropic owns this test.
func TestRefreshAnthropicProviderFixture(t *testing.T) {
	if !providerfixture.UpdateRequested() {
		t.Skip("fixture refresh requires the explicit -update flag")
	}
	fixture, provider := anthropicFixture(t)
	payload := fetchLiveAnthropicCatalog(t, provider)

	// A capture must satisfy the parser it exists to prove, so replay the exact
	// bytes through the real client before they reach the fixture.
	assertAnthropicPayloadParses(t, provider, payload)

	if err := fixture.Capture(payload, time.Now().UTC()); err != nil {
		t.Fatalf("capture provider fixture: %v", err)
	}
}

func TestAnthropicProviderFixtureCurrency(t *testing.T) {
	if !providerfixture.CurrencyRequested() {
		t.Skipf("set %s=1 to compare fixtures against live provider responses",
			providerfixture.CurrencyVariable)
	}
	fixture, provider := anthropicFixture(t)

	now := time.Now().UTC()
	age, maxAge, err := fixture.Freshness(now)
	if err != nil {
		t.Fatalf("read fixture freshness: %v", err)
	}
	t.Logf("fixture age %s of reviewed maximum %s", age.Round(time.Hour), maxAge)
	if err := fixture.VerifyFreshness(now); err != nil {
		t.Errorf("fixture needs a live refresh: %v", err)
	}

	recorded, err := fixture.Read()
	if err != nil {
		t.Fatalf("read fixture payload: %v", err)
	}
	live := fetchLiveAnthropicCatalog(t, provider)
	absent, added, err := providerfixture.WireDrift(recorded, live)
	if err != nil {
		t.Fatalf("compare fixture against live response: %v", err)
	}
	if len(absent) > 0 {
		t.Errorf("fixture exercises provider fields the live response no longer returns: %v", absent)
	}
	if len(added) > 0 {
		t.Errorf("live response returns provider fields the fixture does not record: %v", added)
	}
}

func TestAnthropicFixtureCaptureRoundTrip(t *testing.T) {
	fixture, provider := anthropicFixture(t)
	recorded, err := fixture.Read()
	if err != nil {
		t.Fatalf("read fixture payload: %v", err)
	}
	assertAnthropicPayloadParses(t, provider, recorded)

	scratch := copyFixture(t, fixture)
	capturedAt := time.Now().UTC()
	if err := scratch.Capture(recorded, capturedAt); err != nil {
		t.Fatalf("capture provider fixture: %v", err)
	}
	if err := scratch.VerifyFreshness(capturedAt); err != nil {
		t.Fatalf("captured fixture is not fresh: %v", err)
	}
	captured, err := scratch.Read()
	if err != nil {
		t.Fatalf("read captured payload: %v", err)
	}
	// Capture canonicalizes JSON, and a capture taken before that rule can differ
	// in formatting, so compare decoded values rather than bytes.
	assertSameJSON(t, recorded, captured)

	// Canonical bytes must survive a second capture unchanged, otherwise every
	// refresh would report drift that the provider did not cause.
	if err := scratch.Capture(captured, capturedAt); err != nil {
		t.Fatalf("recapture canonical payload: %v", err)
	}
	recaptured, err := scratch.Read()
	if err != nil {
		t.Fatalf("read recaptured payload: %v", err)
	}
	if !bytes.Equal(recaptured, captured) {
		t.Fatal("capture is not idempotent on canonical bytes")
	}
}

func assertSameJSON(t *testing.T, want, have []byte) {
	t.Helper()
	var wantValue, haveValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("decode recorded payload: %v", err)
	}
	if err := json.Unmarshal(have, &haveValue); err != nil {
		t.Fatalf("decode captured payload: %v", err)
	}
	if !reflect.DeepEqual(wantValue, haveValue) {
		t.Fatal("capture changed the payload contents")
	}
}

// copyFixture returns the fixture rewritten into a scratch directory, so a
// capture test never rewrites the reviewed fixture.
func copyFixture(t *testing.T, fixture providerfixture.Fixture) providerfixture.Fixture {
	t.Helper()
	directory := filepath.Join(t.TempDir(), fixture.Provider)
	if err := os.MkdirAll(directory, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	scratch := providerfixture.Fixture{
		Provider:     fixture.Provider,
		PayloadPath:  filepath.Join(directory, filepath.Base(fixture.PayloadPath)),
		MetadataPath: filepath.Join(directory, filepath.Base(fixture.MetadataPath)),
	}
	for source, destination := range map[string]string{
		fixture.PayloadPath:  scratch.PayloadPath,
		fixture.MetadataPath: scratch.MetadataPath,
	} {
		data, err := os.ReadFile(source) //nolint:gosec // Fixture paths are repository-controlled.
		if err != nil {
			t.Fatalf("ReadFile %s: %v", source, err)
		}
		if err := os.WriteFile(destination, data, constants.SecureFilePermissions); err != nil {
			t.Fatalf("WriteFile %s: %v", destination, err)
		}
	}
	return scratch
}

// anthropicFixture returns the governed fixture and the embedded provider record
// that governs its acquisition.
func anthropicFixture(t *testing.T) (providerfixture.Fixture, *catalogs.Provider) {
	t.Helper()
	fixture, err := providerfixture.Find(anthropicFixtureRoot, string(catalogs.ProviderIDAnthropic))
	if err != nil {
		t.Fatalf("select fixture: %v", err)
	}
	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}
	provider, found := builder.Providers().Get(catalogs.ProviderIDAnthropic)
	if !found {
		t.Fatal("embedded provider \"anthropic\" is missing")
	}
	assertAnthropicCatalogContract(t, provider)
	return fixture, provider
}

func fetchLiveAnthropicCatalog(t *testing.T, provider *catalogs.Provider) []byte {
	t.Helper()
	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}
	fetcher := acquisition.NewProviderFetcher(builder.Providers())
	payload, _, err := fetcher.FetchRawResponse(t.Context(), provider, provider.CatalogEndpointURL())
	if err != nil {
		t.Fatalf("fetch raw provider catalog: %v", err)
	}
	return payload
}

// assertAnthropicPayloadParses replays payload through the real Anthropic client,
// so an accepted capture always satisfies the strict list-response contract.
func assertAnthropicPayloadParses(t *testing.T, provider *catalogs.Provider, payload []byte) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	// Providers().Get returns a caller-owned deep copy, so redirecting the
	// endpoint cannot reach the embedded catalog.
	replay := provider
	replay.Catalog.Endpoint.URL = server.URL

	models, err := anthropic.NewClient(replay).ListModels(
		t.Context(), testcatalog.APIKeyMaterial(replay.Credentials, "test-api-key"),
	)
	if err != nil {
		t.Fatalf("parse fetched provider response: %v", err)
	}
	if len(models) == 0 {
		t.Fatal("fetched provider response contains no models")
	}
}

func assertAnthropicCatalogContract(t *testing.T, provider *catalogs.Provider) {
	t.Helper()
	if provider == nil || provider.Catalog == nil {
		t.Fatal("provider has no catalog acquisition contract")
	}
	if provider.Catalog.Endpoint.Type != catalogs.EndpointTypeAnthropic {
		t.Fatalf("endpoint type = %q, want %q",
			provider.Catalog.Endpoint.Type, catalogs.EndpointTypeAnthropic)
	}
	if provider.Catalog.Endpoint.URL == "" ||
		provider.CatalogEndpointURL() != provider.Catalog.Endpoint.URL {
		t.Fatalf("catalog endpoint = %q", provider.Catalog.Endpoint.URL)
	}
	if provider.Catalog.Endpoint.ProtocolOptions.Anthropic == nil {
		t.Fatal("Anthropic protocol options are missing")
	}
	if provider.Credentials == nil ||
		len(provider.Credentials.CatalogAcquisition.Alternatives) == 0 {
		t.Fatal("catalog credential metadata is missing")
	}
	if err := provider.ValidateContract(); err != nil {
		t.Fatalf("provider contract: %v", err)
	}
}
