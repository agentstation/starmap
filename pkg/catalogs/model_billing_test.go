package catalogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestTextChatBillingWireContract(t *testing.T) {
	input := []byte(`{"text_chat":{"input":["input","cache_read"],"output":["output"],"request_charge":false}}`)
	var billing ModelBilling
	if err := json.Unmarshal(input, &billing); err != nil {
		t.Fatal(err)
	}
	if err := billing.Validate(); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(billing)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"text_chat"`)) || !bytes.Contains(encoded, []byte(`"cache_read"`)) {
		t.Fatalf("billing contract lost on round trip: %s", encoded)
	}
}

func recognitionBilling(basis RecognitionBillingBasis) *ModelBilling {
	return &ModelBilling{Recognition: &RecognitionBilling{Basis: basis}}
}

func TestRecognitionBillingSurvivesPriceReplacement(t *testing.T) {
	original := Model{Billing: recognitionBilling(RecognitionBillingTokens), Pricing: &ModelPricing{Currency: "USD", Operations: &ModelOperationPricing{PageInput: price(0.001)}}}
	replacement := Model{Pricing: &ModelPricing{Currency: "EUR", Tokens: &ModelTokenPricing{Input: &ModelTokenCost{Per1M: 0.3}}}}
	merged := MergeModels(original, replacement)
	if merged.Billing == nil || merged.Billing.Recognition.Basis != RecognitionBillingTokens {
		t.Fatal("price replacement lost the billing basis")
	}
	if !reflect.DeepEqual(merged.Pricing, replacement.Pricing) {
		t.Fatal("pricing must remain one complete selected record")
	}
	merged.Billing.Recognition.Basis = RecognitionBillingPages
	if original.Billing.Recognition.Basis != RecognitionBillingTokens {
		t.Fatal("merge shares billing state with its input")
	}
}

func TestRecognitionBillingWirePresenceAndIsolation(t *testing.T) {
	for _, wire := range []struct {
		name   string
		encode func(any) ([]byte, error)
		decode func([]byte, any) error
	}{
		{"json", json.Marshal, json.Unmarshal},
		{"yaml", func(v any) ([]byte, error) { return yaml.Marshal(v) }, func(b []byte, v any) error { return yaml.Unmarshal(b, v) }},
	} {
		t.Run(wire.name, func(t *testing.T) {
			for _, state := range []ValuePresence{ValueMissing, ValueUnknown, ValueKnown} {
				original := Model{ID: "recognition", Name: "Recognition"}
				if state == ValueKnown {
					original.Billing = recognitionBilling(RecognitionBillingTokens)
				}
				if state == ValueUnknown {
					original.SetRecordUnknown(ModelRecordBilling)
				}
				data, err := wire.encode(original)
				if err != nil {
					t.Fatal(err)
				}
				var restored Model
				if err := wire.decode(data, &restored); err != nil {
					t.Fatal(err)
				}
				if restored.RecordPresence(ModelRecordBilling) != state {
					t.Fatalf("billing presence = %v, want %v", restored.RecordPresence(ModelRecordBilling), state)
				}
			}
		})
	}
	original := Model{Billing: recognitionBilling(RecognitionBillingTokens)}
	original.Billing.Recognition.InputPageEstimate = &RecognitionInputPageEstimate{Tokens: 258, Source: "https://example.test/billing", Assumptions: "One page at the documented resolution; excludes output."}
	copied := DeepCopyModel(original)
	copied.Billing.Recognition.InputPageEstimate.Tokens = 1
	if original.Billing.Recognition.InputPageEstimate.Tokens != 258 {
		t.Fatal("copy shares estimate state")
	}
	offering := ProviderOffering{Billing: original.Billing}
	copiedOffering := copyProviderOffering(offering)
	copiedOffering.Billing.Recognition.Basis = RecognitionBillingPages
	if offering.Billing.Recognition.Basis != RecognitionBillingTokens {
		t.Fatal("offering copy shares billing state")
	}
}

func TestRecognitionBillingValidation(t *testing.T) {
	for _, test := range []struct {
		name    string
		billing *ModelBilling
		valid   bool
	}{
		{"absent", nil, true},
		{"empty", &ModelBilling{}, true},
		{"pages", recognitionBilling(RecognitionBillingPages), true},
		{"tokens", recognitionBilling(RecognitionBillingTokens), true},
		{"missing basis", recognitionBilling(""), false},
		{"unknown basis", recognitionBilling("characters"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.billing.Validate(); (err == nil) != test.valid {
				t.Fatalf("Validate = %v, want valid %v", err, test.valid)
			}
		})
	}
	for _, tokens := range []float64{-1, 0, math.NaN(), math.Inf(1), 258} {
		billing := recognitionBilling(RecognitionBillingTokens)
		billing.Recognition.InputPageEstimate = &RecognitionInputPageEstimate{Tokens: tokens, Source: "https://example.test/billing", Assumptions: "One page; excludes output."}
		if err := billing.Validate(); (err == nil) != (tokens == 258) {
			t.Fatalf("estimate %v: %v", tokens, err)
		}
	}
	billing := recognitionBilling(RecognitionBillingPages)
	billing.Recognition.InputPageEstimate = &RecognitionInputPageEstimate{Tokens: 258, Source: "https://example.test", Assumptions: "One page"}
	if err := billing.Validate(); err == nil {
		t.Fatal("page billing accepted token estimate")
	}
}

func TestRecognitionBillingPayloadRequiresCompatibleConsumer(t *testing.T) {
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	authored := Model{ID: "model", Name: "Model", Authors: []Author{{ID: "author", Name: "Author"}}}
	if err := builder.SetAuthorModel("author", authored); err != nil {
		t.Fatal(err)
	}
	model := Model{ID: "model", ModelRef: "author/model", Name: "Model", Billing: recognitionBilling(RecognitionBillingTokens)}
	if err := builder.SetProvider(Provider{ID: "provider", Name: "Provider", Models: map[string]*Model{"model": &model}}); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := DecodeCatalogPayload(encoded)
	if err != nil {
		t.Fatal(err)
	}
	offering, err := catalog.Offering("provider", "model")
	if err != nil || offering.Billing == nil || offering.Billing.Recognition.Basis != RecognitionBillingTokens {
		t.Fatal("offering lost billing declaration")
	}
	var payload CatalogPayload
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	payload.SchemaVersion = CanonicalAliasSchemaVersion
	downgraded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCatalogPayload(downgraded); err == nil {
		t.Fatal("legacy schema accepted billing semantics")
	}
	authored.Billing = recognitionBilling(RecognitionBillingPages)
	if err := builder.SetAuthorModel("author", authored); err == nil {
		t.Fatal("provider billing accepted as intrinsic author fact")
	}
}

func TestTextChatBillingValidation(t *testing.T) {
	for _, test := range []struct {
		name, wire string
		valid      bool
	}{
		{"standard", `{"input":["input"],"output":["output"],"request_charge":false}`, true},
		{"all classes", `{"input":["input","cache_read","cache_write"],"output":["output","reasoning"],"request_charge":true}`, true},
		{"missing request decision", `{"input":["input"],"output":["output"]}`, false},
		{"null request decision", `{"input":["input"],"output":["output"],"request_charge":null}`, false},
		{"missing input", `{"output":["output"],"request_charge":false}`, false},
		{"missing ordinary input", `{"input":["cache_read"],"output":["output"],"request_charge":false}`, false},
		{"missing output", `{"input":["input"],"request_charge":false}`, false},
		{"crossed classes", `{"input":["input","reasoning"],"output":["output"],"request_charge":false}`, false},
		{"repeated class", `{"input":["input","input"],"output":["output"],"request_charge":false}`, false},
		{"unknown class", `{"input":["input","future"],"output":["output"],"request_charge":false}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var billing ModelBilling
			if err := json.Unmarshal([]byte(`{"text_chat":`+test.wire+`}`), &billing); err != nil {
				t.Fatal(err)
			}
			if err := billing.Validate(); (err == nil) != test.valid {
				t.Fatalf("Validate = %v, want valid %v", err, test.valid)
			}
		})
	}
}

func TestTextChatBillingCopiesAndPayloadVersions(t *testing.T) {
	var model Model
	if err := json.Unmarshal([]byte(`{"id":"model","name":"Model","model":"author/model","billing":{"text_chat":{"input":["input","cache_read"],"output":["output"],"request_charge":false}}}`), &model); err != nil {
		t.Fatal(err)
	}
	for _, copied := range []Model{DeepCopyModel(model), MergeModels(model, Model{Pricing: &ModelPricing{Currency: "USD"}})} {
		copied.Billing.TextChat.Input[0] = TokenBillingCacheWrite
		copied.Billing.TextChat.Output[0] = TokenBillingReasoning
		*copied.Billing.TextChat.RequestCharge = true
	}
	if model.Billing.TextChat.Input[0] != TokenBillingInput || model.Billing.TextChat.Output[0] != TokenBillingOutput || *model.Billing.TextChat.RequestCharge {
		t.Fatal("copied billing shares mutable state")
	}
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetAuthorModel("author", Model{ID: "model", Name: "Model", Authors: []Author{{ID: "author", Name: "Author"}}}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetProvider(Provider{ID: "provider", Name: "Provider", Models: map[string]*Model{"model": &model}}); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := DecodeCatalogPayload(encoded)
	if err != nil {
		t.Fatal(err)
	}
	offering, err := catalog.Offering("provider", "model")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(offering.Billing, model.Billing) {
		t.Fatal("catalog lost text billing")
	}
	offering.Billing.TextChat.Input[0] = TokenBillingCacheWrite
	again, err := catalog.Offering("provider", "model")
	if err != nil {
		t.Fatal(err)
	}
	if again.Billing.TextChat.Input[0] != TokenBillingInput {
		t.Fatal("offering exposes catalog billing mutation")
	}
	for _, version := range []uint64{CanonicalAliasSchemaVersion, RecognitionBillingSchemaVersion} {
		lowered := bytes.Replace(encoded, []byte(fmt.Sprintf(`"schema_version":%d`, CurrentCatalogSchemaVersion)), []byte(fmt.Sprintf(`"schema_version":%d`, version)), 1)
		if _, err := DecodeCatalogPayload(lowered); err == nil {
			t.Fatalf("schema %d accepted text billing", version)
		}
	}
	yamlData, err := yaml.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	var restored Model
	if err := yaml.Unmarshal(yamlData, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(model.Billing, restored.Billing) {
		t.Fatal("YAML lost text billing")
	}
}
