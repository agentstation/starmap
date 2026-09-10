package catalogs

import "github.com/agentstation/starmap/pkg/errors"

// CatalogRemovalKind distinguishes account entry removal from canonical removal.
type CatalogRemovalKind string

const (
	// CatalogRemovalScoped selects one provider model within one account or public scope.
	CatalogRemovalScoped CatalogRemovalKind = "scoped"
	// CatalogRemovalCanonical selects a canonical model across its provider offerings.
	CatalogRemovalCanonical CatalogRemovalKind = "canonical"
)

// CatalogRemovalScope identifies an account independently of credential binding revisions.
// Publisher identity requires separate transport authentication before enforcement.
type CatalogRemovalScope struct {
	PublisherID string     `json:"publisher_id"`
	ProviderID  ProviderID `json:"provider_id"`
	AccountID   string     `json:"account_id,omitempty"`
	ProjectID   string     `json:"project_id,omitempty"`
	Region      string     `json:"region"`
	APISurface  string     `json:"api_surface"`
	Public      bool       `json:"public"`
}

// CatalogRemovalTarget names exactly one explicit operator removal action.
// Scoped actions preserve other accounts. Canonical actions require a separate selection.
type CatalogRemovalTarget struct {
	Kind            CatalogRemovalKind   `json:"kind"`
	Scope           *CatalogRemovalScope `json:"scope,omitempty"`
	ProviderModelID ProviderModelID      `json:"provider_model_id,omitempty"`
	DefinitionID    ModelDefinitionID    `json:"definition_id,omitempty"`
}

// NewScopedRemovalTarget captures a validated scope without retaining credential identity.
// Callers must resolve the source scope through their authenticated profile link first.
func NewScopedRemovalTarget(scope ProviderMembershipScope, model ProviderModelID) (CatalogRemovalTarget, error) {
	if err := scope.Validate(); err != nil {
		return CatalogRemovalTarget{}, err
	}
	target := CatalogRemovalTarget{Kind: CatalogRemovalScoped, ProviderModelID: model, Scope: &CatalogRemovalScope{
		PublisherID: scope.PublisherID, ProviderID: scope.ProviderID, AccountID: scope.AccountID, ProjectID: scope.ProjectID,
		Region: scope.Region, APISurface: scope.APISurface, Public: scope.Public,
	}}
	if err := target.Validate(); err != nil {
		return CatalogRemovalTarget{}, err
	}
	return target, nil
}

// NewCanonicalRemovalTarget selects a canonical model through a separate explicit action.
func NewCanonicalRemovalTarget(definition ModelDefinitionID) (CatalogRemovalTarget, error) {
	target := CatalogRemovalTarget{Kind: CatalogRemovalCanonical, DefinitionID: definition}
	if err := target.Validate(); err != nil {
		return CatalogRemovalTarget{}, err
	}
	return target, nil
}

// Validate rejects incomplete and mixed removal selectors before publication.
func (t CatalogRemovalTarget) Validate() error {
	switch t.Kind {
	case CatalogRemovalScoped:
		if t.Scope == nil || t.DefinitionID != "" || !validMembershipIdentifier(string(t.ProviderModelID)) {
			return invalidRemovalTarget("scope", "requires one account scope and provider model ID without a canonical selector")
		}
		return t.Scope.validate()
	case CatalogRemovalCanonical:
		if t.Scope != nil || t.ProviderModelID != "" || !validMembershipIdentifier(string(t.DefinitionID)) {
			return invalidRemovalTarget("definition_id", "requires one canonical model ID without an account selector")
		}
		if _, _, err := ParseModelDefinitionID(t.DefinitionID); err != nil {
			return invalidRemovalTarget("definition_id", "must be a canonical author and model identity")
		}
		return nil
	default:
		return invalidRemovalTarget("kind", "requires an explicit scoped or canonical action")
	}
}

// MatchesScope compares exact account identity and the opaque provider model ID.
// Validate targets and source scopes before this query.
// Credential rotation does not change the target. The query allocates no memory and reads no storage.
func (t CatalogRemovalTarget) MatchesScope(scope ProviderMembershipScope, model ProviderModelID) bool {
	if t.Kind != CatalogRemovalScoped || t.Scope == nil || t.DefinitionID != "" || t.ProviderModelID != model {
		return false
	}
	target := t.Scope
	return target.PublisherID == scope.PublisherID && target.ProviderID == scope.ProviderID &&
		target.AccountID == scope.AccountID && target.ProjectID == scope.ProjectID && target.Region == scope.Region &&
		target.APISurface == scope.APISurface && target.Public == scope.Public
}

func (s CatalogRemovalScope) validate() error {
	for _, field := range []struct{ name, value string }{
		{"publisher_id", s.PublisherID}, {"provider_id", string(s.ProviderID)}, {"region", s.Region}, {"api_surface", s.APISurface},
	} {
		if !validMembershipIdentifier(field.value) {
			return invalidRemovalTarget(field.name, "must be a bounded nonempty identifier")
		}
	}
	if s.Public && (s.AccountID != "" || s.ProjectID != "") {
		return invalidRemovalTarget("public", "cannot declare an account or project")
	}
	if !s.Public && s.AccountID == "" && s.ProjectID == "" {
		return invalidRemovalTarget("account_id", "account or project is required for a private scope")
	}
	for _, value := range []string{s.AccountID, s.ProjectID} {
		if value != "" && !validMembershipIdentifier(value) {
			return invalidRemovalTarget("account_id", "scope identifiers must be bounded and contain no control characters")
		}
	}
	return nil
}

func invalidRemovalTarget(field, message string) error {
	return &errors.ValidationError{Field: "catalog_removal." + field, Message: message}
}
