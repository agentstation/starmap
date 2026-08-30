package catalogs

import (
	"slices"
	"strings"
	"testing"
)

// TestModerationsIsAPublishableOperation holds the contract that lets a
// catalog state a provider classifies text against harm categories. Before
// this operation existed, a moderation model could only be misread as a chat
// model, so no consumer could route a moderation request to it.
func TestModerationsIsAPublishableOperation(t *testing.T) {
	t.Parallel()

	provider := Provider{
		ID:   ProviderIDOpenAI,
		Name: "OpenAI",
		Inference: &ProviderInference{
			BaseURL: "https://api.example",
			Endpoints: []ProviderInferenceEndpoint{
				{Operation: ProviderOperationChatCompletions, Type: EndpointTypeOpenAI, Path: "/v1/chat/completions"},
				{Operation: ProviderOperationModerations, Type: EndpointTypeOpenAI, Path: "/v1/moderations"},
			},
		},
	}
	if err := provider.ValidateContract(); err != nil {
		t.Fatalf("ValidateContract: %v", err)
	}

	endpoint, found := provider.Inference.Endpoint(ProviderOperationModerations)
	if !found {
		t.Fatal("moderations endpoint not found")
	}
	if got := provider.Inference.EndpointURL(endpoint, ""); got != "https://api.example/v1/moderations" {
		t.Fatalf("moderations URL = %q", got)
	}
}

// TestModerationsEndpointStillNeedsAPath proves the operation gained no
// exemption from the path rule. An endpoint that names the operation and no
// path would route a moderation request to the provider base URL.
func TestModerationsEndpointStillNeedsAPath(t *testing.T) {
	t.Parallel()

	provider := Provider{
		ID:   ProviderIDOpenAI,
		Name: "OpenAI",
		Inference: &ProviderInference{
			BaseURL:   "https://api.example",
			Endpoints: []ProviderInferenceEndpoint{{Operation: ProviderOperationModerations, Type: EndpointTypeOpenAI}},
		},
	}
	err := provider.ValidateContract()
	if err == nil {
		t.Fatal("ValidateContract accepted a moderations endpoint with no path")
	}
	if !strings.Contains(err.Error(), "path") {
		t.Fatalf("error = %v, want it to name the path", err)
	}
}

// TestOperationRefusalNamesModerations pins the refusal message. The
// operation list grew, so the message has to grow with it, or an operator
// reading the refusal cannot learn the operation exists.
func TestOperationRefusalNamesModerations(t *testing.T) {
	t.Parallel()

	provider := Provider{
		ID:   ProviderIDOpenAI,
		Name: "OpenAI",
		Inference: &ProviderInference{
			BaseURL: "https://api.example",
			Endpoints: []ProviderInferenceEndpoint{
				{Operation: ProviderOperation("moderation"), Type: EndpointTypeOpenAI, Path: "/v1/moderations"},
			},
		},
	}
	err := provider.ValidateContract()
	if err == nil {
		t.Fatal("ValidateContract accepted an operation no member names")
	}
	if !strings.Contains(err.Error(), string(ProviderOperationModerations)) {
		t.Fatalf("error = %v, want it to list %s", err, ProviderOperationModerations)
	}
}

// TestModerationModelServesModerationsAndNotChat mirrors the rerank guard. A
// moderation model reads text like a chat model does, so the tag is the only
// fact that separates the two. A caller that saw a moderation model in the
// chat list would send it a request it cannot answer.
func TestModerationModelServesModerationsAndNotChat(t *testing.T) {
	t.Parallel()

	moderation := textModel(nil, ModelTagModeration)
	got := deriveOfferingCapabilities(moderation).Operations
	if !slices.Contains(got, ProviderOperationModerations) {
		t.Fatalf("operations = %v, want the moderations operation", got)
	}
	if slices.Contains(got, ProviderOperationChatCompletions) {
		t.Fatalf("operations = %v, want no chat completions", got)
	}

	// The same modalities without the tag stay a chat model.
	chat := deriveOfferingCapabilities(textModel(nil)).Operations
	if !slices.Contains(chat, ProviderOperationChatCompletions) {
		t.Fatalf("untagged operations = %v, want chat completions", chat)
	}
	if slices.Contains(chat, ProviderOperationModerations) {
		t.Fatalf("untagged operations = %v, want no moderations", chat)
	}
}
