package catalogs

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestModelTokenPricingRoundTripsEveryTokenCostField(t *testing.T) {
	t.Parallel()

	costType := reflect.TypeOf(ModelTokenCost{})
	original := &ModelTokenPricing{}
	value := reflect.ValueOf(original).Elem()
	structType := value.Type()

	// Give each field a distinct pair of prices. A branch that writes the
	// wrong field then fails on the value, not only on a missing key.
	filled := 0
	for i := range structType.NumField() {
		field := structType.Field(i)
		if field.Type != reflect.PointerTo(costType) {
			t.Fatalf("field %s has type %s, want *ModelTokenCost; extend this test", field.Name, field.Type)
		}
		filled++
		value.Field(i).Set(reflect.ValueOf(&ModelTokenCost{
			PerToken: float64(filled) * 0.000001,
			Per1M:    float64(filled) * 1.5,
		}))
	}
	if filled == 0 {
		t.Fatal("ModelTokenPricing declares no token cost fields")
	}

	for _, encoding := range []struct {
		name      string
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{
		{"json", json.Marshal, json.Unmarshal},
		{"yaml", yaml.Marshal, yaml.Unmarshal},
	} {
		t.Run(encoding.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := encoding.marshal(original)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var decoded ModelTokenPricing
			if err := encoding.unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("unmarshal %s: %v", encoded, err)
			}

			decodedValue := reflect.ValueOf(&decoded).Elem()
			for i := range structType.NumField() {
				name := structType.Field(i).Name
				want := value.Field(i).Interface().(*ModelTokenCost)
				got, _ := decodedValue.Field(i).Interface().(*ModelTokenCost)
				if got == nil {
					t.Errorf("%s is nil after the round trip; encoded form:\n%s", name, encoded)
					continue
				}
				if *got != *want {
					t.Errorf("%s = %+v, want %+v", name, *got, *want)
				}
			}
		})
	}
}

// TestModelTokenPricingAudioIsSeparateFromOperationAudio pins the distinction
// the two audio prices carry. ModelTokenPricing.AudioInput is a rate per
// million audio tokens. ModelOperationPricing.AudioInput is a flat price for
// one audio input. A catalog that stores one under the other misprices an
// audio turn by orders of magnitude.
func TestModelTokenPricingAudioIsSeparateFromOperationAudio(t *testing.T) {
	t.Parallel()

	flat := 0.006
	pricing := ModelPricing{
		Currency:   ModelPricingCurrencyUSD,
		Tokens:     &ModelTokenPricing{AudioInput: &ModelTokenCost{Per1M: 1.0}},
		Operations: &ModelOperationPricing{AudioInput: &flat},
	}

	encoded, err := yaml.Marshal(&pricing)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ModelPricing
	if err := yaml.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal %s: %v", encoded, err)
	}

	if decoded.Tokens == nil || decoded.Tokens.AudioInput == nil ||
		decoded.Tokens.AudioInput.Per1M != 1.0 {
		t.Errorf("token audio price = %+v, want per 1M of 1.0; encoded form:\n%s", decoded.Tokens, encoded)
	}
	if decoded.Operations == nil || decoded.Operations.AudioInput == nil ||
		*decoded.Operations.AudioInput != flat {
		t.Errorf("operation audio price = %+v, want %v", decoded.Operations, flat)
	}
}
