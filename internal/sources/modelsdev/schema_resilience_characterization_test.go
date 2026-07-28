package modelsdev

import "testing"

// TestF009CharacterizationMalformedModelsDevSiblingRejectsWholeAPI pins the
// monolithic models.dev decode. P4.8 must decode bounded records independently
// so the valid sibling survives with typed rejection evidence for the invalid
// record.
func TestF009CharacterizationMalformedModelsDevSiblingRejectsWholeAPI(t *testing.T) {
	payload := []byte(`{
		"provider": {
			"id": "provider",
			"name": "Provider",
			"models": {
				"valid": {
					"id": "valid",
					"name": "Valid",
					"limit": {"context": 8192, "input": 4096, "output": 4096}
				},
				"invalid": {
					"id": "invalid",
					"name": "Invalid",
					"limit": {"context": "schema-drift", "input": 4096, "output": 4096}
				}
			}
		}
	}`)

	api, err := parseAPIData(payload)
	if err == nil {
		t.Fatal("F-009 characterization changed: malformed sibling did not reject models.dev API")
	}
	if api != nil {
		t.Fatalf("F-009 characterization changed: partial models.dev API escaped: %#v", api)
	}
}
