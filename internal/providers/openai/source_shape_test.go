package openai

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

type providerPathDecision struct {
	outcome string
	note    string
}

const (
	providerOutcomeCanonical = "canonical"
	providerOutcomeExtension = "extension"
	providerOutcomeIgnored   = "ignored"
)

func TestOpenAICompatibleProviderShapePathsAreClassified(t *testing.T) {
	var response any
	if err := json.Unmarshal([]byte(openAICompatibleProviderShapeFixture), &response); err != nil {
		t.Fatalf("failed to parse provider shape fixture: %v", err)
	}

	paths := collectProviderScalarPaths(response)
	classified := openAICompatiblePathDecisions()

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
		case providerOutcomeCanonical, providerOutcomeExtension, providerOutcomeIgnored:
		default:
			t.Fatalf("path %q has invalid outcome %q", path, decision.outcome)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("unclassified OpenAI-compatible provider source paths:\n%s", strings.Join(missing, "\n"))
	}
}

func openAICompatiblePathDecisions() map[string]providerPathDecision {
	return map[string]providerPathDecision{
		"object": {outcome: providerOutcomeIgnored, note: "response envelope type; not model metadata"},

		"data.[].id":       {outcome: providerOutcomeCanonical, note: "model ID"},
		"data.[].object":   {outcome: providerOutcomeIgnored, note: "provider response object type"},
		"data.[].owned_by": {outcome: providerOutcomeCanonical, note: "model author/provider ownership"},
		"data.[].created":  {outcome: providerOutcomeCanonical, note: "provider-reported creation timestamp"},
		"data.[].root":     {outcome: providerOutcomeCanonical, note: "planned model lineage root"},
		"data.[].parent":   {outcome: providerOutcomeCanonical, note: "planned model lineage parent"},

		"data.[].active":                           {outcome: providerOutcomeCanonical, note: "provider availability signal"},
		"data.[].context_length":                   {outcome: providerOutcomeCanonical, note: "context window alias"},
		"data.[].context_window":                   {outcome: providerOutcomeCanonical, note: "context window"},
		"data.[].hugging_face_id":                  {outcome: providerOutcomeCanonical, note: "planned external model identifier"},
		"data.[].input_modalities.[]":              {outcome: providerOutcomeCanonical, note: "provider-reported input modalities"},
		"data.[].max_completion_tokens":            {outcome: providerOutcomeCanonical, note: "output token limit"},
		"data.[].max_output_length":                {outcome: providerOutcomeCanonical, note: "output token limit alias"},
		"data.[].name":                             {outcome: providerOutcomeCanonical, note: "provider display name"},
		"data.[].output_modalities.[]":             {outcome: providerOutcomeCanonical, note: "provider-reported output modalities"},
		"data.[].supported_features.[]":            {outcome: providerOutcomeCanonical, note: "provider-reported feature list"},
		"data.[].supported_sampling_parameters.[]": {outcome: providerOutcomeCanonical, note: "provider-reported generation controls"},

		"data.[].pricing.completion":       {outcome: providerOutcomeCanonical, note: "provider output token pricing"},
		"data.[].pricing.image":            {outcome: providerOutcomeCanonical, note: "provider image operation pricing"},
		"data.[].pricing.input_cache_read": {outcome: providerOutcomeCanonical, note: "provider cache read pricing"},
		"data.[].pricing.prompt":           {outcome: providerOutcomeCanonical, note: "provider input token pricing"},
		"data.[].pricing.request":          {outcome: providerOutcomeCanonical, note: "provider request pricing"},

		"data.[].metadata.context_length":            {outcome: providerOutcomeCanonical, note: "metadata context window"},
		"data.[].metadata.default_height":            {outcome: providerOutcomeCanonical, note: "planned media generation default"},
		"data.[].metadata.default_iterations":        {outcome: providerOutcomeCanonical, note: "planned media generation default"},
		"data.[].metadata.default_width":             {outcome: providerOutcomeCanonical, note: "planned media generation default"},
		"data.[].metadata.description":               {outcome: providerOutcomeCanonical, note: "metadata description"},
		"data.[].metadata.max_tokens":                {outcome: providerOutcomeCanonical, note: "metadata output token limit"},
		"data.[].metadata.pricing.cache_read_tokens": {outcome: providerOutcomeCanonical, note: "metadata cache read pricing"},
		"data.[].metadata.pricing.input_characters":  {outcome: providerOutcomeCanonical, note: "metadata character input pricing"},
		"data.[].metadata.pricing.input_seconds":     {outcome: providerOutcomeCanonical, note: "metadata audio/video input pricing"},
		"data.[].metadata.pricing.input_tokens":      {outcome: providerOutcomeCanonical, note: "metadata input token pricing"},
		"data.[].metadata.pricing.output_seconds":    {outcome: providerOutcomeCanonical, note: "metadata audio/video output pricing"},
		"data.[].metadata.pricing.output_tokens":     {outcome: providerOutcomeCanonical, note: "metadata output token pricing"},
		"data.[].metadata.pricing.per_image_unit":    {outcome: providerOutcomeCanonical, note: "metadata image operation pricing"},
		"data.[].metadata.tags.[]":                   {outcome: providerOutcomeCanonical, note: "metadata tags"},

		"data.[].kind":                 {outcome: providerOutcomeCanonical, note: "provider model kind/type"},
		"data.[].supports_chat":        {outcome: providerOutcomeCanonical, note: "chat support flag"},
		"data.[].supports_image_input": {outcome: providerOutcomeCanonical, note: "image input support flag"},
		"data.[].supports_tools":       {outcome: providerOutcomeCanonical, note: "tool support flag"},

		"data.[].permission.[].created":      {outcome: providerOutcomeExtension, note: "provider permission metadata"},
		"data.[].permission.[].group":        {outcome: providerOutcomeExtension, note: "provider permission metadata"},
		"data.[].permission.[].id":           {outcome: providerOutcomeExtension, note: "provider permission metadata"},
		"data.[].permission.[].object":       {outcome: providerOutcomeExtension, note: "provider permission metadata"},
		"data.[].permission.[].organization": {outcome: providerOutcomeExtension, note: "provider permission metadata"},
		"data.[].supports_image_in":          {outcome: providerOutcomeCanonical, note: "image input support flag"},
		"data.[].supports_reasoning":         {outcome: providerOutcomeCanonical, note: "reasoning support flag"},
		"data.[].supports_video_in":          {outcome: providerOutcomeCanonical, note: "video input support flag"},
	}
}

func collectProviderScalarPaths(value any) []string {
	seen := map[string]struct{}{}
	collectProviderScalarPathsInto(value, nil, seen)

	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func collectProviderScalarPathsInto(value any, path []string, seen map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			collectProviderScalarPathsInto(child, append(path, key), seen)
		}
	case []any:
		for _, child := range typed {
			collectProviderScalarPathsInto(child, append(path, "[]"), seen)
		}
	default:
		if len(path) == 0 {
			return
		}
		seen[strings.Join(path, ".")] = struct{}{}
	}
}

const openAICompatibleProviderShapeFixture = `{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "owned_by": "system",
      "created": 1715367049,
      "root": "gpt-4o",
      "parent": null
    },
    {
      "id": "llama-3.3-70b-versatile",
      "object": "model",
      "name": "Llama 3.3 70B Versatile",
      "owned_by": "Meta",
      "created": 1733447754,
      "active": true,
      "context_window": 131072,
      "context_length": 131072,
      "max_completion_tokens": 32768,
      "max_output_length": 32768,
      "hugging_face_id": "meta-llama/Llama-3.3-70B-Instruct",
      "input_modalities": ["text"],
      "output_modalities": ["text"],
      "supported_features": ["tools", "json_mode"],
      "supported_sampling_parameters": ["temperature", "top_p", "stop"],
      "pricing": {
        "request": 0,
        "prompt": 0.59,
        "completion": 0.79,
        "input_cache_read": 0.1,
        "image": 0.0
      }
    },
    {
      "id": "black-forest-labs/FLUX-1-schnell",
      "object": "model",
      "owned_by": "black-forest-labs",
      "created": 1700000000,
      "metadata": {
        "description": "Image generation model",
        "context_length": 4096,
        "max_tokens": 4096,
        "tags": ["image-generation", "text-to-image"],
        "default_width": 1024,
        "default_height": 1024,
        "default_iterations": 4,
        "pricing": {
          "input_tokens": 0.0,
          "output_tokens": 0.0,
          "cache_read_tokens": 0.0,
          "per_image_unit": 0.003,
          "input_characters": 0.001,
          "input_seconds": 0.002,
          "output_seconds": 0.004
        }
      }
    },
    {
      "id": "accounts/fireworks/models/llama-v3p1-405b-instruct",
      "object": "model",
      "owned_by": "fireworks",
      "created": 1722384000,
      "kind": "chat",
      "context_length": 131072,
      "supports_chat": true,
      "supports_tools": true,
      "supports_image_input": false
    },
    {
      "id": "kimi-latest",
      "object": "model",
      "owned_by": "Moonshot",
      "created": 1733447754,
      "root": "kimi-latest",
      "parent": null,
      "context_length": 128000,
      "supports_image_in": true,
      "supports_video_in": true,
      "supports_reasoning": true,
      "permission": [
        {
          "id": "modelperm-kimi",
          "object": "model_permission",
          "created": 1733447754,
          "organization": "*",
          "group": null
        }
      ]
    }
  ]
}`
