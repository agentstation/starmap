package auth

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

var azureSecretNamePattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,127}$`)

type azureSecretRead func(context.Context, string, string) (azsecrets.GetSecretResponse, error)

type azureSecretOpen func(string) (azureSecretRead, func() error, error)

type azureKeyVaultSource struct {
	open azureSecretOpen
}

func newAzureKeyVaultSource() *azureKeyVaultSource {
	return &azureKeyVaultSource{open: func(vaultURL string) (azureSecretRead, func() error, error) {
		httpClient, closeClient := ownedHTTPClient()
		credentialOptions := &azidentity.DefaultAzureCredentialOptions{}
		credentialOptions.Transport = httpClient
		credential, err := azidentity.NewDefaultAzureCredential(credentialOptions)
		if err != nil {
			_ = closeClient()
			return nil, nil, err
		}
		clientOptions := &azsecrets.ClientOptions{}
		clientOptions.Transport = httpClient
		client, err := azsecrets.NewClient(vaultURL, credential, clientOptions)
		if err != nil {
			_ = closeClient()
			return nil, nil, err
		}
		read := func(ctx context.Context, name, version string) (azsecrets.GetSecretResponse, error) {
			return client.GetSecret(ctx, name, version, nil)
		}
		return read, closeClient, nil
	}}
}

func (*azureKeyVaultSource) Backend() ReferenceBackend {
	return referenceBackendAzureVault
}

func (s *azureKeyVaultSource) Resolve(
	ctx context.Context,
	reference Reference,
) (material sourceMaterial, err error) {
	vaultURL, secretName, parseErr := azureSecretResource(reference.resource)
	if parseErr != nil || hasControlCharacter(reference.version) {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
	}
	read, closeClient, openErr := s.open(vaultURL)
	if openErr != nil {
		return sourceMaterial{}, sourceFailure(ctx, s.Backend(), openErr, func(error) SourceErrorKind {
			return SourceErrorUnavailable
		})
	}
	defer func() {
		if closeErr := closeClient(); closeErr != nil && err == nil {
			material = sourceMaterial{}
			err = newSourceError(SourceErrorUnavailable, s.Backend())
		}
	}()
	response, readErr := read(ctx, secretName, reference.version)
	if readErr != nil {
		return sourceMaterial{}, sourceFailure(ctx, s.Backend(), readErr, classifyAzureSecretError)
	}
	if response.Value == nil || response.ID == nil {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
	}
	return scalarSecretMaterial(
		s.Backend(), []byte(*response.Value), reference.field, response.ID.Version(),
	)
}

func azureSecretResource(resource string) (string, string, error) {
	parsed, err := url.Parse(resource)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", errInvalidSecretObject
	}
	parts := strings.Split(strings.TrimPrefix(parsed.EscapedPath(), "/"), "/")
	if len(parts) != 2 || parts[0] != "secrets" || parts[1] == "" {
		return "", "", errInvalidSecretObject
	}
	secretName, err := url.PathUnescape(parts[1])
	if err != nil || !azureSecretNamePattern.MatchString(secretName) {
		return "", "", errInvalidSecretObject
	}
	return parsed.Scheme + "://" + parsed.Host, secretName, nil
}

func classifyAzureSecretError(err error) SourceErrorKind {
	var responseErr *azcore.ResponseError
	if !stderrors.As(err, &responseErr) {
		return SourceErrorUnavailable
	}
	switch responseErr.StatusCode {
	case http.StatusNotFound:
		return SourceErrorNotConfigured
	case http.StatusUnauthorized, http.StatusForbidden:
		return SourceErrorDenied
	case http.StatusBadRequest, http.StatusConflict, http.StatusUnprocessableEntity:
		return SourceErrorInvalid
	default:
		return SourceErrorUnavailable
	}
}
