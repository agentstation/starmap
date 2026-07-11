package modelsdev

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

type sourcePathDecision struct {
	outcome string
	note    string
}

const (
	outcomeCanonical = "canonical"
	outcomeExtension = "extension"
	outcomeIgnored   = "ignored"
)

func TestModelsDevSourceShapePathsAreClassified(t *testing.T) {
	var provider any
	if err := json.Unmarshal([]byte(modelsDevProviderShapeFixture), &provider); err != nil {
		t.Fatalf("failed to parse models.dev shape fixture: %v", err)
	}

	paths := collectScalarPaths(provider)
	classified := modelsDevPathDecisions()

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
		case outcomeCanonical, outcomeExtension, outcomeIgnored:
		default:
			t.Fatalf("path %q has invalid outcome %q", path, decision.outcome)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("unclassified models.dev source paths:\n%s", strings.Join(missing, "\n"))
	}
}

func modelsDevPathDecisions() map[string]sourcePathDecision {
	return map[string]sourcePathDecision{
		"api":    {outcome: outcomeCanonical, note: "provider API endpoint metadata"},
		"doc":    {outcome: outcomeCanonical, note: "provider catalog documentation URL"},
		"env.[]": {outcome: outcomeCanonical, note: "provider environment variable hints"},
		"id":     {outcome: outcomeCanonical, note: "provider ID"},
		"name":   {outcome: outcomeCanonical, note: "provider display name"},
		"npm":    {outcome: outcomeCanonical, note: "provider SDK/package metadata"},

		"models.{}.attachment":                     {outcome: outcomeCanonical, note: "attachment feature flag"},
		"models.{}.cost.cache":                     {outcome: outcomeCanonical, note: "legacy cache pricing"},
		"models.{}.cost.cache_read":                {outcome: outcomeCanonical, note: "cache read token pricing"},
		"models.{}.cost.cache_write":               {outcome: outcomeCanonical, note: "cache write token pricing"},
		"models.{}.cost.input":                     {outcome: outcomeCanonical, note: "input token pricing"},
		"models.{}.cost.input_audio":               {outcome: outcomeCanonical, note: "audio input operation pricing"},
		"models.{}.cost.output":                    {outcome: outcomeCanonical, note: "output token pricing"},
		"models.{}.cost.output_audio":              {outcome: outcomeCanonical, note: "audio output operation pricing"},
		"models.{}.cost.reasoning":                 {outcome: outcomeCanonical, note: "reasoning token pricing"},
		"models.{}.description":                    {outcome: outcomeCanonical, note: "model description"},
		"models.{}.family":                         {outcome: outcomeCanonical, note: "planned model family/lineage schema"},
		"models.{}.id":                             {outcome: outcomeCanonical, note: "model ID"},
		"models.{}.knowledge":                      {outcome: outcomeCanonical, note: "knowledge cutoff"},
		"models.{}.last_updated":                   {outcome: outcomeCanonical, note: "model update date"},
		"models.{}.limit.context":                  {outcome: outcomeCanonical, note: "context window"},
		"models.{}.limit.input":                    {outcome: outcomeCanonical, note: "planned input token limit"},
		"models.{}.limit.output":                   {outcome: outcomeCanonical, note: "output token limit"},
		"models.{}.modalities.input.[]":            {outcome: outcomeCanonical, note: "input modalities"},
		"models.{}.modalities.output.[]":           {outcome: outcomeCanonical, note: "output modalities"},
		"models.{}.name":                           {outcome: outcomeCanonical, note: "model display name"},
		"models.{}.open_weights":                   {outcome: outcomeCanonical, note: "open weights metadata"},
		"models.{}.reasoning":                      {outcome: outcomeCanonical, note: "reasoning support"},
		"models.{}.reasoning_options.[].max":       {outcome: outcomeCanonical, note: "planned numeric reasoning option maximum"},
		"models.{}.reasoning_options.[].min":       {outcome: outcomeCanonical, note: "planned numeric reasoning option minimum"},
		"models.{}.reasoning_options.[].type":      {outcome: outcomeCanonical, note: "reasoning option type"},
		"models.{}.reasoning_options.[].values.[]": {outcome: outcomeCanonical, note: "reasoning option values"},
		"models.{}.release_date":                   {outcome: outcomeCanonical, note: "release date"},
		"models.{}.status":                         {outcome: outcomeCanonical, note: "planned lifecycle/status schema"},
		"models.{}.structured_output":              {outcome: outcomeCanonical, note: "structured output support"},
		"models.{}.temperature":                    {outcome: outcomeCanonical, note: "temperature support"},
		"models.{}.tool_call":                      {outcome: outcomeCanonical, note: "tool call support"},

		"models.{}.cost.context_over_200k.cache_read":  {outcome: outcomeCanonical, note: "planned context-threshold cache read pricing"},
		"models.{}.cost.context_over_200k.cache_write": {outcome: outcomeCanonical, note: "planned context-threshold cache write pricing"},
		"models.{}.cost.context_over_200k.input":       {outcome: outcomeCanonical, note: "planned context-threshold input pricing"},
		"models.{}.cost.context_over_200k.output":      {outcome: outcomeCanonical, note: "planned context-threshold output pricing"},
		"models.{}.cost.tiers.[].cache_read":           {outcome: outcomeCanonical, note: "planned tiered cache read pricing"},
		"models.{}.cost.tiers.[].cache_write":          {outcome: outcomeCanonical, note: "planned tiered cache write pricing"},
		"models.{}.cost.tiers.[].input":                {outcome: outcomeCanonical, note: "planned tiered input pricing"},
		"models.{}.cost.tiers.[].input_audio":          {outcome: outcomeCanonical, note: "planned tiered audio input pricing"},
		"models.{}.cost.tiers.[].output":               {outcome: outcomeCanonical, note: "planned tiered output pricing"},
		"models.{}.cost.tiers.[].tier.size":            {outcome: outcomeCanonical, note: "planned pricing tier threshold"},
		"models.{}.cost.tiers.[].tier.type":            {outcome: outcomeCanonical, note: "planned pricing tier type"},

		"models.{}.experimental.modes.fast.cost.cache_read":                 {outcome: outcomeCanonical, note: "mode-specific cache read pricing"},
		"models.{}.experimental.modes.fast.cost.cache_write":                {outcome: outcomeCanonical, note: "mode-specific cache write pricing"},
		"models.{}.experimental.modes.fast.cost.input":                      {outcome: outcomeCanonical, note: "mode-specific input pricing"},
		"models.{}.experimental.modes.fast.cost.output":                     {outcome: outcomeCanonical, note: "mode-specific output pricing"},
		"models.{}.experimental.modes.fast.provider.body.service_tier":      {outcome: outcomeCanonical, note: "mode-specific request body override"},
		"models.{}.experimental.modes.fast.provider.body.speed":             {outcome: outcomeCanonical, note: "mode-specific request body override"},
		"models.{}.experimental.modes.fast.provider.headers.anthropic-beta": {outcome: outcomeCanonical, note: "mode-specific request header override"},
		"models.{}.interleaved":                                             {outcome: outcomeExtension, note: "source-specific interleaved reasoning response metadata"},
		"models.{}.interleaved.field":                                       {outcome: outcomeExtension, note: "source-specific interleaved reasoning response field"},
		"models.{}.provider.api":                                            {outcome: outcomeExtension, note: "model-level provider API override"},
		"models.{}.provider.npm":                                            {outcome: outcomeExtension, note: "model-level SDK/package override"},
		"models.{}.provider.shape":                                          {outcome: outcomeExtension, note: "model-level API shape override"},
	}
}

func collectScalarPaths(value any) []string {
	seen := map[string]struct{}{}
	collectScalarPathsInto(value, nil, seen)

	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func collectScalarPathsInto(value any, path []string, seen map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			collectScalarPathsInto(child, append(path, key), seen)
		}
	case []any:
		for _, child := range typed {
			collectScalarPathsInto(child, append(path, "[]"), seen)
		}
	default:
		if len(path) == 0 {
			return
		}
		seen[normalizeModelsDevPath(path)] = struct{}{}
	}
}

func normalizeModelsDevPath(path []string) string {
	normalized := append([]string(nil), path...)
	for i := 0; i < len(normalized)-1; i++ {
		if normalized[i] == "models" {
			normalized[i+1] = "{}"
			i++
		}
	}
	return strings.Join(normalized, ".")
}

const modelsDevProviderShapeFixture = `{
  "id": "example-provider",
  "env": ["EXAMPLE_API_KEY"],
  "npm": "@ai-sdk/openai-compatible",
  "api": "https://example.test/v1",
  "name": "Example Provider",
  "doc": "https://example.test/docs",
  "models": {
    "example-model": {
      "id": "example-model",
      "name": "Example Model",
      "description": "Representative models.dev source shape.",
      "family": "example",
      "attachment": true,
      "reasoning": true,
      "reasoning_options": [
        {"type": "effort", "values": ["minimal", "low", "medium", "high", "max"]},
        {"type": "budget_tokens", "min": 0, "max": 8192},
        {"type": "toggle", "values": ["none", "default"]}
      ],
      "structured_output": true,
      "temperature": true,
      "tool_call": true,
      "knowledge": "2026-01",
      "release_date": "2026-02-03",
      "last_updated": "2026-02-04",
      "modalities": {
        "input": ["text", "image", "audio", "video", "pdf", "embedding"],
        "output": ["text", "audio", "video", "image"]
      },
      "open_weights": true,
      "status": "beta",
      "provider": {
        "npm": "@ai-sdk/openai-compatible",
        "api": "https://example.test/v1",
        "shape": "completions"
      },
      "interleaved": {"field": "reasoning_content"},
      "experimental": {
        "modes": {
          "fast": {
            "cost": {
              "input": 5,
              "output": 30,
              "cache_read": 0.5,
              "cache_write": 2.5
            },
            "provider": {
              "body": {
                "service_tier": "priority",
                "speed": "fast"
              },
              "headers": {
                "anthropic-beta": "fast-mode-2026-02-01"
              }
            }
          }
        }
      },
      "cost": {
        "input": 2.5,
        "output": 15,
        "reasoning": 3,
        "cache": 1.25,
        "cache_read": 0.25,
        "cache_write": 1.5,
        "input_audio": 0.003,
        "output_audio": 0.004,
        "tiers": [
          {
            "input": 5,
            "output": 22.5,
            "cache_read": 0.5,
            "cache_write": 2.5,
            "input_audio": 0.006,
            "tier": {
              "type": "context",
              "size": 272000
            }
          }
        ],
        "context_over_200k": {
          "input": 5,
          "output": 22.5,
          "cache_read": 0.5,
          "cache_write": 2.5
        }
      },
      "limit": {
        "context": 400000,
        "input": 272000,
        "output": 128000
      }
    },
    "interleaved-boolean-model": {
      "id": "interleaved-boolean-model",
      "name": "Interleaved Boolean Model",
      "description": "Covers the boolean interleaved source shape.",
      "release_date": "2026",
      "last_updated": "2026",
      "modalities": {
        "input": ["text"],
        "output": ["text"]
      },
      "open_weights": false,
      "reasoning": true,
      "structured_output": false,
      "temperature": true,
      "tool_call": true,
      "attachment": false,
      "interleaved": true,
      "limit": {
        "context": 128000,
        "output": 4096
      }
    }
  }
}`
