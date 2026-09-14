package s3

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// The bound includes URL-encoded keys and XML metadata for one complete page.
const maxInventoryResponseBytes = 8 << 20

// List returns one bounded page through ListObjectsV2 without a delimiter.
// The service must return explicit completeness and matching namespace metadata.
func (b *Backend) List(ctx context.Context, request storage.ObjectListRequest) (storage.ObjectPage, error) {
	if err := request.Validate(); err != nil {
		return storage.ObjectPage{}, err
	}
	if err := ctx.Err(); err != nil {
		return storage.ObjectPage{}, err
	}
	limit := request.Limit
	if limit < 0 || limit > math.MaxInt32 {
		return storage.ObjectPage{}, &errors.ValidationError{Field: "object.list.limit", Message: "exceeds the SDK integer range"}
	}
	input := &awss3.ListObjectsV2Input{
		Bucket: aws.String(b.bucket), Prefix: aws.String(request.Prefix),
		MaxKeys: aws.Int32(int32(limit)), EncodingType: types.EncodingTypeUrl,
	}
	if request.Cursor != "" {
		input.ContinuationToken = aws.String(request.Cursor)
	}
	output, err := b.client.ListObjectsV2(ctx, input, func(options *awss3.Options) {
		options.HTTPClient = inventoryHTTPClient{client: options.HTTPClient}
	})
	if err != nil {
		return storage.ObjectPage{}, b.classifyError("list", request.Prefix, "", err)
	}
	return b.inventoryPage(request, output)
}

func (b *Backend) inventoryPage(request storage.ObjectListRequest, output *awss3.ListObjectsV2Output) (storage.ObjectPage, error) {
	if output == nil || output.IsTruncated == nil || output.KeyCount == nil {
		return storage.ObjectPage{}, invalidResponse("list", "completeness and object count are required")
	}
	prefix, err := url.PathUnescape(aws.ToString(output.Prefix))
	if err != nil || prefix != request.Prefix || aws.ToString(output.Name) != b.bucket || output.EncodingType != types.EncodingTypeUrl {
		return storage.ObjectPage{}, invalidResponse("list", "namespace or encoding differs from the request")
	}
	if len(output.CommonPrefixes) != 0 || len(output.Contents) > request.Limit || int64(*output.KeyCount) != int64(len(output.Contents)) {
		return storage.ObjectPage{}, invalidResponse("list", "object count or grouping violates the requested page")
	}
	next := aws.ToString(output.NextContinuationToken)
	if *output.IsTruncated {
		if strings.TrimSpace(next) == "" || next == request.Cursor {
			return storage.ObjectPage{}, invalidResponse("list", "an advancing continuation cursor is required")
		}
	} else if next != "" {
		return storage.ObjectPage{}, invalidResponse("list", "a complete page cannot contain a continuation cursor")
	}
	page := storage.ObjectPage{Next: next}
	seen := make(map[string]bool, len(output.Contents))
	for _, object := range output.Contents {
		entry, err := inventoryEntry(request.Prefix, object)
		if err != nil {
			return storage.ObjectPage{}, err
		}
		if seen[entry.Key] {
			return storage.ObjectPage{}, invalidResponse("list", "duplicate object key")
		}
		seen[entry.Key] = true
		page.Objects = append(page.Objects, entry)
	}
	return page, nil
}

func inventoryEntry(prefix string, object types.Object) (storage.ObjectEntry, error) {
	key, err := url.PathUnescape(aws.ToString(object.Key))
	if err != nil || !strings.HasPrefix(key, prefix) || object.Size == nil || *object.Size < 0 {
		return storage.ObjectEntry{}, invalidResponse("list", "object key or size is invalid")
	}
	version := aws.ToString(object.ETag)
	if err := exactETag(version); err != nil {
		return storage.ObjectEntry{}, err
	}
	return storage.ObjectEntry{Key: key, Version: version, Size: *object.Size}, nil
}

// Delete conditionally removes the current object through DeleteObject If-Match.
// It does not delete historical bucket versions or bypass object retention rules.
func (b *Backend) Delete(ctx context.Context, key, version string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if err := exactETag(version); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := b.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(b.bucket), Key: aws.String(key), IfMatch: aws.String(version),
	})
	return b.classifyError("delete", key, version, err)
}

func exactETag(version string) error {
	if len(version) < 2 || version[0] != '"' || version[len(version)-1] != '"' {
		return &errors.ValidationError{Field: "object.version", Message: "one quoted ETag is required"}
	}
	for _, char := range []byte(version[1 : len(version)-1]) {
		if char == '"' || char < 0x21 || char == 0x7f {
			return &errors.ValidationError{Field: "object.version", Message: "one quoted ETag is required"}
		}
	}
	return nil
}

type inventoryHTTPClient struct{ client awss3.HTTPClient }

func (c inventoryHTTPClient) Do(request *http.Request) (*http.Response, error) {
	response, err := c.client.Do(request)
	if err == nil && response != nil && response.Body != nil {
		response.Body = &inventoryBody{reader: io.LimitReader(response.Body, maxInventoryResponseBytes), closer: response.Body}
	}
	return response, err
}

type inventoryBody struct {
	reader io.Reader
	closer io.Closer
}

func (b *inventoryBody) Read(data []byte) (int, error) { return b.reader.Read(data) }
func (b *inventoryBody) Close() error                  { return b.closer.Close() }

var _ storage.ObjectCollectionBackend = (*Backend)(nil)
