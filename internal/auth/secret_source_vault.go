package auth

import (
	"context"
	stderrors "errors"
	"net/http"

	vault "github.com/hashicorp/vault/api"
)

type vaultSecretRead = kvV2Read[vault.KVSecret]

type vaultSecretOpen func() (vaultSecretRead, func() error, error)

type vaultSource struct {
	open vaultSecretOpen
}

func newVaultSource() *vaultSource {
	return &vaultSource{open: func() (vaultSecretRead, func() error, error) {
		return openKVV2Read(func() (kvV2MountReader[vault.KVSecret], *http.Client, error) {
			config := vault.DefaultConfig()
			if config == nil || config.Error != nil {
				return nil, nil, errInvalidSecretObject
			}
			client, err := vault.NewClient(config)
			if err != nil {
				return nil, nil, err
			}
			return func(mount string) kvV2SecretReader[vault.KVSecret] {
				return client.KVv2(mount)
			}, config.HttpClient, nil
		})
	}}
}

func (*vaultSource) Backend() ReferenceBackend { return referenceBackendVault }

func (s *vaultSource) Resolve(
	ctx context.Context,
	reference Reference,
) (material sourceMaterial, err error) {
	mount, path, version, parseErr := kvV2SecretResource(reference)
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

func vaultSourceMaterial(
	backend ReferenceBackend,
	secret *vault.KVSecret,
	field string,
) (sourceMaterial, error) {
	if secret == nil || secret.Data == nil || secret.VersionMetadata == nil ||
		secret.VersionMetadata.Version < 1 {
		return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, backend)
	}
	return kvV2SourceMaterial(backend, secret.Data, secret.VersionMetadata.Version, field)
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
