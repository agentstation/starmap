package sources

import (
	"context"
	stderrors "errors"
	"math"
	rand "math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// ProviderRetryPolicy defines bounded provider-call retry with jitter.
type ProviderRetryPolicy struct {
	MaxAttempts    int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	JitterFraction float64
}

// DefaultProviderRetryPolicy returns the reviewed provider-call defaults.
func DefaultProviderRetryPolicy() ProviderRetryPolicy {
	return ProviderRetryPolicy{MaxAttempts: 3, BaseDelay: 500 * time.Millisecond, MaxDelay: 20 * time.Second, JitterFraction: 0.2}
}

// Validate verifies a finite provider retry policy.
func (p ProviderRetryPolicy) Validate() error {
	if p.MaxAttempts <= 0 || p.BaseDelay <= 0 || p.MaxDelay < p.BaseDelay {
		return &errors.ValidationError{Field: "provider_retry", Value: p, Message: "attempts and delays must be positive and bounded"}
	}
	if math.IsNaN(p.JitterFraction) || math.IsInf(p.JitterFraction, 0) || p.JitterFraction < 0 || p.JitterFraction > 1 {
		return &errors.ValidationError{Field: "provider_retry.jitter_fraction", Value: p.JitterFraction, Message: "must be between zero and one"}
	}
	return nil
}

// RetryHint carries response metadata without retaining a response body.
type RetryHint struct {
	StatusCode int
	RetryAfter string
}

// RetryProviderCall applies bounded retry to one context-aware provider operation.
func RetryProviderCall(ctx context.Context, policy ProviderRetryPolicy, operation func(context.Context) (RetryHint, error)) error {
	return retryProviderCall(ctx, policy, operation, time.Now, sleepProviderRetry, rand.Float64)
}

func retryProviderCall(
	ctx context.Context,
	policy ProviderRetryPolicy,
	operation func(context.Context) (RetryHint, error),
	now func() time.Time,
	sleep func(context.Context, time.Duration) error,
	random func() float64,
) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	if operation == nil {
		return &errors.ValidationError{Field: "provider_retry.operation", Message: "is required"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		hint, err := operation(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == policy.MaxAttempts || !retryableProviderFailure(hint.StatusCode, err) {
			return err
		}
		delay, retryAfterErr := ParseRetryAfter(hint.RetryAfter, now())
		if retryAfterErr != nil {
			return retryAfterErr
		}
		if delay == 0 {
			delay = providerBackoff(policy, attempt, random())
		}
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
		if err := sleep(ctx, delay); err != nil {
			return err
		}
	}
	return lastErr
}

// ParseRetryAfter parses delta-seconds or an HTTP-date into a non-negative delay.
func ParseRetryAfter(value string, now time.Time) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds < 0 {
			return 0, &errors.ValidationError{Field: "retry_after", Value: value, Message: "delta seconds must not be negative"}
		}
		return time.Duration(seconds) * time.Second, nil
	}
	parsed, err := http.ParseTime(value)
	if err != nil {
		return 0, errors.WrapParse("http-date", "Retry-After", err)
	}
	if !parsed.After(now) {
		return 0, nil
	}
	return parsed.Sub(now), nil
}

func retryableProviderFailure(statusCode int, err error) bool {
	var apiError *errors.APIError
	if statusCode == 0 && stderrors.As(err, &apiError) {
		statusCode = apiError.StatusCode
	}
	return statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooEarly ||
		statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func providerBackoff(policy ProviderRetryPolicy, attempt int, random float64) time.Duration {
	delay := float64(policy.BaseDelay) * math.Pow(2, float64(attempt-1))
	if delay > float64(policy.MaxDelay) {
		delay = float64(policy.MaxDelay)
	}
	if random < 0 {
		random = 0
	} else if random > 1 {
		random = 1
	}
	delay *= 1 - policy.JitterFraction + 2*policy.JitterFraction*random
	return time.Duration(delay)
}

func sleepProviderRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Page is one cursor-addressed source page.
type Page[T any] struct {
	Records    []T
	NextCursor string
}

// PaginationPolicy bounds provider pagination work.
type PaginationPolicy struct {
	MaxPages   int
	MaxRecords int
}

// CollectPages reads a complete bounded cursor sequence.
func CollectPages[T any](ctx context.Context, policy PaginationPolicy, fetch func(context.Context, string) (Page[T], error)) ([]T, error) {
	if policy.MaxPages <= 0 || policy.MaxRecords <= 0 {
		return nil, &errors.ValidationError{Field: "pagination.policy", Value: policy, Message: "page and record limits must be positive"}
	}
	if fetch == nil {
		return nil, &errors.ValidationError{Field: "pagination.fetch", Message: "is required"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	records := make([]T, 0)
	cursor := ""
	seen := make(map[string]struct{})
	for pageNumber := 1; pageNumber <= policy.MaxPages; pageNumber++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page, err := fetch(ctx, cursor)
		if err != nil {
			return nil, err
		}
		if len(records)+len(page.Records) > policy.MaxRecords {
			return nil, &errors.ValidationError{Field: "pagination.records", Value: len(records) + len(page.Records), Message: "exceeds maximum record count"}
		}
		records = append(records, page.Records...)
		if page.NextCursor == "" {
			return records, nil
		}
		if page.NextCursor == cursor {
			return nil, &errors.ConflictError{Resource: "pagination cursor", Expected: cursor, Actual: page.NextCursor, Message: "cursor did not advance"}
		}
		if _, found := seen[page.NextCursor]; found {
			return nil, &errors.ConflictError{Resource: "pagination cursor", Actual: page.NextCursor, Message: "cursor repeated"}
		}
		seen[page.NextCursor] = struct{}{}
		cursor = page.NextCursor
	}
	return nil, &errors.ValidationError{Field: "pagination.pages", Value: policy.MaxPages, Message: "source did not terminate within maximum pages"}
}
