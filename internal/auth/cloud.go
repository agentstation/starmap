package auth

import (
	"context"
	stderrors "errors"
	"time"

	cloudauth "cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azurepolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

const credentialRefreshSkew = 5 * time.Minute

type googleCredentials interface {
	Token(context.Context) (*cloudauth.Token, error)
	ProjectID(context.Context) (string, error)
	QuotaProjectID(context.Context) (string, error)
}

type googleCredentialDetector func([]string) (googleCredentials, error)

type googleDefaultChain struct {
	detect googleCredentialDetector
	now    func() time.Time
}

func defaultCloudChains() map[catalogs.ProviderAuthenticationPrimitive]cloudChain {
	return map[catalogs.ProviderAuthenticationPrimitive]cloudChain{
		catalogs.ProviderAuthenticationGoogleDefault: &googleDefaultChain{
			detect: func(scopes []string) (googleCredentials, error) {
				return credentials.DetectDefault(&credentials.DetectOptions{Scopes: scopes})
			},
			now: time.Now,
		},
		catalogs.ProviderAuthenticationAzureDefault: &azureDefaultChain{
			newCredential: func() (azureTokenCredential, error) {
				return azidentity.NewDefaultAzureCredential(nil)
			},
			now: time.Now,
		},
		catalogs.ProviderAuthenticationAWSDefault: &awsDefaultChain{
			load: func(ctx context.Context) (aws.Config, error) {
				return awsconfig.LoadDefaultConfig(ctx)
			},
			now: time.Now,
		},
	}
}

func (c *googleDefaultChain) resolve(
	ctx context.Context,
	profile catalogs.ProviderCredentialProfile,
	_ map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sourceMaterial, error) {
	if err := ctx.Err(); err != nil {
		return sourceMaterial{}, err
	}
	creds, err := c.detect(profile.Scopes)
	if err != nil {
		return sourceMaterial{}, newSourceError(
			SourceErrorNotConfigured,
			ReferenceBackend(catalogs.ProviderAuthenticationGoogleDefault),
		)
	}
	token, err := creds.Token(ctx)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return sourceMaterial{}, contextErr
		}
		return sourceMaterial{}, newSourceError(
			SourceErrorUnavailable,
			ReferenceBackend(catalogs.ProviderAuthenticationGoogleDefault),
		)
	}
	if token == nil || token.Value == "" {
		return sourceMaterial{}, newSourceError(
			SourceErrorNotConfigured,
			ReferenceBackend(catalogs.ProviderAuthenticationGoogleDefault),
		)
	}
	values := make(map[string]string)
	if fieldID := bearerTokenField(profile); fieldID != "" {
		values[string(fieldID)] = token.Value
	}
	if options := profile.ProtocolOptions.GoogleDefault; options != nil {
		if options.QuotaProjectField != "" {
			if project, projectErr := creds.QuotaProjectID(ctx); projectErr == nil && project != "" {
				values[string(options.QuotaProjectField)] = project
			}
		}
		if options.ProjectField != "" {
			if _, exists := values[string(options.ProjectField)]; !exists {
				if project, projectErr := creds.ProjectID(ctx); projectErr == nil && project != "" {
					values[string(options.ProjectField)] = project
				}
			}
		}
	}
	lease := renewableLease(token.Expiry, c.now())
	return sourceMaterial{
		values:    values,
		version:   token.Value + "\x00" + token.Expiry.UTC().Format(time.RFC3339Nano),
		expiresAt: token.Expiry,
		lease:     lease,
	}, nil
}

func bearerTokenField(profile catalogs.ProviderCredentialProfile) catalogs.ProviderCredentialFieldID {
	for _, placement := range profile.Placements {
		if placement.Kind == catalogs.ProviderCredentialPlacementHeader &&
			placement.Scheme == catalogs.ProviderCredentialSchemeBearer {
			return placement.Field
		}
	}
	return ""
}

func renewableLease(expiresAt, now time.Time) *sources.ProviderCredentialLease {
	if expiresAt.IsZero() {
		return &sources.ProviderCredentialLease{Renewable: true}
	}
	refreshAfter := expiresAt.Add(-credentialRefreshSkew)
	if refreshAfter.Before(now) {
		refreshAfter = now
	}
	return &sources.ProviderCredentialLease{
		Renewable: true, RefreshAfter: refreshAfter,
	}
}

type azureTokenCredential interface {
	GetToken(context.Context, azurepolicy.TokenRequestOptions) (azcore.AccessToken, error)
}

type azureDefaultChain struct {
	newCredential func() (azureTokenCredential, error)
	now           func() time.Time
}

func (c *azureDefaultChain) resolve(
	ctx context.Context,
	profile catalogs.ProviderCredentialProfile,
	_ map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sourceMaterial, error) {
	if err := ctx.Err(); err != nil {
		return sourceMaterial{}, err
	}
	credential, err := c.newCredential()
	if err != nil {
		return sourceMaterial{}, newSourceError(
			SourceErrorUnavailable,
			ReferenceBackend(catalogs.ProviderAuthenticationAzureDefault),
		)
	}
	token, err := credential.GetToken(ctx, azurepolicy.TokenRequestOptions{Scopes: profile.Scopes})
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return sourceMaterial{}, contextErr
		}
		var authenticationErr *azidentity.AuthenticationFailedError
		kind := SourceErrorNotConfigured
		if stderrors.As(err, &authenticationErr) {
			kind = SourceErrorDenied
		}
		return sourceMaterial{}, newSourceError(
			kind,
			ReferenceBackend(catalogs.ProviderAuthenticationAzureDefault),
		)
	}
	if token.Token == "" {
		return sourceMaterial{}, newSourceError(
			SourceErrorNotConfigured,
			ReferenceBackend(catalogs.ProviderAuthenticationAzureDefault),
		)
	}
	values := make(map[string]string)
	if fieldID := bearerTokenField(profile); fieldID != "" {
		values[string(fieldID)] = token.Token
	}
	return sourceMaterial{
		values:    values,
		version:   token.Token + "\x00" + token.ExpiresOn.UTC().Format(time.RFC3339Nano),
		expiresAt: token.ExpiresOn,
		lease:     renewableLease(token.ExpiresOn, c.now()),
	}, nil
}

type awsConfigLoader func(context.Context) (aws.Config, error)

type awsDefaultChain struct {
	load awsConfigLoader
	now  func() time.Time
}

func (c *awsDefaultChain) resolve(
	ctx context.Context,
	profile catalogs.ProviderCredentialProfile,
	_ map[catalogs.ProviderCredentialFieldID]catalogs.ProviderCredentialField,
) (sourceMaterial, error) {
	if err := ctx.Err(); err != nil {
		return sourceMaterial{}, err
	}
	config, err := c.load(ctx)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return sourceMaterial{}, contextErr
		}
		return sourceMaterial{}, newSourceError(
			SourceErrorUnavailable,
			ReferenceBackend(catalogs.ProviderAuthenticationAWSDefault),
		)
	}
	if config.Credentials == nil {
		return sourceMaterial{}, newSourceError(
			SourceErrorNotConfigured,
			ReferenceBackend(catalogs.ProviderAuthenticationAWSDefault),
		)
	}
	credentials, err := config.Credentials.Retrieve(ctx)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return sourceMaterial{}, contextErr
		}
		return sourceMaterial{}, newSourceError(
			SourceErrorUnavailable,
			ReferenceBackend(catalogs.ProviderAuthenticationAWSDefault),
		)
	}
	if !credentials.HasKeys() {
		return sourceMaterial{}, newSourceError(
			SourceErrorNotConfigured,
			ReferenceBackend(catalogs.ProviderAuthenticationAWSDefault),
		)
	}
	values := map[string]string{
		string(catalogs.ProviderAWSCredentialAccessKeyID):     credentials.AccessKeyID,
		string(catalogs.ProviderAWSCredentialSecretAccessKey): credentials.SecretAccessKey,
	}
	if credentials.SessionToken != "" {
		values[string(catalogs.ProviderAWSCredentialSessionToken)] = credentials.SessionToken
	}
	if options := profile.ProtocolOptions.AWSDefault; options != nil &&
		options.RegionField != "" && config.Region != "" {
		values[string(options.RegionField)] = config.Region
	}
	expiresAt := time.Time{}
	var lease *sources.ProviderCredentialLease
	if credentials.CanExpire {
		expiresAt = credentials.Expires
		lease = renewableLease(credentials.Expires, c.now())
	}
	return sourceMaterial{
		values: values,
		version: credentials.AccessKeyID + "\x00" + credentials.SecretAccessKey + "\x00" +
			credentials.SessionToken + "\x00" + expiresAt.UTC().Format(time.RFC3339Nano),
		expiresAt: expiresAt,
		lease:     lease,
	}, nil
}
