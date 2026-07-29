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
		"schema_version": 3,
		"providers": [{"id":"provider","name":"Provider"}],
		"authors": [{"id":"author","name":"Author"}],
		"provider_models": {
			"provider": [
				{"id":"valid","model":"author/valid","name":"Valid"},
				{"id":"invalid","name":"Invalid","limits":{"context_window":"schema-drift","input_tokens":0,"output_tokens":0}}
			]
		},
		"author_models": {
			"author": [
				{"id":"valid","name":"Valid","authors":[{"id":"author","name":"Author"}]}
			]
		},
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
	provider, providerErr := catalog.Provider("provider")
	if providerErr != nil {
		t.Fatalf("Provider: %v", providerErr)
	}
	if valid, found := provider.Models["valid"]; !found || valid.Name != "Valid" {
		t.Fatalf("valid model = %#v, found %v", valid, found)
	}
	if _, found := provider.Models["invalid"]; found {
		t.Fatal("malformed model was not quarantined")
	}
	if quarantineErr.Report.Rejected != 1 || len(quarantineErr.Report.Issues) != 1 {
		t.Fatalf("quarantine report = %#v, want one rejected record", quarantineErr.Report)
	}
}

func TestMalformedAuthoredModelSiblingReturnsPartialDiagnostic(t *testing.T) {
	payload := []byte(`{
		"schema_version": 3,
		"providers": [],
		"authors": [{"id":"author","name":"Author"}],
		"provider_models": {},
		"author_models": {
			"author": [
				{"id":"valid","name":"Valid","authors":[{"id":"author","name":"Author"}]},
				{"id":"invalid","name":"Invalid","authors":[{"id":"author","name":"Author"}],
				 "pricing":{"currency":"USD","tokens":{"input":{"per_1m":1}}}}
			]
		},
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
	valid, findErr := catalog.Definition("author/valid")
	if findErr != nil || valid.Name != "Valid" {
		t.Fatalf("valid authored model = %#v, %v", valid, findErr)
	}
	if _, findErr := catalog.Definition("author/invalid"); findErr == nil {
		t.Fatal("invalid authored model was not quarantined")
	}
	if quarantineErr.Report.Rejected != 1 || len(quarantineErr.Report.Issues) != 1 {
		t.Fatalf("quarantine report = %#v, want one rejected record", quarantineErr.Report)
	}
}

func TestMalformedProviderAndAuthorModelsReturnOneCompleteQuarantineReport(t *testing.T) {
	payload := []byte(`{
		"schema_version": 3,
		"providers": [{"id":"provider","name":"Provider"}],
		"authors": [{"id":"author","name":"Author"}],
		"provider_models": {
			"provider": [
				{"id":"invalid","name":"Invalid","limits":{"context_window":"schema-drift"}}
			]
		},
		"author_models": {
			"author": [
				{"id":"invalid","name":"Invalid","authors":"schema-drift"}
			]
		},
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
	if quarantineErr.Report.Rejected != 2 || len(quarantineErr.Report.Issues) != 2 {
		t.Fatalf("quarantine report = %#v, want both rejected records", quarantineErr.Report)
	}
}

func TestSourceObservationPayloadMayRemainUnresolvedButCannotActivate(t *testing.T) {
	payload := []byte(`{
		"schema_version": 3,
		"providers": [{"id":"provider","name":"Provider"}],
		"authors": [],
		"provider_models": {
			"provider": [{"id":"unresolved","name":"Unresolved Provider Record"}]
		},
		"author_models": {},
		"provenance": {}
	}`)

	if catalog, err := DecodeCatalogPayload(payload); err == nil || catalog != nil {
		t.Fatalf("DecodeCatalogPayload = (%#v, %v), want activation failure", catalog, err)
	}
	observation, err := DecodeSourceObservationPayload(payload)
	if err != nil {
		t.Fatalf("DecodeSourceObservationPayload: %v", err)
	}
	provider, err := observation.Provider("provider")
	model := provider.Models["unresolved"]
	if err != nil || model == nil || model.Name != "Unresolved Provider Record" {
		t.Fatalf("unresolved observation model = %#v, %v", model, err)
	}
	if got := observation.Definitions(); len(got) != 0 {
		t.Fatalf("source observation derived definitions = %#v, want none", got)
	}
}

func TestPrelaunchSchemaVersionTwoIsRejected(t *testing.T) {
	payload := []byte(`{
		"schema_version":2,
		"providers":[],
		"authors":[],
		"provider_models":{},
		"author_models":{},
		"provenance":{}
	}`)
	catalog, err := DecodeCatalogPayload(payload)
	if err == nil || catalog != nil {
		t.Fatalf("DecodeCatalogPayload = %#v, %v; want schema rejection", catalog, err)
	}
	var validationErr *pkgerrors.ValidationError
	if !stderrors.As(err, &validationErr) {
		t.Fatalf("error = %T: %v, want *errors.ValidationError", err, err)
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
				"schema_version":3,
				"providers":[{"id":"provider","name":"Provider"}],
				"authors":[],
				"provider_models":{"provider":null},
				"author_models":{},
				"provenance":{}
			}`,
		},
		{
			name: "missing provider model identity",
			payload: `{
				"schema_version":3,
				"providers":[{"id":"provider","name":"Provider"}],
				"authors":[],
				"provider_models":{},
				"author_models":{},
				"provenance":{}
			}`,
		},
		{
			name: "missing author model identity",
			payload: `{
				"schema_version":3,
				"providers":[],
				"authors":[{"id":"author","name":"Author"}],
				"provider_models":{},
				"author_models":{},
				"provenance":{}
			}`,
		},
		{
			name: "author model identity references unknown author",
			payload: `{
				"schema_version":3,
				"providers":[],
				"authors":[],
				"provider_models":{},
				"author_models":{"author":[]},
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
