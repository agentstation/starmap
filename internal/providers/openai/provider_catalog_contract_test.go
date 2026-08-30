package openai_test

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/internal/providers/openai"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/internal/test/providerfixture"
	"github.com/agentstation/starmap/pkg/catalogs"
)

const providerFixtureRoot = "testdata/providers"

func TestOpenAICompatibleProviderCatalogContracts(t *testing.T) {
	fixtures, err := providerfixture.Discover(providerFixtureRoot)
	if err != nil {
		t.Fatalf("discover provider fixtures: %v", err)
	}
	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.Provider, func(t *testing.T) {
			t.Parallel()
			if err := fixture.Verify(time.Now().UTC()); err != nil {
				t.Fatalf("verify fixture: %v", err)
			}
			provider, found := builder.Providers().Get(catalogs.ProviderID(fixture.Provider))
			if !found {
				t.Fatalf("embedded provider %q is missing", fixture.Provider)
			}
			assertProviderCatalogContract(t, provider)

			var response openai.Response
			if err := fixture.Decode(&response); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			if response.Data == nil {
				t.Fatal("fixture response has a missing or null data array")
			}
			client, err := openai.NewClient(provider)
			if err != nil {
				t.Fatalf("create catalog-driven client: %v", err)
			}
			assertConfiguredAuthorMappings(t, client, provider.Catalog.Endpoint.AuthorMapping)
			mapped := make(map[string]bool, len(provider.Catalog.Endpoint.FieldMappings))
			for _, source := range response.Data {
				converted, err := client.ConvertToModel(source)
				if err != nil {
					t.Fatalf("convert exact model %q: %v", source.ID, err)
				}
				if converted.ID != source.ID {
					t.Fatalf("converted model ID = %q, want exact provider ID %q", converted.ID, source.ID)
				}
				assertAuthorMapping(t, provider.Catalog.Endpoint.AuthorMapping, source, converted)
				for _, mapping := range provider.Catalog.Endpoint.FieldMappings {
					if assertFieldMapping(t, mapping, source, converted) {
						mapped[mapping.From+"->"+mapping.To] = true
					}
				}
			}
			for _, mapping := range provider.Catalog.Endpoint.FieldMappings {
				key := mapping.From + "->" + mapping.To
				if !mapped[key] {
					t.Errorf("fixture does not exercise catalog field mapping %s", key)
				}
			}
		})
	}
}

func assertConfiguredAuthorMappings(
	t *testing.T,
	client *openai.Client,
	mapping *catalogs.AuthorMapping,
) {
	t.Helper()
	if mapping == nil {
		return
	}
	for source, want := range mapping.Normalized {
		value := mappingExample(source)
		model := openai.Model{ID: "opaque-model-id", OwnedBy: "unmapped-owner"}
		switch mapping.Field {
		case "id":
			model.ID = value
		case "owned_by":
			model.OwnedBy = value
		default:
			t.Fatalf("unsupported configured author field %q", mapping.Field)
		}
		converted, err := client.ConvertToModel(model)
		if err != nil {
			t.Fatalf("convert author mapping source %q: %v", source, err)
		}
		if len(converted.Authors) != 1 || converted.Authors[0].ID != want {
			t.Errorf("author mapping %q produced %#v, want %q", source, converted.Authors, want)
		}
	}
}

func mappingExample(pattern string) string {
	var example []rune
	for _, character := range pattern {
		switch character {
		case '*':
			example = append(example, []rune("fixture")...)
		case '?':
			example = append(example, 'x')
		default:
			example = append(example, character)
		}
	}
	return string(example)
}

func TestRefreshOpenAICompatibleProviderFixture(t *testing.T) {
	if !providerfixture.UpdateRequested() {
		t.Skip("fixture refresh requires the explicit -update flag")
	}
	providerID := os.Getenv("STARMAP_PROVIDER_FIXTURE")
	if providerID == "" {
		t.Fatal("set STARMAP_PROVIDER_FIXTURE to one discovered provider ID")
	}
	fixture, err := providerfixture.Find(providerFixtureRoot, providerID)
	if err != nil {
		t.Fatalf("select fixture: %v", err)
	}
	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}
	provider, found := builder.Providers().Get(catalogs.ProviderID(fixture.Provider))
	if !found {
		t.Fatalf("embedded provider %q is missing", fixture.Provider)
	}
	assertProviderCatalogContract(t, provider)

	fetcher := acquisition.NewProviderFetcher(builder.Providers())
	payload, _, err := fetcher.FetchRawResponse(t.Context(), provider, provider.CatalogEndpointURL())
	if err != nil {
		t.Fatalf("fetch raw provider catalog: %v", err)
	}
	var response openai.Response
	if err := response.UnmarshalJSON(payload); err != nil {
		t.Fatalf("validate fetched provider response: %v", err)
	}
	if response.Data == nil {
		t.Fatal("fetched provider response has a missing or null data array")
	}
	capturedAt := time.Now().UTC()
	if err := fixture.Capture(payload, capturedAt); err != nil {
		t.Fatalf("capture provider fixture: %v", err)
	}
}

func TestOpenAICompatibleProviderFixtureCurrency(t *testing.T) {
	if !providerfixture.CurrencyRequested() {
		t.Skipf("set %s=1 to compare fixtures against live provider responses",
			providerfixture.CurrencyVariable)
	}
	fixtures, err := providerfixture.Discover(providerFixtureRoot)
	if err != nil {
		t.Fatalf("discover provider fixtures: %v", err)
	}
	builder, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.Provider, func(t *testing.T) {
			t.Parallel()
			now := time.Now().UTC()
			age, maxAge, err := fixture.Freshness(now)
			if err != nil {
				t.Fatalf("read fixture freshness: %v", err)
			}
			t.Logf("fixture age %s of reviewed maximum %s", age.Round(time.Hour), maxAge)
			if err := fixture.VerifyFreshness(now); err != nil {
				t.Errorf("fixture needs a live refresh: %v", err)
			}
			provider, found := builder.Providers().Get(catalogs.ProviderID(fixture.Provider))
			if !found {
				t.Fatalf("embedded provider %q is missing", fixture.Provider)
			}
			recorded, err := fixture.Read()
			if err != nil {
				t.Fatalf("read fixture payload: %v", err)
			}
			fetcher := acquisition.NewProviderFetcher(builder.Providers())
			live, _, err := fetcher.FetchRawResponse(t.Context(), provider, provider.CatalogEndpointURL())
			if err != nil {
				t.Fatalf("fetch live provider catalog: %v", err)
			}
			assertNoWireDrift(t, recorded, live)
		})
	}
}

// assertNoWireDrift reports provider fields that the fixture and the live
// response do not share. Either direction means the fixture no longer mirrors
// the provider, so the mapping contract it proves is no longer current.
func assertNoWireDrift(t *testing.T, recorded, live []byte) {
	t.Helper()
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

func assertProviderCatalogContract(t *testing.T, provider *catalogs.Provider) {
	t.Helper()
	if provider == nil || provider.Catalog == nil {
		t.Fatal("provider has no catalog acquisition contract")
	}
	if provider.Catalog.Endpoint.Type != catalogs.EndpointTypeOpenAI {
		t.Fatalf("endpoint type = %q, want %q", provider.Catalog.Endpoint.Type, catalogs.EndpointTypeOpenAI)
	}
	if provider.Catalog.Endpoint.URL == "" || provider.CatalogEndpointURL() != provider.Catalog.Endpoint.URL {
		t.Fatalf("catalog endpoint = %q", provider.Catalog.Endpoint.URL)
	}
	if provider.Catalog.Endpoint.ProtocolOptions.OpenAI == nil {
		t.Fatal("OpenAI-compatible protocol options are missing")
	}
	if provider.Credentials == nil || len(provider.Credentials.CatalogAcquisition.Alternatives) == 0 {
		t.Fatal("catalog credential metadata is missing")
	}
	profiles := make(map[catalogs.ProviderCredentialProfileID]catalogs.ProviderCredentialProfile,
		len(provider.Credentials.Profiles))
	for _, profile := range provider.Credentials.Profiles {
		profiles[profile.ID] = profile
	}
	fields := make(map[catalogs.ProviderCredentialFieldID]struct{}, len(provider.Credentials.Fields))
	for _, field := range provider.Credentials.Fields {
		fields[field.ID] = struct{}{}
	}
	for _, profileID := range provider.Credentials.CatalogAcquisition.Alternatives {
		profile, found := profiles[profileID]
		if !found {
			t.Fatalf("catalog credential profile %q is missing", profileID)
		}
		for _, fieldID := range profile.Fields {
			if _, found := fields[fieldID]; !found {
				t.Errorf("catalog credential field %q is missing", fieldID)
			}
		}
	}
	if err := provider.ValidateContract(); err != nil {
		t.Fatalf("provider contract: %v", err)
	}
}

func assertAuthorMapping(
	t *testing.T,
	mapping *catalogs.AuthorMapping,
	source openai.Model,
	converted *catalogs.Model,
) {
	t.Helper()
	if mapping == nil {
		if len(converted.Authors) != 0 {
			t.Fatalf("model %q has authors without an author mapping: %#v", source.ID, converted.Authors)
		}
		return
	}
	value := source.ID
	if mapping.Field == "owned_by" {
		value = source.OwnedBy
	}
	authorID, found := mapping.Resolve(value)
	if !found {
		if len(converted.Authors) != 0 {
			t.Fatalf("unmapped author value %q produced %#v", value, converted.Authors)
		}
		return
	}
	if len(converted.Authors) != 1 || converted.Authors[0].ID != authorID {
		t.Fatalf("author value %q produced %#v, want %q", value, converted.Authors, authorID)
	}
}

func assertFieldMapping(
	t *testing.T,
	mapping catalogs.FieldMapping,
	source openai.Model,
	converted *catalogs.Model,
) bool {
	t.Helper()
	value, present := mappedSourceValue(mapping.From, source)
	if !present {
		return false
	}
	switch mapping.To {
	case "limits.context_window":
		assertMappedLimit(t, converted, mapping, value)
	case "limits.input_tokens":
		assertMappedLimit(t, converted, mapping, value)
	case "limits.output_tokens":
		assertMappedLimit(t, converted, mapping, value)
	case "name":
		if converted.Name != value.(string) {
			t.Errorf("mapping %s->%s = %q, want %q", mapping.From, mapping.To, converted.Name, value)
		}
	case "description":
		if converted.Description != value.(string) {
			t.Errorf("mapping %s->%s = %q, want %q", mapping.From, mapping.To, converted.Description, value)
		}
	case "metadata.tags":
		if converted.Metadata == nil {
			t.Fatalf("mapping %s->%s did not create metadata", mapping.From, mapping.To)
		}
		want := make([]catalogs.ModelTag, 0, len(value.([]string)))
		for _, tag := range value.([]string) {
			want = append(want, catalogs.ModelTag(tag))
		}
		if !reflect.DeepEqual(converted.Metadata.Tags, want) {
			t.Errorf("mapping %s->%s = %#v, want %#v", mapping.From, mapping.To, converted.Metadata.Tags, want)
		}
	default:
		t.Fatalf("catalog contains unsupported destination %q", mapping.To)
	}
	return true
}

func assertMappedLimit(
	t *testing.T,
	model *catalogs.Model,
	mapping catalogs.FieldMapping,
	value any,
) {
	t.Helper()
	if model.Limits == nil {
		t.Fatalf("mapping %s->%s did not create limits", mapping.From, mapping.To)
	}
	want := *value.(*int64)
	var actual int64
	switch mapping.To {
	case "limits.context_window":
		actual = model.Limits.ContextWindow
	case "limits.input_tokens":
		actual = model.Limits.InputTokens
	case "limits.output_tokens":
		actual = model.Limits.OutputTokens
	}
	if actual != want {
		t.Errorf("mapping %s->%s = %d, want %d", mapping.From, mapping.To, actual, want)
	}
}

func mappedSourceValue(path string, model openai.Model) (any, bool) {
	switch path {
	case "max_model_len":
		return model.MaxModelLen, model.MaxModelLen != nil
	case "context_window":
		return model.ContextWindow, model.ContextWindow != nil
	case "context_length":
		return model.ContextLength, model.ContextLength != nil
	case "max_completion_tokens":
		return model.MaxCompletionTokens, model.MaxCompletionTokens != nil
	case "max_output_length":
		return model.MaxOutputLength, model.MaxOutputLength != nil
	case "input_token_limit":
		return model.InputTokenLimit, model.InputTokenLimit != nil
	case "output_token_limit":
		return model.OutputTokenLimit, model.OutputTokenLimit != nil
	case "name":
		return model.Name, model.Name != ""
	case "id":
		return model.ID, model.ID != ""
	case "owned_by":
		return model.OwnedBy, model.OwnedBy != ""
	case "metadata.description":
		if model.Metadata != nil {
			return model.Metadata.Description, model.Metadata.Description != ""
		}
	case "metadata.context_length":
		if model.Metadata != nil {
			return model.Metadata.ContextLength, model.Metadata.ContextLength != nil
		}
	case "metadata.max_tokens":
		if model.Metadata != nil {
			return model.Metadata.MaxTokens, model.Metadata.MaxTokens != nil
		}
	case "metadata.tags":
		if model.Metadata != nil {
			return model.Metadata.Tags, len(model.Metadata.Tags) != 0
		}
	}
	return nil, false
}
