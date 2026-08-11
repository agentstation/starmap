package auth

import (
	"context"
	"hash/crc32"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type gcpSecretRead func(context.Context, string) (*secretmanagerpb.AccessSecretVersionResponse, error)

type gcpSecretOpen func(context.Context) (gcpSecretRead, func() error, error)

type gcpSecretManagerSource struct {
	open gcpSecretOpen
}

func newGCPSecretManagerSource() *gcpSecretManagerSource {
	return &gcpSecretManagerSource{open: func(ctx context.Context) (gcpSecretRead, func() error, error) {
		client, err := secretmanager.NewClient(ctx)
		if err != nil {
			return nil, nil, err
		}
		read := func(
			readCtx context.Context,
			name string,
		) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			return client.AccessSecretVersion(readCtx, &secretmanagerpb.AccessSecretVersionRequest{Name: name})
		}
		return read, client.Close, nil
	}}
}

func (*gcpSecretManagerSource) Backend() ReferenceBackend {
	return referenceBackendGCPStore
}

func (s *gcpSecretManagerSource) Resolve(
	ctx context.Context,
	reference Reference,
) (material sourceMaterial, err error) {
	name, parseErr := gcpSecretVersionName(reference)
	if parseErr != nil {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
	}
	read, closeClient, openErr := s.open(ctx)
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
	response, readErr := read(ctx, name)
	if readErr != nil {
		return sourceMaterial{}, sourceFailure(ctx, s.Backend(), readErr, classifyGCPSecretError)
	}
	if response == nil || response.Payload == nil {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
	}
	payload := response.Payload.Data
	if checksum := response.Payload.DataCrc32C; checksum != nil {
		got := int64(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)))
		if got != *checksum {
			return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
		}
	}
	return scalarSecretMaterial(s.Backend(), payload, reference.field, response.Name)
}

func gcpSecretVersionName(reference Reference) (string, error) {
	parts := strings.Split(reference.resource, "/")
	valid := len(parts) == 4 && parts[0] == "projects" && parts[2] == "secrets"
	valid = valid || len(parts) == 6 && parts[0] == "projects" &&
		parts[2] == "locations" && parts[4] == "secrets"
	if !valid {
		return "", errInvalidSecretObject
	}
	for index, part := range parts {
		if part == "" || (index%2 == 1 && hasControlCharacter(part)) {
			return "", errInvalidSecretObject
		}
	}
	version := reference.version
	if version == "" {
		version = "latest"
	}
	if strings.Contains(version, "/") || hasControlCharacter(version) {
		return "", errInvalidSecretObject
	}
	return reference.resource + "/versions/" + version, nil
}

func classifyGCPSecretError(err error) SourceErrorKind {
	switch status.Code(err) {
	case codes.NotFound:
		return SourceErrorNotConfigured
	case codes.PermissionDenied, codes.Unauthenticated:
		return SourceErrorDenied
	case codes.InvalidArgument, codes.FailedPrecondition:
		return SourceErrorInvalid
	default:
		return SourceErrorUnavailable
	}
}
