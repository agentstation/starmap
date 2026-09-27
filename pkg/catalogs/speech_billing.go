package catalogs

// SpeechBilling declares all charges for synchronous character-priced speech.
// Input characters are Unicode code points, including whitespace and punctuation.
// RequestCharge states whether Operations.Request also applies.
// CharacterInput supplies the price per character. No audio-output fee applies.
// This contract does not establish token consumption.
type SpeechBilling struct {
	Basis              SpeechBillingBasis `json:"basis" yaml:"basis"`
	MaxInputCharacters int64              `json:"max_input_characters" yaml:"max_input_characters"`
	RequestCharge      *bool              `json:"request_charge" yaml:"request_charge"`
}

// SpeechBillingBasis identifies the input unit for speech generation.
type SpeechBillingBasis string

// SpeechBillingCodePoints charges each Unicode code point in the submitted input.
const SpeechBillingCodePoints SpeechBillingBasis = "unicode_code_points"

func (b *SpeechBilling) validate() error {
	if b == nil {
		return nil
	}
	if b.Basis != SpeechBillingCodePoints {
		return billingValidationError("speech.basis", b.Basis, "must be unicode_code_points")
	}
	if b.MaxInputCharacters <= 0 {
		return billingValidationError("speech.max_input_characters", b.MaxInputCharacters, "must be positive")
	}
	if b.RequestCharge == nil {
		return billingValidationError("speech.request_charge", nil, "requires an explicit boolean")
	}
	return nil
}
