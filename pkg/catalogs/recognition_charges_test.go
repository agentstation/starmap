package catalogs

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"github.com/goccy/go-yaml"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecognitionCompleteChargeContract(t *testing.T) {
	for _, test := range []struct {
		name, wire string
		valid      bool
	}{
		{"unknown charges", `{"recognition":{"basis":"tokens"}}`, true},
		{"complete tokens", `{"recognition":{"basis":"tokens","input":["input","cache_read"],"output":["output"],"request_charge":false}}`, true},
		{"complete pages", `{"recognition":{"basis":"pages","request_charge":false}}`, true},
		{"missing output", `{"recognition":{"basis":"tokens","input":["input"],"request_charge":false}}`, false},
		{"missing request charge", `{"recognition":{"basis":"tokens","input":["input"],"output":["output"]}}`, false},
		{"page tokens conflict", `{"recognition":{"basis":"pages","input":["input"],"request_charge":false}}`, false},
		{"unknown class", `{"recognition":{"basis":"tokens","input":["input","unknown"],"output":["output"],"request_charge":false}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var billing ModelBilling
			require.NoError(t, json.Unmarshal([]byte(test.wire), &billing))
			require.Equal(t, test.valid, billing.Validate() == nil)
			if test.valid {
				encoded, err := json.Marshal(billing)
				require.NoError(t, err)
				require.JSONEq(t, test.wire, string(encoded))
			}
		})
	}
}

func TestRecognitionChargesPayloadAndCopies(t *testing.T) {
	var billing ModelBilling
	require.NoError(t, json.Unmarshal([]byte(`{"recognition":{"basis":"tokens","input":["input","cache_read"],"output":["output"],"request_charge":false}}`), &billing))
	model := Model{ID: "model", Name: "Model", ModelRef: "author/model", Billing: &billing}
	wire, err := yaml.Marshal(model)
	require.NoError(t, err)
	var restored Model
	require.NoError(t, yaml.Unmarshal(wire, &restored))
	require.Equal(t, model.Billing, restored.Billing)
	for _, copied := range []Model{DeepCopyModel(model), MergeModels(model, Model{Pricing: &ModelPricing{Currency: "USD"}})} {
		*copied.Billing.Recognition.RequestCharge = true
		copied.Billing.Recognition.Input[0] = TokenBillingCacheWrite
		copied.Billing.Recognition.Output[0] = TokenBillingReasoning
		require.False(t, *model.Billing.Recognition.RequestCharge)
		require.Equal(t, TokenBillingInput, model.Billing.Recognition.Input[0])
		require.Equal(t, TokenBillingOutput, model.Billing.Recognition.Output[0])
	}
	builder := NewEmpty()
	require.NoError(t, builder.SetAuthor(Author{ID: "author", Name: "Author"}))
	require.NoError(t, builder.SetAuthorModel("author", Model{ID: "model", Name: "Model", Authors: []Author{{ID: "author", Name: "Author"}}}))
	require.NoError(t, builder.SetProvider(Provider{ID: "provider", Name: "Provider", Models: map[string]*Model{"model": &model}}))
	payload, err := EncodeCatalogPayload(builder)
	require.NoError(t, err)
	catalog, err := DecodeCatalogPayload(payload)
	require.NoError(t, err)
	offering, err := catalog.Offering("provider", "model")
	require.NoError(t, err)
	require.Equal(t, billing.Recognition, offering.Billing.Recognition)
	offering.Billing.Recognition.Input[0] = TokenBillingCacheWrite
	*offering.Billing.Recognition.RequestCharge = true
	again, err := catalog.Offering("provider", "model")
	require.NoError(t, err)
	require.Equal(t, billing.Recognition, again.Billing.Recognition)
	old := bytes.Replace(payload, []byte(fmt.Sprintf(`"schema_version":%d`, CurrentCatalogSchemaVersion)), []byte(`"schema_version":12`), 1)
	_, err = DecodeCatalogPayload(old)
	require.Error(t, err, "schema 12 cannot silently accept new charge fields")
}
