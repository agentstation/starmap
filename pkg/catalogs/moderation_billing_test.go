package catalogs

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"reflect"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestModerationBillingWireContract(t *testing.T) {
	input := []byte(`{"moderations":{"basis":"requests"}}`)
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
	if !bytes.Equal(input, encoded) {
		t.Fatalf("billing declaration lost: %s", encoded)
	}
}

func TestModerationBillingValidationAndCopies(t *testing.T) {
	for _, wire := range []string{`{}`, `{"basis":"pages"}`, `{"basis":null}`} {
		var billing ModelBilling
		if err := json.Unmarshal([]byte(`{"moderations":`+wire+`}`), &billing); err != nil {
			t.Fatal(err)
		}
		if billing.Validate() == nil {
			t.Fatalf("accepted incomplete declaration: %s", wire)
		}
	}
	model := Model{ID: "model", Name: "Model", ModelRef: "author/model", Billing: &ModelBilling{Moderations: &ModerationBilling{Basis: ModerationBillingRequests}}}
	yamlData, err := yaml.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	var restored Model
	if err := yaml.Unmarshal(yamlData, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(model.Billing, restored.Billing) {
		t.Fatal("YAML lost billing presence")
	}
	merged := MergeModels(model, Model{Pricing: &ModelPricing{Currency: "USD"}})
	merged.Billing.Moderations.Basis = "invalid"
	if model.Billing.Moderations.Basis != ModerationBillingRequests {
		t.Fatal("merge shares billing state")
	}
	copy := DeepCopyModel(model)
	copy.Billing.Moderations.Basis = "invalid"
	if model.Billing.Moderations.Basis != ModerationBillingRequests {
		t.Fatal("copy shares billing state")
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
	if offering.Billing.Moderations.Basis != ModerationBillingRequests {
		t.Fatal("payload lost moderation billing")
	}
	offering.Billing.Moderations.Basis = "invalid"
	again, err := catalog.Offering("provider", "model")
	if err != nil {
		t.Fatal(err)
	}
	if again.Billing.Moderations.Basis != ModerationBillingRequests {
		t.Fatal("offering exposes mutable billing")
	}
	old := bytes.Replace(payload, []byte(fmt.Sprintf(`"schema_version":%d`, CurrentCatalogSchemaVersion)), []byte(`"schema_version":13`), 1)
	if _, err := DecodeCatalogPayload(old); err == nil {
		t.Fatal("schema 13 accepted moderation billing")
	}
}
