package catalogs

import (
	"fmt"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// RouteAliasID is a Starport-facing routing identity independent of provider IDs.
type RouteAliasID string

// RouteAlias names a set of candidate offering identities. It intentionally
// contains no weights, fallback order, tenancy, or routing strategy.
type RouteAlias struct {
	ID      RouteAliasID  `json:"id" yaml:"id"`
	Targets []OfferingKey `json:"targets" yaml:"targets"`
}

// RouteAliasRejectionReason classifies why a target is not currently eligible.
type RouteAliasRejectionReason string

const (
	// RouteAliasRejectedMissing means the offering key is absent from the catalog.
	RouteAliasRejectedMissing RouteAliasRejectionReason = "missing"
	// RouteAliasRejectedUnavailable means the provider marks the offering unavailable.
	RouteAliasRejectedUnavailable RouteAliasRejectionReason = "unavailable"
	// RouteAliasRejectedRetired means the provider has retired the offering.
	RouteAliasRejectedRetired RouteAliasRejectionReason = "retired"
	// RouteAliasRejectedNotRoutable means the offering has no supported server-to-server invocation contract.
	RouteAliasRejectedNotRoutable RouteAliasRejectionReason = "not_routable"
)

// RouteAliasRejection records one ineligible target without hiding it.
type RouteAliasRejection struct {
	Key    OfferingKey               `json:"key" yaml:"key"`
	Reason RouteAliasRejectionReason `json:"reason" yaml:"reason"`
}

// RouteAliasResolution is a point-in-time materialization against one catalog generation.
type RouteAliasResolution struct {
	AliasID  RouteAliasID          `json:"alias_id" yaml:"alias_id"`
	Eligible []ProviderOffering    `json:"eligible" yaml:"eligible"`
	Rejected []RouteAliasRejection `json:"rejected,omitempty" yaml:"rejected,omitempty"`
}

// Validate verifies route identity and exact target uniqueness.
func (a RouteAlias) Validate() error {
	if strings.TrimSpace(string(a.ID)) == "" {
		return &errors.ValidationError{Field: "id", Value: a.ID, Message: validationMessageIsRequired}
	}
	if len(a.Targets) == 0 {
		return &errors.ValidationError{Field: "targets", Message: "at least one offering key is required"}
	}
	seen := make(map[OfferingKey]struct{}, len(a.Targets))
	for index, target := range a.Targets {
		if strings.TrimSpace(string(target.ProviderID)) == "" {
			return &errors.ValidationError{Field: fmt.Sprintf("targets[%d].provider_id", index), Message: validationMessageIsRequired}
		}
		if strings.TrimSpace(string(target.ProviderModelID)) == "" {
			return &errors.ValidationError{Field: fmt.Sprintf("targets[%d].provider_model_id", index), Message: validationMessageIsRequired}
		}
		if _, exists := seen[target]; exists {
			return &errors.ValidationError{Field: fmt.Sprintf("targets[%d]", index), Value: target, Message: "offering key must be unique"}
		}
		seen[target] = struct{}{}
	}
	return nil
}

// MaterializeRouteAlias resolves current eligibility without storing routing
// policy in source ingestion or the canonical catalog.
func (r *Catalog) MaterializeRouteAlias(alias RouteAlias) (RouteAliasResolution, error) {
	if err := alias.Validate(); err != nil {
		return RouteAliasResolution{}, err
	}
	resolution := RouteAliasResolution{AliasID: alias.ID}
	for _, key := range alias.Targets {
		offering, found := r.offerings[key]
		if !found {
			resolution.Rejected = append(resolution.Rejected, RouteAliasRejection{Key: key, Reason: RouteAliasRejectedMissing})
			continue
		}
		switch {
		case offering.Lifecycle == OfferingLifecycleRetired:
			resolution.Rejected = append(resolution.Rejected, RouteAliasRejection{Key: key, Reason: RouteAliasRejectedRetired})
		case offering.Availability == OfferingAvailabilityUnavailable:
			resolution.Rejected = append(resolution.Rejected, RouteAliasRejection{Key: key, Reason: RouteAliasRejectedUnavailable})
		case !offering.IsRoutable():
			resolution.Rejected = append(resolution.Rejected, RouteAliasRejection{Key: key, Reason: RouteAliasRejectedNotRoutable})
		default:
			resolution.Eligible = append(resolution.Eligible, copyProviderOffering(offering))
		}
	}
	return resolution, nil
}
