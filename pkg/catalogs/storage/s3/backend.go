// Package s3 provides an S3-compatible implementation of
// storage.ObjectBackend.
//
// The caller owns the S3 client, including endpoint selection, credentials,
// retries, transport, and lifecycle. Backend construction performs no network
// operation. The selected service must implement conditional PutObject writes:
// If-None-Match for immutable objects and If-Match for pointer promotion.
package s3

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// Config describes one caller-owned S3 bucket.
type Config struct {
	// Bucket is the S3-compatible bucket that contains catalog objects.
	Bucket string
	// MaxObjectBytes bounds each object read and write. Zero uses
	// constants.MaxSourcePayloadBytes.
	MaxObjectBytes int64
}

// Backend adapts a caller-owned AWS SDK v2 S3 client to ObjectBackend.
//
// Backend does not close or otherwise manage Client. A Backend is safe to share
// when its Client is safe to share.
type Backend struct {
	client         *awss3.Client
	bucket         string
	maxObjectBytes int64
}

// New creates an inert S3-compatible object backend.
//
// The caller must configure client with its credentials, region, endpoint,
// transport, and retry policy before calling New. New performs no network work.
func New(client *awss3.Client, config Config) (*Backend, error) {
	if client == nil {
		return nil, &errors.ConfigError{Component: "S3 catalog backend", Message: "client is required"}
	}
	bucket := strings.TrimSpace(config.Bucket)
	if bucket == "" {
		return nil, &errors.ConfigError{Component: "S3 catalog backend", Message: "bucket is required"}
	}
	maxObjectBytes := config.MaxObjectBytes
	if maxObjectBytes == 0 {
		maxObjectBytes = constants.MaxSourcePayloadBytes
	}
	if maxObjectBytes < 0 {
		return nil, &errors.ConfigError{Component: "S3 catalog backend", Message: "maximum object bytes must be positive"}
	}
	return &Backend{
		client:         client,
		bucket:         bucket,
		maxObjectBytes: maxObjectBytes,
	}, nil
}

// Get fetches one object and returns its opaque ETag as Version.
func (b *Backend) Get(ctx context.Context, key string) (storage.ObjectValue, error) {
	if err := validateKey(key); err != nil {
		return storage.ObjectValue{}, err
	}
	if err := ctx.Err(); err != nil {
		return storage.ObjectValue{}, err
	}
	output, err := b.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return storage.ObjectValue{}, b.classifyError("fetch", key, "", err)
	}
	if output == nil || output.Body == nil {
		return storage.ObjectValue{}, invalidResponse("get", "response body is required")
	}
	version, err := requireETag(output.ETag, "get")
	if err != nil {
		_ = output.Body.Close()
		return storage.ObjectValue{}, err
	}
	if output.ContentLength != nil && *output.ContentLength > b.maxObjectBytes {
		_ = output.Body.Close()
		return storage.ObjectValue{}, objectTooLarge(*output.ContentLength, b.maxObjectBytes)
	}
	data, readErr := io.ReadAll(io.LimitReader(output.Body, b.maxObjectBytes+1))
	closeErr := output.Body.Close()
	if readErr != nil {
		return storage.ObjectValue{}, resourceError("read", key, readErr)
	}
	if closeErr != nil {
		return storage.ObjectValue{}, resourceError("close", key, closeErr)
	}
	if int64(len(data)) > b.maxObjectBytes {
		return storage.ObjectValue{}, objectTooLarge(int64(len(data)), b.maxObjectBytes)
	}
	return storage.ObjectValue{Data: data, Version: version}, nil
}

// Put conditionally writes one object and returns its opaque ETag as Version.
//
// Exactly one condition is required. Backend never performs an unconditional
// last-writer-wins write.
func (b *Backend) Put(
	ctx context.Context,
	key string,
	data []byte,
	condition storage.ObjectPutCondition,
) (storage.ObjectValue, error) {
	if err := validateKey(key); err != nil {
		return storage.ObjectValue{}, err
	}
	if err := validateCondition(condition); err != nil {
		return storage.ObjectValue{}, err
	}
	if int64(len(data)) > b.maxObjectBytes {
		return storage.ObjectValue{}, objectTooLarge(int64(len(data)), b.maxObjectBytes)
	}
	if err := ctx.Err(); err != nil {
		return storage.ObjectValue{}, err
	}

	input := &awss3.PutObjectInput{
		Body:          bytes.NewReader(data),
		Bucket:        aws.String(b.bucket),
		ContentLength: aws.Int64(int64(len(data))),
		Key:           aws.String(key),
	}
	expected := ""
	if condition.IfAbsent {
		input.IfNoneMatch = aws.String("*")
	} else {
		expected = condition.IfVersion
		input.IfMatch = aws.String(condition.IfVersion)
	}
	output, err := b.client.PutObject(ctx, input)
	if err != nil {
		return storage.ObjectValue{}, b.classifyError("write", key, expected, err)
	}
	if output == nil {
		return storage.ObjectValue{}, invalidResponse("put", "response is required")
	}
	version, err := requireETag(output.ETag, "put")
	if err != nil {
		return storage.ObjectValue{}, err
	}
	return storage.ObjectValue{
		Data:    append([]byte(nil), data...),
		Version: version,
	}, nil
}

func validateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return &errors.ValidationError{Field: "object.key", Message: "is required"}
	}
	return nil
}

func validateCondition(condition storage.ObjectPutCondition) error {
	switch {
	case condition.IfAbsent && condition.IfVersion != "":
		return &errors.ValidationError{
			Field:   "object.condition",
			Message: "cannot combine IfAbsent and IfVersion",
		}
	case !condition.IfAbsent && strings.TrimSpace(condition.IfVersion) == "":
		return &errors.ValidationError{
			Field:   "object.condition",
			Message: "conditional write is required",
		}
	default:
		return nil
	}
}

func requireETag(value *string, operation string) (string, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "", invalidResponse(operation, "ETag is required for conditional publication")
	}
	return *value, nil
}

func objectTooLarge(actual, maximum int64) error {
	return &errors.ValidationError{
		Field:   "object.data",
		Value:   actual,
		Message: fmt.Sprintf("must not exceed %d bytes", maximum),
	}
}

func invalidResponse(operation, message string) error {
	return &errors.ValidationError{
		Field:   "s3." + operation + ".response",
		Message: message,
	}
}

func resourceError(operation, key string, err error) error {
	return &errors.ResourceError{
		Operation: operation,
		Resource:  "S3 object",
		ID:        key,
		Message:   "request failed",
		Err:       err,
	}
}

func (b *Backend) classifyError(operation, key, expected string, err error) error {
	if err == nil {
		return nil
	}
	var apiError smithy.APIError
	code := ""
	if stderrors.As(err, &apiError) {
		code = apiError.ErrorCode()
	}
	var responseError *awshttp.ResponseError
	status := 0
	if stderrors.As(err, &responseError) {
		status = responseError.HTTPStatusCode()
	}
	switch {
	case operation == "fetch" && (code == "NoSuchKey" || code == "NotFound" ||
		(status == 404 && code == "")):
		return &errors.NotFoundError{Resource: "S3 object", ID: key}
	case operation == "write" && (status == 409 || status == 412 ||
		code == "ConditionalRequestConflict" || code == "PreconditionFailed"):
		return &errors.ConflictError{
			Resource: "S3 object",
			Expected: expected,
			Message:  "conditional write was rejected",
		}
	default:
		return resourceError(operation, key, err)
	}
}

var _ storage.ObjectBackend = (*Backend)(nil)
