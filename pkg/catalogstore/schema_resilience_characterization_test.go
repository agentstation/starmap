package catalogstore

import (
	"bytes"
	stderrors "errors"
	"testing"

	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

// TestF009MalformedPayloadSiblingReturnsPartialDiagnostic proves valid records
// remain inspectable while callers receive a typed error that prevents the
// partial catalog from being activated as a manifest-bound generation.
func TestF009MalformedPayloadSiblingReturnsPartialDiagnostic(t *testing.T) {
	payload := []byte(`{
		"schema_version": 1,
		"providers": [{"id":"provider","name":"Provider"}],
		"authors": [],
		"endpoints": [],
		"provider_models": {
			"provider": [
				{"id":"valid","name":"Valid"},
				{"id":"invalid","name":"Invalid","limits":{"context_window":"schema-drift","input_tokens":0,"output_tokens":0}}
			]
		},
		"author_models": {},
		"provenance": {}
	}`)

	catalog, err := DecodeCatalogPayload(payload)
	var quarantineErr *sourcepayload.QuarantineError
	if !stderrors.As(err, &quarantineErr) {
		t.Fatalf("error = %T: %v, want *sourcepayload.QuarantineError", err, err)
	}
	if catalog == nil {
		t.Fatal("partial diagnostic catalog is nil")
	}
	models, modelsErr := catalog.ProviderModels("provider")
	if modelsErr != nil {
		t.Fatalf("ProviderModels: %v", modelsErr)
	}
	if valid, found := models.Get("valid"); !found || valid.Name != "Valid" {
		t.Fatalf("valid model = %#v, found %v", valid, found)
	}
	if models.Exists("invalid") {
		t.Fatal("malformed model was not quarantined")
	}
	if quarantineErr.Report.Rejected != 1 || len(quarantineErr.Report.Issues) != 1 {
		t.Fatalf("quarantine report = %#v, want one rejected record", quarantineErr.Report)
	}
}

func TestCatalogPayloadDecodeIsSizeBounded(t *testing.T) {
	catalog, err := DecodeCatalogPayload(bytes.Repeat([]byte(" "), constants.MaxSourcePayloadBytes+1))
	if err == nil || catalog != nil {
		t.Fatalf("DecodeCatalogPayload = %#v, %v; want bounded failure", catalog, err)
	}
	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error = %T: %v, want *errors.ValidationError", err, err)
	}
}

func TestCatalogPayloadCollectionIdentityRemainsFailClosed(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name: "null provider model collection",
			payload: `{
				"schema_version":1,
				"providers":[{"id":"provider","name":"Provider"}],
				"authors":[],
				"endpoints":[],
				"provider_models":{"provider":null},
				"author_models":{},
				"provenance":{}
			}`,
		},
		{
			name: "missing provider model identity",
			payload: `{
				"schema_version":1,
				"providers":[{"id":"provider","name":"Provider"}],
				"authors":[],
				"endpoints":[],
				"provider_models":{},
				"author_models":{},
				"provenance":{}
			}`,
		},
		{
			name: "missing author model identity",
			payload: `{
				"schema_version":1,
				"providers":[],
				"authors":[{"id":"author","name":"Author"}],
				"endpoints":[],
				"provider_models":{},
				"author_models":{},
				"provenance":{}
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog, err := DecodeCatalogPayload([]byte(test.payload))
			if err == nil || catalog != nil {
				t.Fatalf("DecodeCatalogPayload = %#v, %v; want structural rejection", catalog, err)
			}
			var validationErr *pkgerrors.ValidationError
			if !stderrors.As(err, &validationErr) {
				t.Fatalf("error = %T: %v, want *errors.ValidationError", err, err)
			}
		})
	}
}
