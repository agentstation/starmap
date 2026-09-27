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

// EmbeddingBillingSchemaVersion adds complete embedding charge declarations.
const EmbeddingBillingSchemaVersion uint64 = 12

// RecognitionChargesSchemaVersion adds complete recognition charge declarations.
const RecognitionChargesSchemaVersion uint64 = 13

// ModerationBillingSchemaVersion adds complete moderation charge declarations.
const ModerationBillingSchemaVersion uint64 = 14

// RerankBillingSchemaVersion adds complete rerank charge declarations.
const RerankBillingSchemaVersion uint64 = 15

// SpeechBillingSchemaVersion adds character-priced speech declarations.
const SpeechBillingSchemaVersion uint64 = 16

// ImageBillingSchemaVersion adds explicit image charge units.
const ImageBillingSchemaVersion uint64 = 17

// ModelBilling declares provider billing units independently of current prices.
// A missing operation record means that its billing basis is unknown.
type ModelBilling struct {
	Images      *ImageBilling       `json:"images,omitempty" yaml:"images,omitempty"`
	Speech      *SpeechBilling      `json:"speech,omitempty" yaml:"speech,omitempty"`
	Rerank      *RerankBilling      `json:"rerank,omitempty" yaml:"rerank,omitempty"`
	Moderations *ModerationBilling  `json:"moderations,omitempty" yaml:"moderations,omitempty"`
	Recognition *RecognitionBilling `json:"recognition,omitempty" yaml:"recognition,omitempty"`
	Embeddings  *EmbeddingBilling   `json:"embeddings,omitempty" yaml:"embeddings,omitempty"`
	TextChat    *TextChatBilling    `json:"text_chat,omitempty" yaml:"text_chat,omitempty"`
}

// ModerationBilling declares all charges for one synchronous text moderation request.
// The requests basis uses Operations.Request, including an explicit zero price.
// It does not declare token consumption or grant moderation capability.
type ModerationBilling struct {
	Basis ModerationBillingBasis `json:"basis" yaml:"basis"`
}

// ModerationBillingBasis identifies the units for moderation charges.
type ModerationBillingBasis string

// ModerationBillingRequests charges once per HTTP request, independent of input count.
const ModerationBillingRequests ModerationBillingBasis = "requests"

func (b *ModerationBilling) validate() error {
	if b != nil && b.Basis != ModerationBillingRequests {
		return billingValidationError("moderations.basis", b.Basis, "must be requests")
	}
	return nil
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
	return validateTokenCharges("text_chat", b.Input, b.Output, b.RequestCharge)
}

func validateTokenCharges(field string, input, output []TokenBillingClass, requestCharge *bool) error {
	if requestCharge == nil {
		return billingValidationError(field+".request_charge", nil, "requires an explicit boolean")
	}
	for _, group := range []struct {
		name     string
		classes  []TokenBillingClass
		required TokenBillingClass
		allowed  []TokenBillingClass
	}{
		{"input", input, TokenBillingInput, []TokenBillingClass{TokenBillingInput, TokenBillingCacheRead, TokenBillingCacheWrite}},
		{"output", output, TokenBillingOutput, []TokenBillingClass{TokenBillingOutput, TokenBillingReasoning}},
	} {
		if !slices.Contains(group.classes, group.required) {
			return billingValidationError(field+"."+group.name, group.classes, "requires the ordinary token class")
		}
		for i, class := range group.classes {
			if !slices.Contains(group.allowed, class) || slices.Contains(group.classes[:i], class) {
				return billingValidationError(field+"."+group.name, class, "contains an unsupported or repeated token class")
			}
		}
	}
	return nil
}

// EmbeddingBilling declares all charges for synchronous text or token-ID embeddings.
// The input_tokens basis uses the complete input count and the ordinary input price.
// RequestCharge declares whether Operations.Request also applies.
// This contract excludes media inputs and provider-side batch discounts.
// Vector dimensions do not change the declared token rate.
type EmbeddingBilling struct {
	Basis         EmbeddingBillingBasis `json:"basis" yaml:"basis"`
	RequestCharge *bool                 `json:"request_charge" yaml:"request_charge"`
}

// EmbeddingBillingBasis identifies the units for embedding charges.
type EmbeddingBillingBasis string

// EmbeddingBillingInputTokens charges for all measured input tokens.
const EmbeddingBillingInputTokens EmbeddingBillingBasis = "input_tokens"

func (b *EmbeddingBilling) validate() error {
	if b == nil {
		return nil
	}
	if b.Basis != EmbeddingBillingInputTokens {
		return billingValidationError("embeddings.basis", b.Basis, "must be input_tokens")
	}
	if b.RequestCharge == nil {
		return billingValidationError("embeddings.request_charge", nil, "requires an explicit boolean")
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
	// RequestCharge completes the charge declaration. Nil retains an unknown contract.
	RequestCharge *bool `json:"request_charge,omitempty" yaml:"request_charge,omitempty"`
	// Input and Output partition token billing. Page billing leaves both absent.
	Input             []TokenBillingClass           `json:"input,omitempty" yaml:"input,omitempty"`
	Output            []TokenBillingClass           `json:"output,omitempty" yaml:"output,omitempty"`
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
	if err := b.Images.validate(); err != nil {
		return err
	}
	if err := b.Speech.validate(); err != nil {
		return err
	}
	if err := b.Rerank.validate(); err != nil {
		return err
	}
	if err := b.Moderations.validate(); err != nil {
		return err
	}
	if err := b.Embeddings.validate(); err != nil {
		return err
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
	if recognition.hasCharges() {
		if recognition.RequestCharge == nil {
			return billingValidationError("recognition.request_charge", nil, "requires an explicit boolean")
		}
		if recognition.Basis == RecognitionBillingPages {
			if len(recognition.Input) != 0 || len(recognition.Output) != 0 {
				return billingValidationError("recognition", recognition, "page billing cannot contain token classes")
			}
		} else if err := validateTokenCharges("recognition", recognition.Input, recognition.Output, recognition.RequestCharge); err != nil {
			return err
		}
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
	copied.Images = copyPtr(billing.Images)
	if copied.Images != nil {
		copied.Images.RequestCharge = copyPtr(billing.Images.RequestCharge)
		copied.Images.Operations = slices.Clone(billing.Images.Operations)
	}
	copied.Speech = copyPtr(billing.Speech)
	if copied.Speech != nil {
		copied.Speech.RequestCharge = copyPtr(billing.Speech.RequestCharge)
	}
	copied.Rerank = copyPtr(billing.Rerank)
	if copied.Rerank != nil {
		copied.Rerank.RequestCharge = copyPtr(billing.Rerank.RequestCharge)
	}
	copied.Moderations = copyPtr(billing.Moderations)
	copied.Recognition = copyPtr(billing.Recognition)
	copied.Embeddings = copyPtr(billing.Embeddings)
	if copied.Embeddings != nil {
		copied.Embeddings.RequestCharge = copyPtr(billing.Embeddings.RequestCharge)
	}
	copied.TextChat = copyPtr(billing.TextChat)
	if copied.TextChat != nil {
		copied.TextChat.Input = slices.Clone(billing.TextChat.Input)
		copied.TextChat.Output = slices.Clone(billing.TextChat.Output)
		copied.TextChat.RequestCharge = copyPtr(billing.TextChat.RequestCharge)
	}
	if copied.Recognition != nil {
		copied.Recognition.InputPageEstimate = copyPtr(billing.Recognition.InputPageEstimate)
		copied.Recognition.RequestCharge = copyPtr(billing.Recognition.RequestCharge)
		copied.Recognition.Input = slices.Clone(billing.Recognition.Input)
		copied.Recognition.Output = slices.Clone(billing.Recognition.Output)
	}
	return copied
}

func (b *RecognitionBilling) hasCharges() bool {
	return b != nil && (b.RequestCharge != nil || len(b.Input) != 0 || len(b.Output) != 0)
}
