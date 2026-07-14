// Package cloudchains registers provider-inferred official SDK credential chains.
package cloudchains

import (
	"context"
	"fmt"
	"strings"

	googleauth "cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/oracle/oci-go-sdk/v65/common"

	starmapauth "github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	azureManagementScope = "https://management.azure.com/.default"
	googleCloudScope     = "https://www.googleapis.com/auth/cloud-platform"
)

// NewRegistry returns the exhaustive provider-to-official-SDK chain registry.
func NewRegistry() (*starmapauth.CloudChainRegistry, error) {
	return starmapauth.NewCloudChainRegistry(
		starmapauth.CloudChainRegistration{Provider: catalogs.ProviderIDAmazonBedrock, Adapter: awsAdapter{load: loadAWSDefault}},
		starmapauth.CloudChainRegistration{Provider: catalogs.ProviderIDMicrosoftFoundry, Adapter: azureAdapter{load: loadAzureDefault}},
		starmapauth.CloudChainRegistration{Provider: catalogs.ProviderIDGoogleVertex, Adapter: googleAdapter{load: loadGoogleDefault}},
		starmapauth.CloudChainRegistration{Provider: catalogs.ProviderIDOCI, Adapter: ociAdapter{load: loadOCIDefault}},
	)
}

// AWSSession carries request-scoped AWS configuration without serializable fields.
type AWSSession struct {
	config aws.Config
}

// ProviderID identifies the registered provider.
func (AWSSession) ProviderID() catalogs.ProviderID { return catalogs.ProviderIDAmazonBedrock }

// Config returns the SDK-native request-scoped configuration.
func (session AWSSession) Config() aws.Config { return session.config }

// AzureSession carries a request-scoped Azure credential without serializable fields.
type AzureSession struct {
	credential azcore.TokenCredential
}

// ProviderID identifies the registered provider.
func (AzureSession) ProviderID() catalogs.ProviderID { return catalogs.ProviderIDMicrosoftFoundry }

// Credential returns the SDK-native request-scoped credential.
func (session AzureSession) Credential() azcore.TokenCredential { return session.credential }

// GoogleSession carries request-scoped Google ADC without serializable fields.
type GoogleSession struct {
	credentials *googleauth.Credentials
}

// ProviderID identifies the registered provider.
func (GoogleSession) ProviderID() catalogs.ProviderID { return catalogs.ProviderIDGoogleVertex }

// Credentials returns the SDK-native request-scoped credentials.
func (session GoogleSession) Credentials() *googleauth.Credentials { return session.credentials }

// OCISession carries a request-scoped OCI configuration provider without serializable fields.
type OCISession struct {
	provider common.ConfigurationProvider
}

// ProviderID identifies the registered provider.
func (OCISession) ProviderID() catalogs.ProviderID { return catalogs.ProviderIDOCI }

// ConfigurationProvider returns the SDK-native request-scoped provider.
func (session OCISession) ConfigurationProvider() common.ConfigurationProvider {
	return session.provider
}

type awsLoader func(context.Context) (aws.Config, error)
type azureLoader func(context.Context) (azcore.TokenCredential, error)
type googleLoader func(context.Context) (*googleauth.Credentials, error)
type ociLoader func(context.Context) (common.ConfigurationProvider, error)

type awsAdapter struct{ load awsLoader }
type azureAdapter struct{ load azureLoader }
type googleAdapter struct{ load googleLoader }
type ociAdapter struct{ load ociLoader }

func (adapter awsAdapter) Resolve(ctx context.Context) (starmapauth.CloudChainSession, error) {
	configuration, err := adapter.load(ctx)
	if err != nil {
		return nil, err
	}
	return AWSSession{config: configuration}, nil
}

func (adapter azureAdapter) Resolve(ctx context.Context) (starmapauth.CloudChainSession, error) {
	credential, err := adapter.load(ctx)
	if err != nil {
		return nil, err
	}
	return AzureSession{credential: credential}, nil
}

func (adapter googleAdapter) Resolve(ctx context.Context) (starmapauth.CloudChainSession, error) {
	credential, err := adapter.load(ctx)
	if err != nil {
		return nil, err
	}
	return GoogleSession{credentials: credential}, nil
}

func (adapter ociAdapter) Resolve(ctx context.Context) (starmapauth.CloudChainSession, error) {
	provider, err := adapter.load(ctx)
	if err != nil {
		return nil, err
	}
	return OCISession{provider: provider}, nil
}

func loadAWSDefault(ctx context.Context) (aws.Config, error) {
	configuration, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, err
	}
	if configuration.Credentials == nil {
		return aws.Config{}, errors.ErrNotFound
	}
	if _, err := configuration.Credentials.Retrieve(ctx); err != nil {
		return aws.Config{}, err
	}
	return configuration, nil
}

func loadAzureDefault(ctx context.Context) (azcore.TokenCredential, error) {
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	if _, err := credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{azureManagementScope}}); err != nil {
		return nil, err
	}
	return credential, nil
}

func loadGoogleDefault(context.Context) (*googleauth.Credentials, error) {
	credential, err := credentials.DetectDefault(&credentials.DetectOptions{Scopes: []string{googleCloudScope}})
	if err != nil {
		if strings.Contains(err.Error(), "could not find default credentials") {
			return nil, fmt.Errorf("%w: google application default credentials", errors.ErrNotFound)
		}
		return nil, err
	}
	return credential, nil
}

func loadOCIDefault(ctx context.Context) (common.ConfigurationProvider, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	provider := common.DefaultConfigProvider()
	for _, required := range []func() (string, error){provider.TenancyOCID, provider.UserOCID, provider.KeyFingerprint, provider.Region} {
		value, err := required()
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(value) == "" {
			return nil, errors.ErrNotFound
		}
	}
	return provider, nil
}
