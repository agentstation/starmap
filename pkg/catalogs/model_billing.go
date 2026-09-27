package catalogs

import (
	"math"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// RecognitionBillingSchemaVersion adds explicit provider billing units.
const RecognitionBillingSchemaVersion uint64 = 10

// TextChatBillingSchemaVersion adds complete text-chat charge declarations.
const TextChatBillingSchemaVersion uint64 = 11

// ModelBilling declares provider billing units independently of current prices.
// A missing operation record means that its billing basis is unknown.
type ModelBilling struct {
	Recognition *RecognitionBilling `json:"recognition,omitempty" yaml:"recognition,omitempty"`
	TextChat    *TextChatBilling    `json:"text_chat,omitempty" yaml:"text_chat,omitempty"`
}

// TokenBillingClass names a disjoint token class and its ModelTokenPricing field.
type TokenBillingClass string

const (
	// TokenBillingInput identifies uncached input tokens.
	TokenBillingInput TokenBillingClass = "input"
	// TokenBillingCacheRead identifies input tokens read from a provider cache.
	TokenBillingCacheRead TokenBillingClass = "cache_read"
	// TokenBillingCacheWrite identifies input tokens written to a provider cache.
	TokenBillingCacheWrite TokenBillingClass = "cache_write"
	// TokenBillingOutput identifies output tokens outside a separate reasoning class.
	TokenBillingOutput TokenBillingClass = "output"
	// TokenBillingReasoning identifies separately priced reasoning output tokens.
	TokenBillingReasoning TokenBillingClass = "reasoning"
)

// TextChatBilling declares every charge for online text chat at the default service level.
// Input and Output partition the complete token totals into disjoint price classes.
// Each list must include its ordinary class. A class absent from the list has no separate charge.
// Reasoning uses the output rate unless Output explicitly declares a separate reasoning class.
//
// RequestCharge must explicitly state whether Operations.Request also applies.
// This contract covers text and client function declarations, for streaming and non-streaming calls.
// It excludes media, hosted tools, provider extensions, batch discounts, and other service levels.
// A missing record is unknown. Neither this record nor a zero price grants a capability.
type TextChatBilling struct {
	Input         []TokenBillingClass `json:"input" yaml:"input"`
	Output        []TokenBillingClass `json:"output" yaml:"output"`
	RequestCharge *bool               `json:"request_charge" yaml:"request_charge"`
}

func (b *TextChatBilling) validate() error {
	if b == nil {
		return nil
	}
	if b.RequestCharge == nil {
		return billingValidationError("text_chat.request_charge", nil, "requires an explicit boolean")
	}
	for _, group := range []struct {
		name     string
		classes  []TokenBillingClass
		required TokenBillingClass
		allowed  []TokenBillingClass
	}{
		{"input", b.Input, TokenBillingInput, []TokenBillingClass{TokenBillingInput, TokenBillingCacheRead, TokenBillingCacheWrite}},
		{"output", b.Output, TokenBillingOutput, []TokenBillingClass{TokenBillingOutput, TokenBillingReasoning}},
	} {
		if !slices.Contains(group.classes, group.required) {
			return billingValidationError("text_chat."+group.name, group.classes, "requires the ordinary token class")
		}
		for i, class := range group.classes {
			if !slices.Contains(group.allowed, class) || slices.Contains(group.classes[:i], class) {
				return billingValidationError("text_chat."+group.name, class, "contains an unsupported or repeated token class")
			}
		}
	}
	return nil
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
	if b == nil {
		return nil
	}
	if err := b.TextChat.validate(); err != nil {
		return err
	}
	if b.Recognition == nil {
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
	copied.TextChat = copyPtr(billing.TextChat)
	if copied.TextChat != nil {
		copied.TextChat.Input = slices.Clone(billing.TextChat.Input)
		copied.TextChat.Output = slices.Clone(billing.TextChat.Output)
		copied.TextChat.RequestCharge = copyPtr(billing.TextChat.RequestCharge)
	}
	if copied.Recognition != nil {
		copied.Recognition.InputPageEstimate = copyPtr(billing.Recognition.InputPageEstimate)
	}
	return copied
}
