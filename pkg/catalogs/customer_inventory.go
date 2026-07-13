package catalogs

import (
	"fmt"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// CustomerScope identifies a credential-scoped cloud or provider account.
// It is deliberately absent from Catalog and public generation payloads.
type CustomerScope struct {
	AccountID      string `json:"account_id,omitempty" yaml:"account_id,omitempty"`
	SubscriptionID string `json:"subscription_id,omitempty" yaml:"subscription_id,omitempty"`
	ProjectID      string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	WorkspaceID    string `json:"workspace_id,omitempty" yaml:"workspace_id,omitempty"`
}

// CustomerDeployment is one account-specific deployment or endpoint.
type CustomerDeployment struct {
	ID              string             `json:"id" yaml:"id"`
	DefinitionID    ModelDefinitionID  `json:"definition_id" yaml:"definition_id"`
	ProviderModelID ProviderModelID    `json:"provider_model_id" yaml:"provider_model_id"`
	Region          *CloudRegion       `json:"region,omitempty" yaml:"region,omitempty"`
	Deployment      ProviderDeployment `json:"deployment" yaml:"deployment"`
	Endpoint        string             `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Aliases         []string           `json:"aliases,omitempty" yaml:"aliases,omitempty"`
}

// CustomerInventory is a private, credential-scoped deployment observation.
// It is not a catalogs.Catalog resource and cannot enter embedded publication.
type CustomerInventory struct {
	ProviderID  ProviderID           `json:"provider_id" yaml:"provider_id"`
	Scope       CustomerScope        `json:"scope" yaml:"scope"`
	ObservedAt  time.Time            `json:"observed_at" yaml:"observed_at"`
	Deployments []CustomerDeployment `json:"deployments" yaml:"deployments"`
}

// Validate verifies private inventory identity without making it public-catalog eligible.
func (i CustomerInventory) Validate() error {
	if strings.TrimSpace(string(i.ProviderID)) == "" {
		return customerInventoryError("provider_id", i.ProviderID, "is required")
	}
	if i.ObservedAt.IsZero() {
		return customerInventoryError("observed_at", i.ObservedAt, "is required")
	}
	if i.Scope == (CustomerScope{}) {
		return customerInventoryError("scope", i.Scope, "at least one account scope identifier is required")
	}
	seen := make(map[string]struct{}, len(i.Deployments))
	for index, deployment := range i.Deployments {
		prefix := fmt.Sprintf("deployments[%d]", index)
		if strings.TrimSpace(deployment.ID) == "" || strings.TrimSpace(string(deployment.DefinitionID)) == "" || strings.TrimSpace(string(deployment.ProviderModelID)) == "" {
			return customerInventoryError(prefix, deployment, "requires id, definition_id, and provider_model_id")
		}
		if strings.TrimSpace(deployment.Deployment.Type) == "" {
			return customerInventoryError(prefix+".deployment.type", deployment.Deployment.Type, "is required")
		}
		if _, found := seen[deployment.ID]; found {
			return customerInventoryError(prefix+".id", deployment.ID, "must be unique")
		}
		seen[deployment.ID] = struct{}{}
	}
	return nil
}

func customerInventoryError(field string, value any, message string) error {
	return &errors.ValidationError{Field: "customer_inventory." + field, Value: value, Message: message}
}
