package auth

import (
	"context"
	stderrors "errors"
	"net/http"

	openbao "github.com/openbao/openbao/api/v2"
)

type openBaoSecretRead = kvV2Read[openbao.KVSecret]

type openBaoSecretOpen func() (openBaoSecretRead, func() error, error)

type openBaoSource struct {
	open openBaoSecretOpen
}

func newOpenBaoSource() *openBaoSource {
	return &openBaoSource{open: func() (openBaoSecretRead, func() error, error) {
		return openKVV2Read(func() (kvV2MountReader[openbao.KVSecret], *http.Client, error) {
			config := openbao.DefaultConfig()
			if config == nil || config.Error != nil {
				return nil, nil, errInvalidSecretObject
			}
			client, err := openbao.NewClient(config)
			if err != nil {
				return nil, nil, err
			}
			return func(mount string) kvV2SecretReader[openbao.KVSecret] {
				return client.KVv2(mount)
			}, config.HttpClient, nil
		})
	}}
}

func (*openBaoSource) Backend() ReferenceBackend { return referenceBackendOpenBao }

func (s *openBaoSource) Resolve(
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
		return sourceMaterial{}, sourceFailure(ctx, s.Backend(), readErr, classifyOpenBaoSecretError)
	}
	if secret == nil || secret.VersionMetadata == nil {
		return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, s.Backend())
	}
	return kvV2SourceMaterial(
		s.Backend(), secret.Data, secret.VersionMetadata.Version, reference.field,
	)
}

func classifyOpenBaoSecretError(err error) SourceErrorKind {
	if stderrors.Is(err, openbao.ErrSecretNotFound) {
		return SourceErrorNotConfigured
	}
	var responseErr *openbao.ResponseError
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
