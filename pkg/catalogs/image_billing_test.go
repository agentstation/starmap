package catalogs

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestImageBillingContract(t *testing.T) {
	var billing ModelBilling
	if err := json.Unmarshal([]byte(`{"images":{"basis":"images","operations":["images-generations"],"default_images":1,"request_charge":false}}`), &billing); err != nil {
		t.Fatal(err)
	}
	if err := billing.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, wire := range []string{
		`{"basis":"tokens","operations":["images-generations"],"default_images":1,"request_charge":false}`,
		`{"basis":"images","operations":[],"default_images":1,"request_charge":false}`,
		`{"basis":"images","operations":["images-generations","images-generations"],"default_images":1,"request_charge":false}`,
		`{"basis":"images","operations":["chat-completions"],"default_images":1,"request_charge":false}`,
		`{"basis":"images","operations":["images-generations"],"default_images":0,"request_charge":false}`,
		`{"basis":"images","operations":["images-generations"],"default_images":1}`,
		`{"basis":"pixel_iterations","operations":["images-generations"],"default_images":1,"request_charge":false}`,
	} {
		var value ModelBilling
		if err := json.Unmarshal([]byte(`{"images":`+wire+`}`), &value); err != nil {
			t.Fatal(err)
		}
		if value.Validate() == nil {
			t.Fatalf("accepted incomplete contract: %s", wire)
		}
	}
	rate := 0.04
	model := Model{ID: "model", Name: "Model", ModelRef: "author/model", Billing: &billing, Pricing: &ModelPricing{Currency: "USD", Operations: &ModelOperationPricing{ImageUnit: &rate}}}
	for _, copy := range []Model{DeepCopyModel(model), MergeModels(model, Model{Name: "Updated"})} {
		*copy.Billing.Images.RequestCharge = true
		copy.Billing.Images.Operations[0] = ProviderOperationImagesEdits
		*copy.Pricing.Operations.ImageUnit = 1
		if *model.Billing.Images.RequestCharge || model.Billing.Images.Operations[0] != ProviderOperationImagesGenerations || *model.Pricing.Operations.ImageUnit != rate {
			t.Fatal("copy shares image billing state")
		}
	}
	raw, err := yaml.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	var restored Model
	if err := yaml.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Billing.Images.Basis != ImageBillingImages || *restored.Pricing.Operations.ImageUnit != rate {
		t.Fatal("YAML lost image billing")
	}
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetAuthorModel("author", Model{ID: "model", Name: "Model", Authors: []Author{{ID: "author", Name: "Author"}}}); err != nil {
		t.Fatal(err)
	}
	for _, declared := range []bool{true, false} {
		if !declared {
			model.Billing = nil
		}
		if err := builder.SetProvider(Provider{ID: "provider", Name: "Provider", Models: map[string]*Model{"model": &model}}); err != nil {
			t.Fatal(err)
		}
		payload, err := EncodeCatalogPayload(builder)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeCatalogPayload(payload)
		if err != nil {
			t.Fatal(err)
		}
		offering, err := decoded.Offering("provider", "model")
		if err != nil {
			t.Fatal(err)
		}
		if *offering.Pricing.Operations.ImageUnit != rate {
			t.Fatal("payload lost image price")
		}
		old := bytes.Replace(payload, []byte(fmt.Sprintf(`"schema_version":%d`, CurrentCatalogSchemaVersion)), []byte(`"schema_version":16`), 1)
		if _, err := DecodeCatalogPayload(old); err == nil {
			t.Fatal("schema 16 accepted image units")
		}
	}
	negative := -1.0
	if (&ModelPricing{Operations: &ModelOperationPricing{ImageUnit: &negative}}).Validate() == nil {
		t.Fatal("negative image price accepted")
	}
}
