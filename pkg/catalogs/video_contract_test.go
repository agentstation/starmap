package catalogs

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestVideoBillingContract(t *testing.T) {
	wire := []byte(`{"videos":{"basis":"output_seconds","default_seconds":5,"seconds":[5],"default_size":"1280x720","sizes":["1280x720"],"request_charge":false}}`)
	var billing ModelBilling
	if err := json.Unmarshal(wire, &billing); err != nil {
		t.Fatal(err)
	}
	if err := billing.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*VideoBilling){
		func(b *VideoBilling) { b.Basis = "video_count" },
		func(b *VideoBilling) { b.RequestCharge = nil },
		func(b *VideoBilling) { b.DefaultSeconds = 2 },
		func(b *VideoBilling) { b.Seconds = []int64{5, 5} },
		func(b *VideoBilling) { b.Seconds = []int64{5, -1} },
		func(b *VideoBilling) { b.DefaultSize = "640x480" },
		func(b *VideoBilling) { b.Sizes = []string{"1280x720", "0x720"} },
		func(b *VideoBilling) { b.Sizes = []string{"1280x720", "1280x720"} },
	} {
		copied := deepCopyModelBilling(&billing)
		change(copied.Videos)
		if copied.Validate() == nil {
			t.Fatal("invalid video billing accepted")
		}
	}
	model := Model{ID: "video", Name: "Video", ModelRef: "author/video", Billing: &billing}
	for _, copied := range []Model{DeepCopyModel(model), MergeModels(model, Model{Name: "changed"})} {
		copied.Billing.Videos.Seconds[0] = 99
		copied.Billing.Videos.Sizes[0] = "1x1"
		*copied.Billing.Videos.RequestCharge = true
		if billing.Videos.Seconds[0] != 5 || billing.Videos.Sizes[0] != "1280x720" || *billing.Videos.RequestCharge {
			t.Fatal("billing aliases caller memory")
		}
	}
	raw, err := yaml.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Model
	if err := yaml.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Billing.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := validatePayloadBillingSchema(model, 18); err == nil {
		t.Fatal("older schema accepted video billing")
	}
}

func TestExactModelEndpointOverridesAuthorAndProvider(t *testing.T) {
	endpoint := ProviderInferenceEndpoint{Operation: ProviderOperationVideosGenerations, Type: EndpointTypeOpenAI, Path: "/videos", StreamPath: "/events", ProtocolsByAuthor: map[AuthorID]EndpointType{"author": EndpointTypeAnthropic}, PathsByAuthor: map[AuthorID]string{"author": "/author/video"}, OverridesByModel: map[ProviderModelID]ProviderInferenceModelEndpoint{"Opaque/VIDEO": {Type: EndpointTypeDeepInfraVideo, Path: "/inference/{provider_model_id}"}}}
	provider := Provider{ID: "fixture", Name: "Fixture", Inference: &ProviderInference{BaseURL: "https://provider.example/v1", Endpoints: []ProviderInferenceEndpoint{endpoint}}}
	if err := provider.ValidateContract(); err != nil {
		t.Fatal(err)
	}
	capabilities := ProviderOfferingServiceCapabilities{Operations: []ProviderOperation{ProviderOperationVideosGenerations}}
	selected := deriveProviderOfferingEndpoints(provider, "author/video", "Opaque/VIDEO", capabilities)
	if len(selected) != 1 || selected[0].Type != EndpointTypeDeepInfraVideo || selected[0].URL != "https://provider.example/v1/inference/Opaque/VIDEO" || selected[0].StreamURL != "" {
		t.Fatalf("wrong exact endpoint: %#v", selected)
	}
	fallback := deriveProviderOfferingEndpoints(provider, "author/video", "opaque/video", capabilities)
	if len(fallback) != 1 || fallback[0].Type != EndpointTypeAnthropic || fallback[0].URL != "https://provider.example/v1/author/video" {
		t.Fatalf("opaque ID lost case: %#v", fallback)
	}
	rebound, err := provider.Inference.BindOfferingEndpoint(selected[0], "https://proxy.example/v1", nil)
	if err != nil || rebound.URL != "https://proxy.example/v1/inference/Opaque/VIDEO" {
		t.Fatalf("binding: %#v %v", rebound, err)
	}
	copied := DeepCopyProvider(provider)
	copied.Inference.Endpoints[0].OverridesByModel["Opaque/VIDEO"] = ProviderInferenceModelEndpoint{Type: EndpointTypeOpenAI, Path: "/wrong"}
	if provider.Inference.Endpoints[0].OverridesByModel["Opaque/VIDEO"].Type != EndpointTypeDeepInfraVideo {
		t.Fatal("override aliases caller memory")
	}
	builder := NewEmpty()
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	raw, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCatalogPayload(raw); err != nil {
		t.Fatal(err)
	}
	legacy := bytes.Replace(raw, []byte(`"schema_version":19`), []byte(`"schema_version":18`), 1)
	if bytes.Equal(raw, legacy) {
		t.Fatal("schema replacement failed")
	}
	if _, err := DecodeCatalogPayload(legacy); err == nil {
		t.Fatal("old schema accepted exact-model override")
	}
	for _, override := range []ProviderInferenceModelEndpoint{{Type: "unknown", Path: "/video"}, {Type: EndpointTypeDeepInfraVideo, Path: "relative"}, {Type: EndpointTypeDeepInfraVideo, Path: "/video", StreamPath: "relative"}} {
		copied.Inference.Endpoints[0].OverridesByModel["Opaque/VIDEO"] = override
		if copied.ValidateContract() == nil {
			t.Fatal("invalid override accepted")
		}
	}
}
