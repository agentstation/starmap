package auth

import (
	"context"
	stderrors "errors"
	"net/http"
	"strconv"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

type vaultSecretRead func(context.Context, string, string, int) (*vault.KVSecret, error)

type vaultSecretOpen func() (vaultSecretRead, func() error, error)

type vaultSource struct {
	open vaultSecretOpen
}

func newVaultSource() *vaultSource {
	return &vaultSource{open: func() (vaultSecretRead, func() error, error) {
		config := vault.DefaultConfig()
		if config == nil || config.Error != nil {
			return nil, nil, errInvalidSecretObject
		}
		client, err := vault.NewClient(config)
		if err != nil {
			return nil, nil, err
		}
		closeClient := func() error {
			if config.HttpClient != nil {
				config.HttpClient.CloseIdleConnections()
			}
			return nil
		}
		read := func(ctx context.Context, mount, path string, version int) (*vault.KVSecret, error) {
			kv := client.KVv2(mount)
			if version > 0 {
				return kv.GetVersion(ctx, path, version)
			}
			return kv.Get(ctx, path)
		}
		return read, closeClient, nil
	}}
}

func (*vaultSource) Backend() ReferenceBackend { return referenceBackendVault }

func (s *vaultSource) Resolve(
	ctx context.Context,
	reference Reference,
) (material sourceMaterial, err error) {
	mount, path, version, parseErr := vaultSecretResource(reference)
	if parseErr != nil {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
	}
	read, closeClient, openErr := s.open()
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
	secret, readErr := read(ctx, mount, path, version)
	if readErr != nil {
		return sourceMaterial{}, sourceFailure(ctx, s.Backend(), readErr, classifyVaultSecretError)
	}
	return vaultSourceMaterial(s.Backend(), secret, reference.field)
}

func vaultSecretResource(reference Reference) (string, string, int, error) {
	mount, path, found := strings.Cut(reference.resource, "/")
	if !found || mount == "" || path == "" || strings.Contains(mount, "/") ||
		hasControlCharacter(mount) || hasControlCharacter(path) {
		return "", "", 0, errInvalidSecretObject
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", "", 0, errInvalidSecretObject
		}
	}
	version := 0
	if reference.version != "" {
		parsed, err := strconv.Atoi(reference.version)
		if err != nil || parsed < 1 {
			return "", "", 0, errInvalidSecretObject
		}
		version = parsed
	}
	return mount, path, version, nil
}

func vaultSourceMaterial(
	backend ReferenceBackend,
	secret *vault.KVSecret,
	field string,
) (sourceMaterial, error) {
	if secret == nil || secret.Data == nil || secret.VersionMetadata == nil ||
		secret.VersionMetadata.Version < 1 {
		return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, backend)
	}
	version := strconv.Itoa(secret.VersionMetadata.Version)
	if field != "" {
		value, exists := secret.Data[field]
		if !exists || value == nil {
			return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, backend)
		}
		stringValue, ok := value.(string)
		if !ok {
			return sourceMaterial{}, newSourceError(SourceErrorInvalid, backend)
		}
		if stringValue == "" {
			return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, backend)
		}
		return sourceMaterial{
			values: map[string]string{field: stringValue}, version: version,
		}, nil
	}
	if len(secret.Data) != 1 {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, backend)
	}
	for _, value := range secret.Data {
		stringValue, ok := value.(string)
		if !ok {
			return sourceMaterial{}, newSourceError(SourceErrorInvalid, backend)
		}
		if stringValue == "" {
			return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, backend)
		}
		return sourceMaterial{
			values: map[string]string{"value": stringValue}, version: version,
		}, nil
	}
	return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, backend)
}

func classifyVaultSecretError(err error) SourceErrorKind {
	var responseErr *vault.ResponseError
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
