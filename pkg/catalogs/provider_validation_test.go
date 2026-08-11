package catalogs

import "testing"

func TestProviderContractValidationDoesNotReadCredentialEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	provider := credentialContractProvider()
	if err := provider.ValidateContract(); err != nil {
		t.Fatalf("ValidateContract without runtime credentials: %v", err)
	}
}
