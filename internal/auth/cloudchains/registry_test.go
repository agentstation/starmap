package cloudchains

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"strings"
	"testing"

	googleauth "cloud.google.com/go/auth"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/oracle/oci-go-sdk/v65/common"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestRegistryIsExhaustiveAndProviderInferred(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	for _, providerID := range []catalogs.ProviderID{
		catalogs.ProviderIDAmazonBedrock,
		catalogs.ProviderIDMicrosoftFoundry,
		catalogs.ProviderIDGoogleVertex,
		catalogs.ProviderIDOCI,
	} {
		provider := cloudProvider(providerID)
		if err := registry.ValidateProvider(&provider); err != nil {
			t.Fatalf("ValidateProvider(%s): %v", providerID, err)
		}
	}
	unsupported := cloudProvider("unsupported")
	if err := registry.ValidateProvider(&unsupported); err == nil {
		t.Fatal("unsupported provider validated cloud_chain")
	}
	cloudflare := cloudProvider(catalogs.ProviderIDCloudflare)
	if err := registry.ValidateProvider(&cloudflare); err == nil {
		t.Fatal("Cloudflare validated cloud_chain")
	}
}

func TestAdaptersReturnProviderTypedSecretSafeSessions(t *testing.T) {
	sentinel := stderrors.New("loader failed")
	if _, err := (awsAdapter{load: func(context.Context) (aws.Config, error) {
		return aws.Config{}, sentinel
	}}).Resolve(context.Background()); !stderrors.Is(err, sentinel) {
		t.Fatalf("AWS loader error = %v", err)
	}

	sessions := []any{
		AWSSession{config: aws.Config{}},
		AzureSession{},
		GoogleSession{credentials: &googleauth.Credentials{}},
		OCISession{provider: common.ConfigurationProvider(nil)},
	}
	for _, session := range sessions {
		encoded, err := json.Marshal(session)
		if err != nil {
			t.Fatalf("Marshal %T: %v", session, err)
		}
		if string(encoded) != "{}" || strings.Contains(string(encoded), "credential") {
			t.Fatalf("%T serialized SDK state: %s", session, encoded)
		}
	}
}

func TestGoogleAbsenceIsClassifiedAsUnavailable(t *testing.T) {
	_, err := (googleAdapter{load: func(context.Context) (*googleauth.Credentials, error) {
		return nil, errors.ErrNotFound
	}}).Resolve(context.Background())
	if !stderrors.Is(err, errors.ErrNotFound) {
		t.Fatalf("google absence = %v", err)
	}
}

func cloudProvider(id catalogs.ProviderID) catalogs.Provider {
	return catalogs.Provider{ID: id, Catalog: &catalogs.ProviderCatalog{Sources: []catalogs.ProviderSource{{
		ID: "models", Auth: catalogs.ProviderAuthPolicy{Methods: []catalogs.ProviderCredentialID{"cloud_chain"}},
	}}}}
}
