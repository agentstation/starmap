package auth

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

type kvV2SecretReader[T any] interface {
	Get(context.Context, string) (*T, error)
	GetVersion(context.Context, string, int) (*T, error)
}

type kvV2MountReader[T any] func(string) kvV2SecretReader[T]

type kvV2Read[T any] func(context.Context, string, string, int) (*T, error)

func openKVV2Read[T any](
	open func() (kvV2MountReader[T], *http.Client, error),
) (kvV2Read[T], func() error, error) {
	mountReader, httpClient, err := open()
	if err != nil {
		return nil, nil, err
	}
	read := func(ctx context.Context, mount, path string, version int) (*T, error) {
		reader := mountReader(mount)
		if version > 0 {
			return reader.GetVersion(ctx, path, version)
		}
		return reader.Get(ctx, path)
	}
	closeClient := func() error {
		if httpClient != nil {
			httpClient.CloseIdleConnections()
		}
		return nil
	}
	return read, closeClient, nil
}

func kvV2SecretResource(reference Reference) (string, string, int, error) {
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

func kvV2SourceMaterial(
	backend ReferenceBackend,
	data map[string]any,
	secretVersion int,
	field string,
) (sourceMaterial, error) {
	if data == nil || secretVersion < 1 {
		return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, backend)
	}
	version := strconv.Itoa(secretVersion)
	if field != "" {
		value, exists := data[field]
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
	if len(data) != 1 {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, backend)
	}
	for _, value := range data {
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
