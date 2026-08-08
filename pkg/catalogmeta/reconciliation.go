package catalogmeta

// ReconciliationIssueCode identifies a stable non-fatal reconciliation result.
type ReconciliationIssueCode string

const (
	// ReconciliationIssueUnresolvedModelReference means a provider offering
	// could not resolve an explicit provider-independent model identity.
	ReconciliationIssueUnresolvedModelReference ReconciliationIssueCode = "unresolved_model_reference"
)

// ReconciliationIssue records one provider offering excluded from a canonical
// catalog generation. ProviderModelID remains the provider's exact opaque ID.
type ReconciliationIssue struct {
	Code            ReconciliationIssueCode `json:"code" yaml:"code"`
	ProviderID      string                  `json:"provider_id" yaml:"provider_id"`
	ProviderModelID string                  `json:"provider_model_id" yaml:"provider_model_id"`
	Message         string                  `json:"message" yaml:"message"`
}
