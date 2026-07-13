package sources

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// CloudCredentialResolver resolves one SDK-native credential/configuration value.
// available=false means the chain should try the next resolver.
type CloudCredentialResolver[T any] struct {
	Name    string
	Resolve func(context.Context) (value T, available bool, err error)
}

// ResolvedCloudCredential contains secret-safe diagnostics and an unexported
// SDK-native value. JSON/YAML serialization can never include Value().
type ResolvedCloudCredential[T any] struct {
	Provider string `json:"provider" yaml:"provider"`
	Source   string `json:"source" yaml:"source"`
	value    T
}

// Value returns the runtime-only SDK-native credential/configuration value.
func (c ResolvedCloudCredential[T]) Value() T { return c.value }

// CloudCredentialChain tries context-aware resolvers in declared order.
type CloudCredentialChain[T any] struct {
	provider  string
	resolvers []CloudCredentialResolver[T]
}

// NewCloudCredentialChain creates an immutable SDK-neutral credential chain.
func NewCloudCredentialChain[T any](provider string, resolvers ...CloudCredentialResolver[T]) (*CloudCredentialChain[T], error) {
	if strings.TrimSpace(provider) == "" {
		return nil, &errors.ValidationError{Field: "cloud_credentials.provider", Message: "is required"}
	}
	if len(resolvers) == 0 {
		return nil, &errors.ValidationError{Field: "cloud_credentials.resolvers", Message: "at least one resolver is required"}
	}
	copyResolvers := append([]CloudCredentialResolver[T](nil), resolvers...)
	for index, resolver := range copyResolvers {
		if strings.TrimSpace(resolver.Name) == "" || resolver.Resolve == nil {
			return nil, &errors.ValidationError{Field: "cloud_credentials.resolvers", Value: index, Message: "each resolver requires a name and function"}
		}
	}
	return &CloudCredentialChain[T]{provider: provider, resolvers: copyResolvers}, nil
}

// Resolve returns the first available credential without persisting it.
func (c *CloudCredentialChain[T]) Resolve(ctx context.Context) (ResolvedCloudCredential[T], error) {
	if c == nil {
		return ResolvedCloudCredential[T]{}, &errors.ValidationError{Field: "cloud_credentials.chain", Message: "is required"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for _, resolver := range c.resolvers {
		if err := ctx.Err(); err != nil {
			return ResolvedCloudCredential[T]{}, err
		}
		value, available, err := resolver.Resolve(ctx)
		if err != nil {
			return ResolvedCloudCredential[T]{}, &errors.AuthenticationError{
				Provider: c.provider, Method: resolver.Name, Message: "cloud credential resolution failed", Err: err,
			}
		}
		if available {
			return ResolvedCloudCredential[T]{Provider: c.provider, Source: resolver.Name, value: value}, nil
		}
	}
	return ResolvedCloudCredential[T]{}, &errors.AuthenticationError{
		Provider: c.provider, Method: "credential-chain", Message: "no cloud credential source is available",
	}
}
