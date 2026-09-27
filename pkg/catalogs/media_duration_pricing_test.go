package catalogs

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestMediaDurationPricesSurviveCopiesAndSchema(t *testing.T) {
	input, output := 0.002, 0.075
	model := Model{ID: "model", Name: "Model", ModelRef: "author/model", Pricing: &ModelPricing{Currency: "USD", Operations: &ModelOperationPricing{InputSecond: &input, OutputSecond: &output}}}
	if err := model.Pricing.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, copied := range []Model{DeepCopyModel(model), MergeModels(model, Model{Name: "Updated"})} {
		*copied.Pricing.Operations.InputSecond = 1
		*copied.Pricing.Operations.OutputSecond = 2
		if *model.Pricing.Operations.InputSecond != 0.002 || *model.Pricing.Operations.OutputSecond != 0.075 {
			t.Fatal("copy changed original duration price")
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
	if *restored.Pricing.Operations.InputSecond != input || *restored.Pricing.Operations.OutputSecond != output {
		t.Fatal("YAML lost duration units")
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
	decoded, err := DecodeCatalogPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	offering, err := decoded.Offering("provider", "model")
	if err != nil {
		t.Fatal(err)
	}
	if *offering.Pricing.Operations.InputSecond != input || *offering.Pricing.Operations.OutputSecond != output {
		t.Fatal("payload lost duration units")
	}
	if offering.Billing != nil {
		t.Fatal("a duration price invented a billing contract")
	}
	for version := legacyCatalogSchemaVersion; version < MediaDurationPricingSchemaVersion; version++ {
		old := bytes.Replace(payload, []byte(fmt.Sprintf(`"schema_version":%d`, CurrentCatalogSchemaVersion)), []byte(fmt.Sprintf(`"schema_version":%d`, version)), 1)
		if _, err := DecodeCatalogPayload(old); err == nil {
			t.Fatalf("schema %d accepted duration prices", version)
		}
	}
}

func TestMediaDurationPriceValidation(t *testing.T) {
	for _, value := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, operations := range []*ModelOperationPricing{{InputSecond: &value}, {OutputSecond: &value}} {
			if (&ModelPricing{Currency: "USD", Operations: operations}).Validate() == nil {
				t.Fatalf("accepted invalid duration rate %v", value)
			}
		}
	}
	zero := 0.0
	if err := (&ModelPricing{Currency: "USD", Operations: &ModelOperationPricing{OutputSecond: &zero}}).Validate(); err != nil {
		t.Fatal(err)
	}
}
