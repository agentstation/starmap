package catalogs

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestSpeechBillingContract(t *testing.T) {
	const wire = `{"speech":{"basis":"unicode_code_points","max_input_characters":4096,"request_charge":false}}`
	var billing ModelBilling
	if err := json.Unmarshal([]byte(wire), &billing); err != nil {
		t.Fatal(err)
	}
	if err := billing.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`{"basis":"tokens","max_input_characters":4096,"request_charge":false}`, `{"basis":"unicode_code_points","max_input_characters":0,"request_charge":false}`, `{"basis":"unicode_code_points","max_input_characters":4096}`} {
		var value ModelBilling
		if err := json.Unmarshal([]byte(`{"speech":`+bad+`}`), &value); err != nil {
			t.Fatal(err)
		}
		if value.Validate() == nil {
			t.Fatalf("accepted incomplete contract: %s", bad)
		}
	}
	rate := 0.000015
	model := Model{ID: "model", Name: "Model", ModelRef: "author/model", Billing: &billing, Pricing: &ModelPricing{Currency: "USD", Operations: &ModelOperationPricing{CharacterInput: &rate}}}
	copied := DeepCopyModel(model)
	*copied.Billing.Speech.RequestCharge = true
	*copied.Pricing.Operations.CharacterInput = 1
	if *model.Billing.Speech.RequestCharge || *model.Pricing.Operations.CharacterInput != rate {
		t.Fatal("copy shares speech state")
	}
	merged := MergeModels(model, Model{Name: "Updated"})
	*merged.Billing.Speech.RequestCharge = true
	*merged.Pricing.Operations.CharacterInput = 2
	if *model.Billing.Speech.RequestCharge || *model.Pricing.Operations.CharacterInput != rate {
		t.Fatal("merge shares speech state")
	}
	data, err := yaml.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	var restored Model
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Billing.Speech.Basis != SpeechBillingCodePoints || *restored.Pricing.Operations.CharacterInput != rate {
		t.Fatal("YAML lost speech contract")
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
	payload, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := DecodeCatalogPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	offering, err := catalog.Offering("provider", "model")
	if err != nil {
		t.Fatal(err)
	}
	if *offering.Pricing.Operations.CharacterInput != rate || offering.Billing.Speech.MaxInputCharacters != 4096 {
		t.Fatal("payload lost speech contract")
	}
	old := bytes.Replace(payload, []byte(fmt.Sprintf(`"schema_version":%d`, CurrentCatalogSchemaVersion)), []byte(`"schema_version":15`), 1)
	if _, err := DecodeCatalogPayload(old); err == nil {
		t.Fatal("schema 15 accepted character billing")
	}
	model.Billing = nil
	if err := builder.SetProviderModel("provider", model); err != nil {
		t.Fatal(err)
	}
	priceOnly, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	old = bytes.Replace(priceOnly, []byte(fmt.Sprintf(`"schema_version":%d`, CurrentCatalogSchemaVersion)), []byte(`"schema_version":15`), 1)
	if _, err := DecodeCatalogPayload(old); err == nil {
		t.Fatal("schema 15 accepted character pricing without billing")
	}
	negative := -1.0
	if (&ModelPricing{Currency: "USD", Operations: &ModelOperationPricing{CharacterInput: &negative}}).Validate() == nil {
		t.Fatal("accepted negative character price")
	}
}
