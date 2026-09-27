package catalogs

// RerankBilling declares every charge for synchronous text reranking.
// QueryDocumentTokens charges the query once for each submitted document.
// ContextWindow bounds one query-document pair. InputTokens bounds the complete request.
// RequestCharge declares whether Operations.Request also applies.
// The contract excludes provider-native batch discounts and account credits.
type RerankBilling struct {
	Basis         RerankBillingBasis `json:"basis" yaml:"basis"`
	RequestCharge *bool              `json:"request_charge" yaml:"request_charge"`
}

// RerankBillingBasis identifies the measured units for rerank charges.
type RerankBillingBasis string

// RerankBillingQueryDocumentTokens bills all processed query-document input tokens.
const RerankBillingQueryDocumentTokens RerankBillingBasis = "query_document_tokens"

func (b *RerankBilling) validate() error {
	if b == nil {
		return nil
	}
	if b.Basis != RerankBillingQueryDocumentTokens {
		return billingValidationError("rerank.basis", b.Basis, "must be query_document_tokens")
	}
	if b.RequestCharge == nil {
		return billingValidationError("rerank.request_charge", nil, "requires an explicit boolean")
	}
	return nil
}
