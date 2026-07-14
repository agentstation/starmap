package notify

import (
	"reflect"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestDetectConfiguredProvidersUsesEmbeddedCredentialMetadata(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	for _, provider := range builder.Providers().List() {
		for _, credential := range provider.Credentials {
			for _, name := range credential.Env {
				t.Setenv(name, "")
			}
		}
	}

	// Mistral deliberately was not part of the old hard-coded hint list. Its
	// configured credential metadata is now the sole owner of this lookup.
	t.Setenv("MISTRAL_API_KEY", "configured-for-test")
	if got, want := detectConfiguredProviders(), []string{"mistral"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("configured providers = %#v, want %#v", got, want)
	}
}
