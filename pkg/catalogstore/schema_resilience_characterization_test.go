package catalogstore

import (
	stderrors "errors"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

// TestF009CharacterizationMalformedPayloadSiblingRejectsWholeGeneration pins
// strict whole-payload decoding. P4.8 must add bounded per-record quarantine
// while retaining strict envelope, schema, identity, and digest validation.
func TestF009CharacterizationMalformedPayloadSiblingRejectsWholeGeneration(t *testing.T) {
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
	if err == nil {
		t.Fatal("F-009 characterization changed: malformed payload sibling did not reject generation")
	}
	if catalog != nil {
		t.Fatalf("F-009 characterization changed: partial generation escaped: %#v", catalog)
	}
	var parseErr *pkgerrors.ParseError
	if !stderrors.As(err, &parseErr) {
		t.Fatalf("error = %T: %v, want *errors.ParseError", err, err)
	}
}
