package auth

import (
	"context"
	stderrors "errors"
	"fmt"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// CheckProvider checks authentication status for a provider.
// Performs local checks only - no network calls are made.
func (c *Checker) CheckProvider(provider *catalogs.Provider, supportedMap map[string]bool) *Status {
	// Check if provider is supported
	if !supportedMap[string(provider.ID)] {
		return &Status{
			State:   StateUnsupported,
			Summary: "No client implementation available",
		}
	}

	sources := c.CheckSources(context.Background(), provider)
	status := aggregateSourceStatus(sources)
	status.Sources = sources
	return status
}

// CheckSources reports every configured source independently without exposing values.
func (c *Checker) CheckSources(ctx context.Context, provider *catalogs.Provider) []SourceStatus {
	if provider.Catalog == nil || len(provider.Catalog.Sources) == 0 {
		return []SourceStatus{{State: StateInvalid, Summary: "Provider has no catalog source"}}
	}
	resolver := c.resolver
	if resolver == nil {
		resolver = NewResolver()
	}
	statuses := make([]SourceStatus, 0, len(provider.Catalog.Sources))
	for _, source := range provider.Catalog.Sources {
		status := SourceStatus{SourceID: source.ID, AcceptedMethods: slices.Clone(source.Auth.Methods)}
		for _, method := range source.Auth.Methods {
			if credential, found := provider.Credentials[method]; found {
				status.Environment = appendCredentialEnvironment(status.Environment, credential)
			}
		}
		if source.Auth.Mode == catalogs.ProviderAuthModeOptional {
			if credential, found := provider.Credentials[catalogs.ProviderCredentialID(catalogs.ProviderCredentialKindAPIKey)]; found {
				status.AcceptedMethods = append(status.AcceptedMethods, catalogs.ProviderCredentialID(catalogs.ProviderCredentialKindAPIKey))
				status.Environment = appendCredentialEnvironment(status.Environment, credential)
			}
			if _, found := resolver.cloud.adapter(provider.ID); found {
				status.AcceptedMethods = append(status.AcceptedMethods, "cloud_chain")
			}
		}
		resolved, err := resolver.Resolve(ctx, provider, source)
		if err != nil {
			status.State = StateInvalid
			if stderrors.Is(err, errors.ErrAPIKeyRequired) || stderrors.Is(err, errors.ErrNotFound) {
				status.State = StateUnavailable
			}
			status.Summary = errors.SafeSummary(err)
			statuses = append(statuses, status)
			continue
		}
		if resolved.Anonymous() {
			status.State = StateUnauthenticated
			status.Summary = "Unauthenticated source"
		} else {
			status.State = StateReady
			status.Summary = fmt.Sprintf("Authentication configured (%s)", resolved.Method())
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func appendCredentialEnvironment(target []string, credential catalogs.ProviderCredential) []string {
	target = append(target, credential.Env...)
	inputIDs := make([]string, 0, len(credential.Inputs))
	for inputID := range credential.Inputs {
		inputIDs = append(inputIDs, inputID)
	}
	slices.Sort(inputIDs)
	for _, inputID := range inputIDs {
		target = append(target, credential.Inputs[inputID].Env...)
	}
	return target
}

func aggregateSourceStatus(sources []SourceStatus) *Status {
	status := &Status{State: StateUnauthenticated, Summary: "All sources are unauthenticated"}
	for _, source := range sources {
		switch source.State {
		case StateInvalid:
			return &Status{State: StateInvalid, Summary: source.SourceID + ": " + source.Summary}
		case StateReady:
			status.State = StateReady
			status.Summary = "At least one source has authentication configured"
		case StateUnavailable:
			if status.State != StateReady {
				status.State = StateUnavailable
				status.Summary = source.SourceID + ": " + source.Summary
			}
		}
	}
	return status
}
