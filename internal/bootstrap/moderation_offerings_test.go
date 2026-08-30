package bootstrap

import (
	"slices"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

// TestShippedModerationOfferingsServeModerationsAlone reads the catalog this
// module embeds and holds every moderation offering to its one operation. A
// moderation model reads text and writes category scores, so an offering
// that also advertised chat completions would invite a request the provider
// rejects.
//
// The test also fails when the catalog ships no moderation offering at all.
// An operation the type system names and the data never carries is an
// operation no consumer can exercise.
func TestShippedModerationOfferingsServeModerationsAlone(t *testing.T) {
	builder, err := NewEmbeddedBuilder()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	shipped := 0
	for _, provider := range catalog.Providers().List() {
		offerings, err := catalog.ProviderOfferings(provider.ID)
		if err != nil {
			t.Fatalf("ProviderOfferings(%s): %v", provider.ID, err)
		}
		for _, offering := range offerings {
			if !slices.Contains(offering.Service.Operations, catalogs.ProviderOperationModerations) {
				continue
			}
			shipped++
			name := string(provider.ID) + "/" + string(offering.ProviderModelID)
			if slices.Contains(offering.Service.Operations, catalogs.ProviderOperationChatCompletions) {
				t.Errorf("%s serves moderations and still advertises chat completions", name)
			}
		}
	}
	if shipped == 0 {
		t.Fatal("the embedded catalog ships no moderation offering")
	}
}
