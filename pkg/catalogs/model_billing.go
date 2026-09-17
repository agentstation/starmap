package catalogs

import (
	"math"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// RecognitionBillingSchemaVersion adds explicit provider billing units.
const RecognitionBillingSchemaVersion uint64 = 10

// ModelBilling declares provider billing units independently of current prices.
// A missing recognition record means that its billing basis is unknown.
type ModelBilling struct {
	Recognition *RecognitionBilling `json:"recognition,omitempty" yaml:"recognition,omitempty"`
}

// RecognitionBillingBasis identifies the units used to settle document recognition.
type RecognitionBillingBasis string

const (
	// RecognitionBillingPages charges for processed pages.
	RecognitionBillingPages RecognitionBillingBasis = "pages"
	// RecognitionBillingTokens charges for measured input and output tokens.
	RecognitionBillingTokens RecognitionBillingBasis = "tokens"
)

// RecognitionBilling declares actual units and optional display assumptions.
// It does not grant recognition capability or supply a price.
type RecognitionBilling struct {
	Basis             RecognitionBillingBasis       `json:"basis" yaml:"basis"`
	InputPageEstimate *RecognitionInputPageEstimate `json:"input_page_estimate,omitempty" yaml:"input_page_estimate,omitempty"`
}

// RecognitionInputPageEstimate describes an estimated input token count per page.
// Consumers must label a derived price as an estimate, retain its assumptions,
// and use the selected input rate and currency. It excludes output and must
// never replace measured usage for settlement.
type RecognitionInputPageEstimate struct {
	Tokens      float64 `json:"tokens" yaml:"tokens"`
	Source      string  `json:"source" yaml:"source"`
	Assumptions string  `json:"assumptions" yaml:"assumptions"`
}

// Validate checks declared billing units and optional estimate assumptions.
// Missing records are valid unknowns. A present recognition record needs a basis.
func (b *ModelBilling) Validate() error {
	if b == nil || b.Recognition == nil {
		return nil
	}
	recognition := b.Recognition
	switch recognition.Basis {
	case RecognitionBillingPages, RecognitionBillingTokens:
	default:
		return billingValidationError("recognition.basis", recognition.Basis, "must be pages or tokens")
	}
	estimate := recognition.InputPageEstimate
	if estimate == nil {
		return nil
	}
	if recognition.Basis != RecognitionBillingTokens {
		return billingValidationError("recognition.input_page_estimate", estimate, "requires token billing")
	}
	if math.IsNaN(estimate.Tokens) || math.IsInf(estimate.Tokens, 0) || estimate.Tokens <= 0 {
		return billingValidationError("recognition.input_page_estimate.tokens", estimate.Tokens, "must be finite and greater than zero")
	}
	if strings.TrimSpace(estimate.Source) == "" || strings.TrimSpace(estimate.Assumptions) == "" {
		return billingValidationError("recognition.input_page_estimate", estimate, "requires a source and assumptions")
	}
	return nil
}

func billingValidationError(field string, value any, message string) error {
	return &errors.ValidationError{Field: "billing." + field, Value: value, Message: message}
}

func deepCopyModelBilling(billing *ModelBilling) *ModelBilling {
	copied := copyPtr(billing)
	if copied == nil {
		return nil
	}
	copied.Recognition = copyPtr(billing.Recognition)
	if copied.Recognition != nil {
		copied.Recognition.InputPageEstimate = copyPtr(billing.Recognition.InputPageEstimate)
	}
	return copied
}
