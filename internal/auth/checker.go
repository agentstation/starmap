package auth

import (
	stderrors "errors"
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// CheckProvider checks local catalog-acquisition authentication state.
func (c *Checker) CheckProvider(
	provider *catalogs.Provider,
	supportedMap map[string]bool,
) *Status {
	if provider == nil {
		return &Status{State: StateInvalid, Summary: "Provider is required"}
	}
	if !supportedMap[string(provider.ID)] {
		return &Status{
			State: StateUnsupported, Summary: "No client implementation available",
		}
	}
	if c == nil || c.resolver == nil {
		return &Status{State: StateInvalid, Summary: "Credential resolver is required"}
	}
	material, err := c.resolver.ResolveCatalog(c.context(), provider)
	if err != nil {
		return credentialResolutionStatus(err)
	}
	profile := material.Profile()
	if profile.ID == "" || profile.Primitive == catalogs.ProviderAuthenticationNone {
		return &Status{
			State: StateOptional, Summary: "No catalog credentials required",
			Profile: &ProfileDetails{ID: profile.ID, Primitive: profile.Primitive},
		}
	}
	return &Status{
		State:   StateConfigured,
		Summary: fmt.Sprintf("Catalog credential profile %s is configured", profile.ID),
		Profile: &ProfileDetails{ID: profile.ID, Primitive: profile.Primitive},
	}
}

func credentialResolutionStatus(err error) *Status {
	var authenticationErr *errors.AuthenticationError
	if stderrors.As(err, &authenticationErr) {
		return &Status{State: StateMissing, Summary: "Catalog credentials are not configured"}
	}
	return &Status{State: StateInvalid, Summary: err.Error()}
}
