package auth

import (
	"context"
	stderrors "errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/smithy-go"
)

type awsSecretRead func(context.Context, *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error)

type awsSecretOpen func(context.Context) (awsSecretRead, func() error, error)

type awsSecretsManagerSource struct {
	open awsSecretOpen
}

func newAWSSecretsManagerSource() *awsSecretsManagerSource {
	return &awsSecretsManagerSource{open: func(ctx context.Context) (awsSecretRead, func() error, error) {
		httpClient, closeClient := ownedHTTPClient()
		config, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithHTTPClient(httpClient))
		if err != nil {
			_ = closeClient()
			return nil, nil, err
		}
		client := secretsmanager.NewFromConfig(config)
		read := func(
			readCtx context.Context,
			input *secretsmanager.GetSecretValueInput,
		) (*secretsmanager.GetSecretValueOutput, error) {
			return client.GetSecretValue(readCtx, input)
		}
		return read, closeClient, nil
	}}
}

func (*awsSecretsManagerSource) Backend() ReferenceBackend {
	return referenceBackendAWSStore
}

func (s *awsSecretsManagerSource) Resolve(
	ctx context.Context,
	reference Reference,
) (material sourceMaterial, err error) {
	if reference.resource == "" || hasControlCharacter(reference.resource) ||
		hasControlCharacter(reference.version) {
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
	input := &secretsmanager.GetSecretValueInput{SecretId: aws.String(reference.resource)}
	if reference.version != "" {
		input.VersionId = aws.String(reference.version)
	}
	response, readErr := read(ctx, input)
	if readErr != nil {
		return sourceMaterial{}, sourceFailure(ctx, s.Backend(), readErr, classifyAWSSecretError)
	}
	if response == nil || response.VersionId == nil || *response.VersionId == "" {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
	}
	var payload []byte
	switch {
	case response.SecretString != nil && len(response.SecretBinary) == 0:
		payload = []byte(*response.SecretString)
	case response.SecretString == nil && len(response.SecretBinary) > 0:
		payload = append([]byte(nil), response.SecretBinary...)
	default:
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
	}
	return scalarSecretMaterial(s.Backend(), payload, reference.field, *response.VersionId)
}

func classifyAWSSecretError(err error) SourceErrorKind {
	var apiErr smithy.APIError
	if !stderrors.As(err, &apiErr) {
		return SourceErrorUnavailable
	}
	switch apiErr.ErrorCode() {
	case "ResourceNotFoundException":
		return SourceErrorNotConfigured
	case "AccessDeniedException", "DecryptionFailure", "InvalidSignatureException",
		"UnrecognizedClientException", "UnrecognizedClient":
		return SourceErrorDenied
	case "InvalidParameterException", "InvalidRequestException":
		return SourceErrorInvalid
	default:
		if strings.Contains(apiErr.ErrorCode(), "AccessDenied") {
			return SourceErrorDenied
		}
		return SourceErrorUnavailable
	}
}
