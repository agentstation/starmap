package productpaths

import (
	"context"
	stderrors "errors"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

// ConfigurationInput selects a primary file and its access policy before reading its contents.
type ConfigurationInput struct {
	// Path names the selected file. The host resolves its configured anchor before reading.
	Path         string
	AccessPolicy string
	Explicit     bool
	MaxBytes     int64
}

// ReadConfiguration reads a bounded primary file through the shared native access checks.
// Service-managed access requires an explicit path. The function never changes permissions.
func ReadConfiguration(ctx context.Context, input ConfigurationInput) ([]byte, error) {
	if err := policy.Require("configuration", policy.OwnerOnly); err != nil {
		return nil, err
	}
	access, err := policy.Configuration(input.AccessPolicy, input.Explicit)
	if err != nil {
		return nil, err
	}
	return readInput(ctx, input.Path, access, input.MaxBytes)
}

// ReadDotenv reads a bounded private dotenv file without the service configuration exception.
// The caller parses the returned bytes and selects their precedence.
func ReadDotenv(ctx context.Context, path string, maxBytes int64) ([]byte, error) {
	if err := policy.Require("dotenv", policy.OwnerOnly); err != nil {
		return nil, err
	}
	return readInput(ctx, path, policy.OwnerOnly, maxBytes)
}

func readInput(ctx context.Context, path, access string, maxBytes int64) ([]byte, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" || strings.ContainsRune(path, '\x00') {
		return nil, &errors.ValidationError{Field: "configuration.path", Message: "must be nonempty and contain no NUL"}
	}
	if maxBytes < 0 || maxBytes == math.MaxInt64 {
		return nil, &errors.ValidationError{Field: "configuration.max_bytes", Message: "must be a nonnegative bounded byte limit"}
	}
	if err := privatefiles.ValidateAncestors(path); err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if err := privatefiles.ValidateAncestors(resolved); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(filepath.Dir(resolved))
	if err != nil {
		return nil, err
	}
	reader := privatefiles.ReadFile
	if access == policy.ServiceManaged {
		reader = privatefiles.ReadServiceConfiguration
	}
	data, readErr := reader(root, filepath.Base(resolved), maxBytes)
	if err := stderrors.Join(readErr, root.Close(), ctx.Err()); err != nil {
		return nil, err
	}
	return data, nil
}
