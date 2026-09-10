package catalogs

import (
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/pkg/errors"
)

// MembershipAuthority declares which inventory an accepted scope may replace.
type MembershipAuthority string

const (
	// MembershipEvidenceOnly supplies positive evidence without removal authority.
	MembershipEvidenceOnly MembershipAuthority = ""
	// MembershipScopeAuthority permits replacement within the declared scope.
	MembershipScopeAuthority MembershipAuthority = "scope"
	// MembershipProviderAuthority permits a source to replace a public provider inventory.
	MembershipProviderAuthority MembershipAuthority = "provider"
)

// MembershipInventory is the last complete accepted inventory of a declared scope.
// An empty model list is a known empty inventory. A missing inventory is unknown.
type MembershipInventory struct {
	ObservationID string    `json:"observation_id"`
	ObservedAt    time.Time `json:"observed_at"`
	ModelIDs      []string  `json:"model_ids"`
}

// MembershipPresence records positive evidence accepted after the complete inventory.
type MembershipPresence struct {
	ModelID       string    `json:"model_id"`
	ObservationID string    `json:"observation_id"`
	ObservedAt    time.Time `json:"observed_at"`
}

// ProviderMembershipScope carries effective membership for one publisher and binding.
// Consumers must resolve an explicit profile link before applying its restrictions.
// These records contain no credentials or inference routing policy.
type ProviderMembershipScope struct {
	PublisherID     string               `json:"publisher_id"`
	BindingID       string               `json:"binding_id"`
	BindingRevision string               `json:"binding_revision"`
	ProviderID      ProviderID           `json:"provider_id"`
	AccountID       string               `json:"account_id,omitempty"`
	ProjectID       string               `json:"project_id,omitempty"`
	Region          string               `json:"region"`
	APISurface      string               `json:"api_surface"`
	Public          bool                 `json:"public"`
	Authority       MembershipAuthority  `json:"authority"`
	Inventory       *MembershipInventory `json:"inventory"`
	Additions       []MembershipPresence `json:"additions"`
}

// Membership reports scope-local presence and whether the evidence establishes it.
// Consumers must not treat unknown absence as permission for a required scope link.
func (s ProviderMembershipScope) Membership(modelID string) (present, known bool) {
	for _, addition := range s.Additions {
		if addition.ModelID == modelID {
			return true, true
		}
	}
	if s.Inventory == nil {
		return false, false
	}
	return slices.Contains(s.Inventory.ModelIDs, modelID), true
}

// Validate checks the effective scope independently of transport authentication.
// Publication must also bind each observation identity to accepted source evidence.
func (s ProviderMembershipScope) Validate() error {
	for _, field := range []struct{ name, value string }{
		{"publisher_id", s.PublisherID}, {"binding_id", s.BindingID}, {"binding_revision", s.BindingRevision},
		{"provider_id", string(s.ProviderID)}, {"region", s.Region}, {"api_surface", s.APISurface},
	} {
		if !validMembershipIdentifier(field.value) {
			return invalidMembershipScope(field.name, "must be a bounded nonempty identifier")
		}
	}

	if s.Public && (s.AccountID != "" || s.ProjectID != "") {
		return invalidMembershipScope("public", "cannot declare an account or project")
	}
	if !s.Public && s.AccountID == "" && s.ProjectID == "" {
		return invalidMembershipScope("account_id", "account or project is required for a private scope")
	}
	for _, value := range []string{s.AccountID, s.ProjectID} {
		if value != "" && !validMembershipIdentifier(value) {
			return invalidMembershipScope("account_id", "scope identifiers must be bounded and contain no control characters")
		}
	}
	switch s.Authority {
	case MembershipEvidenceOnly:
		if s.Inventory != nil {
			return invalidMembershipScope("inventory", "requires explicit membership authority")
		}
	case MembershipScopeAuthority:
	case MembershipProviderAuthority:
		if !s.Public {
			return invalidMembershipScope("authority", "provider-wide authority requires a public scope")
		}
	default:
		return invalidMembershipScope("authority", "is not supported")
	}
	return s.validateInventory()
}

func (s ProviderMembershipScope) validateInventory() error {
	if s.Inventory != nil {
		if s.Inventory.ModelIDs == nil {
			return invalidMembershipScope("inventory.model_ids", "must be an array; an empty array declares known absence")
		}
		if !validMembershipIdentifier(s.Inventory.ObservationID) || s.Inventory.ObservedAt.IsZero() {
			return invalidMembershipScope("inventory", "requires an observation identity and time")
		}
		seen := make(map[string]bool, len(s.Inventory.ModelIDs))
		for _, model := range s.Inventory.ModelIDs {
			if !validMembershipIdentifier(model) || seen[model] {
				return invalidMembershipScope("inventory.model_ids", "must contain unique bounded identifiers")
			}
			seen[model] = true
		}
	}
	seen := make(map[string]bool, len(s.Additions))
	for _, addition := range s.Additions {
		if !validMembershipIdentifier(addition.ModelID) || !validMembershipIdentifier(addition.ObservationID) || addition.ObservedAt.IsZero() || seen[addition.ModelID] {
			return invalidMembershipScope("additions", "requires unique models with observation identities and times")
		}
		if s.Inventory != nil && !addition.ObservedAt.After(s.Inventory.ObservedAt) {
			return invalidMembershipScope("additions", "must be newer than the complete inventory")
		}
		seen[addition.ModelID] = true
	}
	return nil
}

const maxMembershipIdentifierBytes = 4096

func validMembershipIdentifier(value string) bool {
	return value != "" && len(value) <= maxMembershipIdentifierBytes && utf8.ValidString(value) && strings.TrimSpace(value) == value && strings.IndexFunc(value, unicode.IsControl) < 0
}

func invalidMembershipScope(field, message string) error {
	return &errors.ValidationError{Field: "membership_scope." + field, Message: message}
}
