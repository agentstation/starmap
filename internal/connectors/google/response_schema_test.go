package google

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

type googlePathDecision struct {
	outcome string
	note    string
}

const (
	googleOutcomeCanonical = "canonical"
	googleOutcomeExtension = "extension"
	googleOutcomeIgnored   = "ignored"
)

func TestGoogleProviderShapePathsAreClassified(t *testing.T) {
	var response any
	if err := json.Unmarshal([]byte(googleProviderShapeFixture), &response); err != nil {
		t.Fatalf("failed to parse Google shape fixture: %v", err)
	}

	paths := collectGoogleScalarPaths(response)
	classified := googlePathDecisions()

	var missing []string
	for _, path := range paths {
		decision, ok := classified[path]
		if !ok {
			missing = append(missing, path)
			continue
		}
		if decision.note == "" {
			t.Fatalf("path %q has empty classification note", path)
		}
		switch decision.outcome {
		case googleOutcomeCanonical, googleOutcomeExtension, googleOutcomeIgnored:
		default:
			t.Fatalf("path %q has invalid outcome %q", path, decision.outcome)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("unclassified Google source paths:\n%s", strings.Join(missing, "\n"))
	}
}

func googlePathDecisions() map[string]googlePathDecision {
	return map[string]googlePathDecision{
		"models.[].name":                          {outcome: googleOutcomeCanonical, note: "provider model name"},
		"models.[].displayName":                   {outcome: googleOutcomeCanonical, note: "model display name"},
		"models.[].description":                   {outcome: googleOutcomeCanonical, note: "model description"},
		"models.[].version":                       {outcome: googleOutcomeCanonical, note: "planned model version/lineage metadata"},
		"models.[].inputTokenLimit":               {outcome: googleOutcomeCanonical, note: "input/context token limit"},
		"models.[].outputTokenLimit":              {outcome: googleOutcomeCanonical, note: "output token limit"},
		"models.[].supportedGenerationMethods.[]": {outcome: googleOutcomeCanonical, note: "generation/delivery capability"},
		"models.[].temperature":                   {outcome: googleOutcomeCanonical, note: "temperature default or support metadata"},
		"models.[].maxTemperature":                {outcome: googleOutcomeCanonical, note: "temperature range metadata"},
		"models.[].topP":                          {outcome: googleOutcomeCanonical, note: "top-p default or support metadata"},
		"models.[].topK":                          {outcome: googleOutcomeCanonical, note: "top-k default or support metadata"},
		"models.[].thinking":                      {outcome: googleOutcomeCanonical, note: "thinking/reasoning support"},
	}
}

func collectGoogleScalarPaths(value any) []string {
	seen := map[string]struct{}{}
	collectGoogleScalarPathsInto(value, nil, seen)

	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func collectGoogleScalarPathsInto(value any, path []string, seen map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			collectGoogleScalarPathsInto(child, append(path, key), seen)
		}
	case []any:
		for _, child := range typed {
			collectGoogleScalarPathsInto(child, append(path, "[]"), seen)
		}
	default:
		if len(path) == 0 {
			return
		}
		seen[strings.Join(path, ".")] = struct{}{}
	}
}

const googleProviderShapeFixture = `{
  "models": [
    {
      "name": "models/gemini-3-pro",
      "displayName": "Gemini 3 Pro",
      "description": "Representative Google model.",
      "version": "003",
      "inputTokenLimit": 1048576,
      "outputTokenLimit": 65536,
      "supportedGenerationMethods": ["generateContent", "streamGenerateContent", "countTokens"],
      "temperature": 1.0,
      "maxTemperature": 2.0,
      "topP": 0.95,
      "topK": 40,
      "thinking": true
    }
  ]
}`
