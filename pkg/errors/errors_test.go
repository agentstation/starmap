package errors_test

import (
	"errors"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	err := pkgerrors.New("test error")
	assert.NotNil(t, err)
	assert.Equal(t, "test error", err.Error())
}

func TestNotFoundError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := &pkgerrors.NotFoundError{
			Resource: "model",
			ID:       "gpt-4",
		}
		assert.Equal(t, "model with ID gpt-4 not found", err.Error())
		assert.True(t, errors.Is(err, pkgerrors.ErrNotFound))
	})

	t.Run("constructor", func(t *testing.T) {
		err := pkgerrors.NewNotFoundError("provider", "openai")
		assert.Equal(t, "provider with ID openai not found", err.Error())
		assert.True(t, pkgerrors.IsNotFound(err))
	})

	t.Run("wrapped error", func(t *testing.T) {
		base := pkgerrors.NewNotFoundError("model", "test")
		wrapped := errors.Join(errors.New("failed"), base)
		assert.True(t, pkgerrors.IsNotFound(wrapped))
	})
}

func TestValidationError(t *testing.T) {
	t.Run("with field", func(t *testing.T) {
		err := &pkgerrors.ValidationError{
			Field:   "api_key",
			Message: "cannot be empty",
		}
		assert.Equal(t, "validation failed for field api_key: cannot be empty", err.Error())
		assert.True(t, errors.Is(err, pkgerrors.ErrInvalidInput))
	})

	t.Run("without field", func(t *testing.T) {
		err := &pkgerrors.ValidationError{
			Message: "invalid configuration",
		}
		assert.Equal(t, "validation failed: invalid configuration", err.Error())
		assert.True(t, pkgerrors.IsValidationError(err))
	})

	t.Run("constructor", func(t *testing.T) {
		err := pkgerrors.NewValidationError("limit", 1000000, "exceeds maximum")
		assert.Contains(t, err.Error(), "limit")
		assert.Contains(t, err.Error(), "exceeds maximum")
	})
}

func TestAPIError(t *testing.T) {
	t.Run("with status code", func(t *testing.T) {
		err := &pkgerrors.APIError{
			Provider:   "openai",
			StatusCode: 429,
			Message:    "rate limit exceeded",
			Endpoint:   "https://api.openai.com/v1/models",
		}
		assert.Contains(t, err.Error(), "openai")
		assert.Contains(t, err.Error(), "429")
		assert.Contains(t, err.Error(), "rate limit exceeded")
	})

	t.Run("with wrapped error", func(t *testing.T) {
		baseErr := errors.New("connection timeout")
		err := &pkgerrors.APIError{
			Provider: "anthropic",
			Message:  "request failed",
			Err:      baseErr,
		}
		assert.Contains(t, err.Error(), "anthropic")
		assert.Contains(t, err.Error(), "request failed")
		assert.Equal(t, baseErr, err.Unwrap())
	})

	t.Run("constructor", func(t *testing.T) {
		err := pkgerrors.NewAPIError("groq", 500, "internal server error")
		assert.Contains(t, err.Error(), "groq")
		assert.Contains(t, err.Error(), "500")
	})
}

func TestConfigError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := &pkgerrors.ConfigError{
			Component: "provider",
			Message:   "api_key: invalid format",
		}
		assert.Contains(t, err.Error(), "provider")
		assert.Contains(t, err.Error(), "api_key")
		assert.Contains(t, err.Error(), "invalid format")
	})

	t.Run("constructor", func(t *testing.T) {
		err := pkgerrors.NewConfigError("database", "connection_string cannot be empty", nil)
		assert.Contains(t, err.Error(), "database")
		assert.Contains(t, err.Error(), "connection_string")
		assert.Contains(t, err.Error(), "cannot be empty")
	})
}

func TestIOError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := &pkgerrors.IOError{
			Operation: "read",
			Path:      "/tmp/test.json",
			Message:   "permission denied",
			Err:       errors.New("permission denied"),
		}
		assert.Contains(t, err.Error(), "read")
		assert.Contains(t, err.Error(), "/tmp/test.json")
		assert.Contains(t, err.Error(), "permission denied")
	})

	t.Run("unwrap", func(t *testing.T) {
		baseErr := errors.New("disk full")
		err := pkgerrors.NewIOError("write", "/data/output.txt", baseErr)
		assert.Equal(t, baseErr, err.Unwrap())
	})

	t.Run("wrap helper", func(t *testing.T) {
		baseErr := errors.New("network error")
		err := pkgerrors.WrapIO("download", "https://example.com/file", baseErr)
		ioErr, ok := err.(*pkgerrors.IOError)
		require.True(t, ok)
		assert.Equal(t, "download", ioErr.Operation)
		assert.Equal(t, "https://example.com/file", ioErr.Path)
	})
}

func TestResourceError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := &pkgerrors.ResourceError{
			Operation: "create",
			Resource:  "model",
			ID:        "claude-3",
			Message:   "already exists",
			Err:       pkgerrors.ErrAlreadyExists,
		}
		assert.Contains(t, err.Error(), "create")
		assert.Contains(t, err.Error(), "model")
		assert.Contains(t, err.Error(), "claude-3")
		assert.Contains(t, err.Error(), "already exists")
		assert.True(t, errors.Is(err, pkgerrors.ErrAlreadyExists))
	})

	t.Run("constructor", func(t *testing.T) {
		err := pkgerrors.NewResourceError("delete", "provider", "openai", errors.New("in use"))
		assert.Contains(t, err.Error(), "delete")
		assert.Contains(t, err.Error(), "provider")
		assert.Contains(t, err.Error(), "openai")
	})

	t.Run("wrap helper", func(t *testing.T) {
		err := pkgerrors.WrapResource("update", "endpoint", "https://api.example.com", errors.New("timeout"))
		resErr, ok := err.(*pkgerrors.ResourceError)
		require.True(t, ok)
		assert.Equal(t, "update", resErr.Operation)
		assert.Equal(t, "endpoint", resErr.Resource)
	})
}

func TestSyncError(t *testing.T) {
	t.Run("with models", func(t *testing.T) {
		err := &pkgerrors.SyncError{
			Provider: "anthropic",
			Models:   []string{"claude-3-opus", "claude-3-sonnet"},
			Err:      errors.New("API unavailable"),
		}
		assert.Contains(t, err.Error(), "anthropic")
		assert.Contains(t, err.Error(), "claude-3-opus")
		assert.Contains(t, err.Error(), "API unavailable")
	})

	t.Run("without models", func(t *testing.T) {
		err := pkgerrors.NewSyncError("groq", nil, errors.New("authentication failed"))
		assert.Contains(t, err.Error(), "groq")
		assert.Contains(t, err.Error(), "authentication failed")
		assert.NotContains(t, err.Error(), "affected models")
	})

	t.Run("unwrap", func(t *testing.T) {
		baseErr := errors.New("network error")
		err := &pkgerrors.SyncError{
			Provider: "openai",
			Err:      baseErr,
		}
		assert.Equal(t, baseErr, err.Unwrap())
	})
}

func TestParseError(t *testing.T) {
	t.Run("with file and position", func(t *testing.T) {
		err := &pkgerrors.ParseError{
			Format:  "json",
			File:    "config.json",
			Line:    10,
			Column:  5,
			Message: "unexpected token",
		}
		assert.Contains(t, err.Error(), "json")
		assert.Contains(t, err.Error(), "config.json")
		assert.Contains(t, err.Error(), "10:5")
		assert.Contains(t, err.Error(), "unexpected token")
	})

	t.Run("with file only", func(t *testing.T) {
		err := &pkgerrors.ParseError{
			Format:  "yaml",
			File:    "data.yaml",
			Message: "invalid indentation",
		}
		assert.Contains(t, err.Error(), "yaml")
		assert.Contains(t, err.Error(), "data.yaml")
		assert.Contains(t, err.Error(), "invalid indentation")
		// The error format is: "parse error in yaml file data.yaml: invalid indentation"
	})

	t.Run("format only", func(t *testing.T) {
		err := &pkgerrors.ParseError{
			Format:  "toml",
			Message: "syntax error",
		}
		assert.Contains(t, err.Error(), "toml parse error")
		assert.Contains(t, err.Error(), "syntax error")
	})

	t.Run("constructor and wrap", func(t *testing.T) {
		baseErr := errors.New("EOF")
		err := pkgerrors.NewParseError("xml", "document.xml", "unexpected end", baseErr)
		assert.Contains(t, err.Error(), "xml")
		assert.Equal(t, baseErr, err.Unwrap())
		
		wrapped := pkgerrors.WrapParse("csv", "data.csv", baseErr)
		parseErr, ok := wrapped.(*pkgerrors.ParseError)
		require.True(t, ok)
		assert.Equal(t, "csv", parseErr.Format)
		assert.Equal(t, "data.csv", parseErr.File)
	})
}

func TestAuthenticationError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := &pkgerrors.AuthenticationError{
			Provider: "openai",
			Method:   "api_key",
			Message:  "invalid API key format",
		}
		assert.Contains(t, err.Error(), "openai")
		assert.Contains(t, err.Error(), "api_key")
		assert.Contains(t, err.Error(), "invalid API key format")
		assert.True(t, errors.Is(err, pkgerrors.ErrAPIKeyInvalid))
	})

	t.Run("with wrapped error", func(t *testing.T) {
		baseErr := errors.New("token expired")
		err := pkgerrors.NewAuthenticationError("google", "oauth", "authentication failed", baseErr)
		assert.Contains(t, err.Error(), "google")
		assert.Contains(t, err.Error(), "oauth")
		assert.Equal(t, baseErr, err.Unwrap())
	})

	t.Run("is API key error", func(t *testing.T) {
		err := &pkgerrors.AuthenticationError{
			Provider: "anthropic",
			Method:   "api_key",
			Message:  "missing",
		}
		assert.True(t, pkgerrors.IsAPIKeyError(err))
	})
}

func TestTimeoutError(t *testing.T) {
	t.Run("with duration", func(t *testing.T) {
		err := &pkgerrors.TimeoutError{
			Operation: "fetch models",
			Duration:  "30s",
			Message:   "provider not responding",
		}
		assert.Contains(t, err.Error(), "fetch models")
		assert.Contains(t, err.Error(), "30s")
		assert.Contains(t, err.Error(), "provider not responding")
		assert.True(t, errors.Is(err, pkgerrors.ErrTimeout))
	})

	t.Run("without duration", func(t *testing.T) {
		err := pkgerrors.NewTimeoutError("upload file", "", "connection lost")
		assert.Contains(t, err.Error(), "upload file")
		assert.Contains(t, err.Error(), "connection lost")
		assert.NotContains(t, err.Error(), "after")
	})

	t.Run("is timeout", func(t *testing.T) {
		err := &pkgerrors.TimeoutError{
			Operation: "sync",
		}
		assert.True(t, pkgerrors.IsTimeout(err))
	})
}

func TestProcessError(t *testing.T) {
	t.Run("with output", func(t *testing.T) {
		err := &pkgerrors.ProcessError{
			Operation: "build api.json",
			Command:   "npm run build",
			Output:    "Error: Module not found",
			ExitCode:  1,
			Err:       errors.New("exit status 1"),
		}
		assert.Contains(t, err.Error(), "build api.json")
		assert.Contains(t, err.Error(), "npm run build")
		assert.Contains(t, err.Error(), "Module not found")
	})

	t.Run("without output", func(t *testing.T) {
		err := pkgerrors.NewProcessError("git clone", "git clone https://...", "", errors.New("signal: killed"))
		assert.Contains(t, err.Error(), "git clone")
		assert.Contains(t, err.Error(), "signal: killed")
		assert.NotContains(t, err.Error(), "Output:")
	})

	t.Run("unwrap", func(t *testing.T) {
		baseErr := errors.New("command not found")
		err := &pkgerrors.ProcessError{
			Operation: "install",
			Command:   "make install",
			Err:       baseErr,
		}
		assert.Equal(t, baseErr, err.Unwrap())
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("IsNotFound", func(t *testing.T) {
		err1 := pkgerrors.NewNotFoundError("model", "test")
		err2 := errors.New("not found")
		err3 := pkgerrors.ErrNotFound

		assert.True(t, pkgerrors.IsNotFound(err1))
		assert.False(t, pkgerrors.IsNotFound(err2))
		assert.True(t, pkgerrors.IsNotFound(err3))
	})

	t.Run("IsAlreadyExists", func(t *testing.T) {
		err1 := &pkgerrors.ResourceError{Err: pkgerrors.ErrAlreadyExists}
		err2 := pkgerrors.ErrAlreadyExists

		assert.True(t, pkgerrors.IsAlreadyExists(err1))
		assert.True(t, pkgerrors.IsAlreadyExists(err2))
	})

	t.Run("IsRateLimited", func(t *testing.T) {
		err := pkgerrors.ErrRateLimited
		assert.True(t, pkgerrors.IsRateLimited(err))
	})

	t.Run("IsCanceled", func(t *testing.T) {
		err := pkgerrors.ErrCanceled
		assert.True(t, pkgerrors.IsCanceled(err))
	})

	t.Run("IsProviderUnavailable", func(t *testing.T) {
		err := pkgerrors.ErrProviderUnavailable
		assert.True(t, pkgerrors.IsProviderUnavailable(err))
	})
}

func TestWrapHelpers(t *testing.T) {
	t.Run("WrapValidation", func(t *testing.T) {
		err := pkgerrors.WrapValidation("username", errors.New("too short"))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "username")
		assert.Contains(t, err.Error(), "too short")

		// nil error returns nil
		assert.Nil(t, pkgerrors.WrapValidation("field", nil))
	})

	t.Run("WrapIO", func(t *testing.T) {
		err := pkgerrors.WrapIO("write", "/tmp/file", errors.New("disk full"))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "write")
		assert.Contains(t, err.Error(), "/tmp/file")

		assert.Nil(t, pkgerrors.WrapIO("read", "file", nil))
	})

	t.Run("WrapResource", func(t *testing.T) {
		err := pkgerrors.WrapResource("delete", "model", "gpt-4", errors.New("in use"))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "delete")
		assert.Contains(t, err.Error(), "model")
		assert.Contains(t, err.Error(), "gpt-4")

		assert.Nil(t, pkgerrors.WrapResource("create", "provider", "test", nil))
	})

	t.Run("WrapParse", func(t *testing.T) {
		err := pkgerrors.WrapParse("json", "config.json", errors.New("invalid syntax"))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "json")
		assert.Contains(t, err.Error(), "config.json")

		assert.Nil(t, pkgerrors.WrapParse("yaml", "file.yaml", nil))
	})

	t.Run("WrapAPI", func(t *testing.T) {
		err := pkgerrors.WrapAPI("openai", 429, errors.New("rate limit"))
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "openai")
		assert.Contains(t, err.Error(), "429")

		assert.Nil(t, pkgerrors.WrapAPI("groq", 200, nil))
	})
}

func TestErrorChaining(t *testing.T) {
	t.Run("multiple wrapping", func(t *testing.T) {
		baseErr := errors.New("connection refused")
		ioErr := pkgerrors.WrapIO("connect", "api.example.com", baseErr)
		apiErr := &pkgerrors.APIError{
			Provider: "test-provider",
			Message:  "failed to connect",
			Err:      ioErr,
		}
		syncErr := &pkgerrors.SyncError{
			Provider: "test-provider",
			Err:      apiErr,
		}

		// Check unwrapping chain
		assert.Equal(t, apiErr, syncErr.Unwrap())
		assert.Equal(t, ioErr, apiErr.Unwrap())
		
		// errors.Is should work through the chain
		var targetIOErr *pkgerrors.IOError
		assert.True(t, errors.As(syncErr, &targetIOErr))
		assert.Equal(t, "connect", targetIOErr.Operation)
	})
}

func TestSentinelErrors(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", pkgerrors.ErrNotFound},
		{"ErrAlreadyExists", pkgerrors.ErrAlreadyExists},
		{"ErrInvalidInput", pkgerrors.ErrInvalidInput},
		{"ErrAPIKeyRequired", pkgerrors.ErrAPIKeyRequired},
		{"ErrAPIKeyInvalid", pkgerrors.ErrAPIKeyInvalid},
		{"ErrProviderUnavailable", pkgerrors.ErrProviderUnavailable},
		{"ErrRateLimited", pkgerrors.ErrRateLimited},
		{"ErrTimeout", pkgerrors.ErrTimeout},
		{"ErrCanceled", pkgerrors.ErrCanceled},
		{"ErrNotImplemented", pkgerrors.ErrNotImplemented},
		{"ErrReadOnly", pkgerrors.ErrReadOnly},
	}

	for _, tc := range sentinels {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotNil(t, tc.err)
			assert.NotEmpty(t, tc.err.Error())
		})
	}
}