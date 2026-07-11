package anthropic

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

type anthropicPathDecision struct {
	outcome string
	note    string
}

const (
	anthropicOutcomeCanonical = "canonical"
	anthropicOutcomeExtension = "extension"
	anthropicOutcomeIgnored   = "ignored"
)

func TestAnthropicProviderShapePathsAreClassified(t *testing.T) {
	var response any
	if err := json.Unmarshal([]byte(anthropicProviderShapeFixture), &response); err != nil {
		t.Fatalf("failed to parse Anthropic shape fixture: %v", err)
	}

	paths := collectAnthropicScalarPaths(response)
	classified := anthropicPathDecisions()

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
		case anthropicOutcomeCanonical, anthropicOutcomeExtension, anthropicOutcomeIgnored:
		default:
			t.Fatalf("path %q has invalid outcome %q", path, decision.outcome)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("unclassified Anthropic source paths:\n%s", strings.Join(missing, "\n"))
	}
}

func anthropicPathDecisions() map[string]anthropicPathDecision {
	return map[string]anthropicPathDecision{
		"data.[].type":         {outcome: anthropicOutcomeIgnored, note: "provider response object type"},
		"data.[].id":           {outcome: anthropicOutcomeCanonical, note: "model ID"},
		"data.[].display_name": {outcome: anthropicOutcomeCanonical, note: "model display name"},
		"data.[].created_at":   {outcome: anthropicOutcomeCanonical, note: "provider creation timestamp"},

		"data.[].max_input_tokens": {outcome: anthropicOutcomeCanonical, note: "planned input/context token limit"},
		"data.[].max_tokens":       {outcome: anthropicOutcomeCanonical, note: "output token limit"},

		"data.[].capabilities.batch.supported":                                       {outcome: anthropicOutcomeCanonical, note: "batch support"},
		"data.[].capabilities.citations.supported":                                   {outcome: anthropicOutcomeCanonical, note: "citations support"},
		"data.[].capabilities.code_execution.supported":                              {outcome: anthropicOutcomeCanonical, note: "code execution support"},
		"data.[].capabilities.context_management.supported":                          {outcome: anthropicOutcomeCanonical, note: "context management support"},
		"data.[].capabilities.context_management.clear_thinking_20251015.supported":  {outcome: anthropicOutcomeCanonical, note: "context-management feature variant"},
		"data.[].capabilities.context_management.clear_tool_uses_20250919.supported": {outcome: anthropicOutcomeCanonical, note: "context-management feature variant"},
		"data.[].capabilities.context_management.compact_20260112.supported":         {outcome: anthropicOutcomeCanonical, note: "context-management feature variant"},
		"data.[].capabilities.effort.high.supported":                                 {outcome: anthropicOutcomeCanonical, note: "reasoning effort level"},
		"data.[].capabilities.effort.low.supported":                                  {outcome: anthropicOutcomeCanonical, note: "reasoning effort level"},
		"data.[].capabilities.effort.max.supported":                                  {outcome: anthropicOutcomeCanonical, note: "reasoning effort level"},
		"data.[].capabilities.effort.medium.supported":                               {outcome: anthropicOutcomeCanonical, note: "reasoning effort level"},
		"data.[].capabilities.effort.supported":                                      {outcome: anthropicOutcomeCanonical, note: "reasoning effort support"},
		"data.[].capabilities.image_input.supported":                                 {outcome: anthropicOutcomeCanonical, note: "image input support"},
		"data.[].capabilities.pdf_input.supported":                                   {outcome: anthropicOutcomeCanonical, note: "PDF input support"},
		"data.[].capabilities.structured_outputs.supported":                          {outcome: anthropicOutcomeCanonical, note: "structured output support"},
		"data.[].capabilities.thinking.supported":                                    {outcome: anthropicOutcomeCanonical, note: "thinking/reasoning support"},
		"data.[].capabilities.thinking.types.adaptive.supported":                     {outcome: anthropicOutcomeCanonical, note: "thinking type support"},
		"data.[].capabilities.thinking.types.enabled.supported":                      {outcome: anthropicOutcomeCanonical, note: "thinking type support"},
	}
}

func collectAnthropicScalarPaths(value any) []string {
	seen := map[string]struct{}{}
	collectAnthropicScalarPathsInto(value, nil, seen)

	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func collectAnthropicScalarPathsInto(value any, path []string, seen map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			collectAnthropicScalarPathsInto(child, append(path, key), seen)
		}
	case []any:
		for _, child := range typed {
			collectAnthropicScalarPathsInto(child, append(path, "[]"), seen)
		}
	default:
		if len(path) == 0 {
			return
		}
		seen[strings.Join(path, ".")] = struct{}{}
	}
}

const anthropicProviderShapeFixture = `{
  "data": [
    {
      "type": "model",
      "id": "claude-sonnet-4-5",
      "display_name": "Claude Sonnet 4.5",
      "created_at": "2026-01-15T00:00:00Z",
      "max_tokens": 64000,
      "max_input_tokens": 200000,
      "capabilities": {
        "batch": {"supported": true},
        "citations": {"supported": true},
        "code_execution": {"supported": true},
        "context_management": {
          "supported": true,
          "clear_tool_uses_20250919": {"supported": true},
          "clear_thinking_20251015": {"supported": true},
          "compact_20260112": {"supported": true}
        },
        "effort": {
          "supported": true,
          "low": {"supported": true},
          "medium": {"supported": true},
          "high": {"supported": true},
          "max": {"supported": true}
        },
        "image_input": {"supported": true},
        "pdf_input": {"supported": true},
        "structured_outputs": {"supported": true},
        "thinking": {
          "supported": true,
          "types": {
            "adaptive": {"supported": true},
            "enabled": {"supported": true}
          }
        }
      }
    }
  ]
}`
