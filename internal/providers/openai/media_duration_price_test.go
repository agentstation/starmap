package openai

import (
	"encoding/json/v2"
	"reflect"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestMetadataDurationPricesKeepTheirUnits(t *testing.T) {
	input, generated, zero := 0.002, 0.075, 0.0
	for _, tc := range []struct {
		name   string
		source *ModelMetadataPricing
		want   map[string]float64
	}{
		{"both", &ModelMetadataPricing{InputSeconds: &input, OutputSeconds: &generated}, map[string]float64{"input_second": input, "output_second": generated}},
		{"input", &ModelMetadataPricing{InputSeconds: &input}, map[string]float64{"input_second": input}},
		{"explicit zero", &ModelMetadataPricing{OutputSeconds: &zero}, map[string]float64{"output_second": 0}},
		{"unknown", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pricing := &catalogs.ModelPricing{}
			applyOpenAICompatibleMetadataPricing(pricing, tc.source)
			raw, err := json.Marshal(pricing.Operations)
			if err != nil {
				t.Fatal(err)
			}
			var prices map[string]float64
			if err := json.Unmarshal(raw, &prices); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(prices, tc.want) {
				t.Fatalf("duration prices lost their units or invented a charge: got %s; want %v", raw, tc.want)
			}
		})
	}
}
