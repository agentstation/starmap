package sources

import (
	"context"
	stderrors "errors"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// ProviderOutcome is the terminal state of one provider acquisition attempt.
// Every attempt reaches exactly one outcome.
type ProviderOutcome string

const (
	// ProviderOutcomeSucceeded means the provider answered and the observation
	// carries provider records.
	ProviderOutcomeSucceeded ProviderOutcome = "succeeded"

	// ProviderOutcomeSkippedNotConfigured means the deployment holds no
	// catalog-acquisition credential for the provider. Acquisition sends no
	// request for a provider with this outcome.
	ProviderOutcomeSkippedNotConfigured ProviderOutcome = "skipped_not_configured"

	// ProviderOutcomeFailed means the attempt reached the provider and did not
	// produce a usable observation.
	ProviderOutcomeFailed ProviderOutcome = "failed"
)

// Valid reports whether the outcome is one of the three defined states.
func (o ProviderOutcome) Valid() bool {
	switch o {
	case ProviderOutcomeSucceeded,
		ProviderOutcomeSkippedNotConfigured,
		ProviderOutcomeFailed:
		return true
	default:
		return false
	}
}

// String returns the wire value of the outcome.
func (o ProviderOutcome) String() string { return string(o) }

// ProviderReason is a safe machine-readable cause for a skip or a failure. A
// reason names no URL, no host, no token, and no credential value.
type ProviderReason string

const (
	// ProviderReasonCredentialReferenceInvalid means the credential reference
	// does not parse or names an unsupported source.
	ProviderReasonCredentialReferenceInvalid ProviderReason = "credential_reference_invalid" //nolint:gosec // A reason code holds no credential value.

	// ProviderReasonCredentialUnavailable means the deployment holds no credential.
	ProviderReasonCredentialUnavailable ProviderReason = "credential_unavailable" //nolint:gosec // A reason code holds no credential value.

	// ProviderReasonCredentialRejected means the provider refused the credential.
	ProviderReasonCredentialRejected ProviderReason = "credential_rejected" //nolint:gosec // A reason code holds no credential value.

	// ProviderReasonCredentialExpired means the credential is past its expiry.
	ProviderReasonCredentialExpired ProviderReason = "credential_expired" //nolint:gosec // A reason code holds no credential value.

	// ProviderReasonInsufficientScope means the credential lacks a required scope.
	ProviderReasonInsufficientScope ProviderReason = "insufficient_scope"

	// ProviderReasonRateLimited means the provider applied a request budget.
	ProviderReasonRateLimited ProviderReason = "rate_limited"

	// ProviderReasonRequestTimeout means the attempt exceeded its time budget.
	ProviderReasonRequestTimeout ProviderReason = "request_timeout"

	// ProviderReasonTransportFailed means the connection failed before a reply.
	ProviderReasonTransportFailed ProviderReason = "transport_failed"

	// ProviderReasonResponseInvalid means the reply did not match the contract.
	ProviderReasonResponseInvalid ProviderReason = "response_invalid"
)

// providerReasons lists every defined reason in declaration order.
var providerReasons = []ProviderReason{
	ProviderReasonCredentialReferenceInvalid,
	ProviderReasonCredentialUnavailable,
	ProviderReasonCredentialRejected,
	ProviderReasonCredentialExpired,
	ProviderReasonInsufficientScope,
	ProviderReasonRateLimited,
	ProviderReasonRequestTimeout,
	ProviderReasonTransportFailed,
	ProviderReasonResponseInvalid,
}

// Valid reports whether the reason is one of the defined safe reason codes.
func (r ProviderReason) Valid() bool { return slices.Contains(providerReasons, r) }

// String returns the wire value of the reason.
func (r ProviderReason) String() string { return string(r) }

// ProviderReasons returns a caller-owned copy of every defined reason code.
func ProviderReasons() []ProviderReason { return slices.Clone(providerReasons) }

// ProviderAttempt records one terminal provider acquisition attempt. It holds
// only values that are safe to log, to serve, and to retain.
type ProviderAttempt struct {
	// ProviderID names the attempted provider.
	ProviderID catalogs.ProviderID

	// Outcome is the terminal state of the attempt.
	Outcome ProviderOutcome

	// Reason explains a skip or a failure. It is empty for a success.
	Reason ProviderReason

	// Requested reports whether the attempt sent a provider request. A skip
	// for a missing credential never sends one.
	Requested bool

	// StartedAt and CompletedAt bound the attempt.
	StartedAt   time.Time
	CompletedAt time.Time

	// Records is the number of accepted provider records.
	Records int
}

// Validate checks that the attempt carries a defined outcome and a defined
// reason for every state other than success.
func (a ProviderAttempt) Validate() error {
	if a.ProviderID == "" {
		return &errors.ValidationError{Field: "provider_attempt.provider_id", Message: "is required"}
	}
	if !a.Outcome.Valid() {
		return &errors.ValidationError{
			Field: "provider_attempt.outcome", Value: a.Outcome, Message: "is not a defined outcome",
		}
	}
	if a.Outcome == ProviderOutcomeSucceeded {
		if a.Reason != "" {
			return &errors.ValidationError{
				Field: "provider_attempt.reason", Value: a.Reason, Message: "must be empty for a success",
			}
		}
		return nil
	}
	if !a.Reason.Valid() {
		return &errors.ValidationError{
			Field: "provider_attempt.reason", Value: a.Reason, Message: "is not a defined safe reason code",
		}
	}
	if a.Outcome == ProviderOutcomeSkippedNotConfigured && a.Requested {
		return &errors.ValidationError{
			Field:   "provider_attempt.requested",
			Value:   a.Requested,
			Message: "a skipped provider sends no request",
		}
	}
	return nil
}

// ClassifyProviderReason maps one acquisition error onto a safe reason code.
// It reads typed error structure first, so provider message text reaches the
// result only through the final transport classification.
func ClassifyProviderReason(err error) ProviderReason {
	if err == nil {
		return ""
	}
	var authenticationErr *errors.AuthenticationError
	if stderrors.As(err, &authenticationErr) {
		return ProviderReasonCredentialUnavailable
	}
	var timeoutErr *errors.TimeoutError
	if stderrors.As(err, &timeoutErr) {
		return ProviderReasonRequestTimeout
	}
	var apiErr *errors.APIError
	if stderrors.As(err, &apiErr) {
		return classifyStatus(apiErr.StatusCode)
	}
	var parseErr *errors.ParseError
	if stderrors.As(err, &parseErr) {
		return ProviderReasonResponseInvalid
	}
	var configErr *errors.ConfigError
	if stderrors.As(err, &configErr) {
		return ProviderReasonCredentialReferenceInvalid
	}
	var validationErr *errors.ValidationError
	if stderrors.As(err, &validationErr) {
		return ProviderReasonCredentialReferenceInvalid
	}
	if stderrors.Is(err, context.DeadlineExceeded) || timedOut(err) {
		return ProviderReasonRequestTimeout
	}
	return ProviderReasonTransportFailed
}

func classifyStatus(status int) ProviderReason {
	switch status {
	case 401:
		return ProviderReasonCredentialRejected
	case 403:
		return ProviderReasonInsufficientScope
	case 408:
		return ProviderReasonRequestTimeout
	case 429:
		return ProviderReasonRateLimited
	}
	switch {
	case status >= 500:
		return ProviderReasonTransportFailed
	case status >= 400:
		return ProviderReasonCredentialRejected
	default:
		return ProviderReasonResponseInvalid
	}
}

func timedOut(err error) bool {
	var netErr net.Error
	if stderrors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}
