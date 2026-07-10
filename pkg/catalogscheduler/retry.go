package catalogscheduler

import (
	"context"
	stderrors "errors"
	"math"
	rand "math/rand/v2"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sync"
)

// RetryClass is the scheduling disposition of a synchronization error.
type RetryClass string

const (
	// RetryClassTransient permits a bounded retry.
	RetryClassTransient RetryClass = "transient"
	// RetryClassPermanent ends the attempt sequence immediately.
	RetryClassPermanent RetryClass = "permanent"
)

// RetryPolicy defines bounded exponential retry with proportional jitter.
type RetryPolicy struct {
	MaxAttempts    int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	JitterFraction float64
}

// DefaultRetryPolicy returns one immediate attempt and no implicit retries.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, MaxDelay: 30 * time.Second}
}

// Validate verifies a finite bounded retry policy.
func (p RetryPolicy) Validate() error {
	if p.MaxAttempts <= 0 {
		return &errors.ValidationError{Field: "catalog_scheduler.retry.max_attempts", Value: p.MaxAttempts, Message: "must be positive"}
	}
	if p.BaseDelay <= 0 || p.MaxDelay <= 0 || p.BaseDelay > p.MaxDelay {
		return &errors.ValidationError{Field: "catalog_scheduler.retry.delay", Value: p, Message: "delays must be positive and base must not exceed maximum"}
	}
	if math.IsNaN(p.JitterFraction) || math.IsInf(p.JitterFraction, 0) || p.JitterFraction < 0 || p.JitterFraction > 1 {
		return &errors.ValidationError{Field: "catalog_scheduler.retry.jitter_fraction", Value: p.JitterFraction, Message: "must be between zero and one"}
	}
	return nil
}

// RunnerOption configures deployment retry policy.
type RunnerOption func(*Runner) error

// WithRetryPolicy configures bounded typed retry for one runner.
func WithRetryPolicy(policy RetryPolicy) RunnerOption {
	return func(runner *Runner) error {
		if err := policy.Validate(); err != nil {
			return err
		}
		runner.retry = policy
		return nil
	}
}

// WithRunLedger enables durable run auditing and base-generation capture.
func WithRunLedger(ledger RunLedger, current CurrentGenerationReader) RunnerOption {
	return func(runner *Runner) error {
		if isNilInterface(ledger) {
			return &errors.ValidationError{Field: "catalog_scheduler.run_ledger", Message: "is required"}
		}
		if isNilInterface(current) {
			return &errors.ValidationError{Field: "catalog_scheduler.current_generation_reader", Message: "is required"}
		}
		runner.ledger = ledger
		runner.current = current
		return nil
	}
}

// WithFreshnessMonitor records successful source observations even when the
// canonical catalog payload does not change.
func WithFreshnessMonitor(monitor *FreshnessMonitor) RunnerOption {
	return func(runner *Runner) error {
		if monitor == nil {
			return &errors.ValidationError{Field: "catalog_scheduler.freshness_monitor", Message: "is required"}
		}
		runner.freshness = monitor
		return nil
	}
}

// ClassifyRetry permits only explicitly transient failures.
func ClassifyRetry(err error) RetryClass {
	if err == nil {
		return RetryClassPermanent
	}
	var configError *errors.ConfigError
	var parseError *errors.ParseError
	var authError *errors.AuthenticationError
	var dependencyError *errors.DependencyError
	if stderrors.As(err, &configError) || stderrors.As(err, &parseError) ||
		stderrors.As(err, &authError) || stderrors.As(err, &dependencyError) {
		return RetryClassPermanent
	}
	if stderrors.Is(err, context.Canceled) || stderrors.Is(err, errors.ErrCanceled) ||
		stderrors.Is(err, errors.ErrInvalidInput) || stderrors.Is(err, errors.ErrConflict) ||
		stderrors.Is(err, errors.ErrAPIKeyRequired) || stderrors.Is(err, errors.ErrAPIKeyInvalid) {
		return RetryClassPermanent
	}
	if stderrors.Is(err, context.DeadlineExceeded) || stderrors.Is(err, errors.ErrTimeout) ||
		stderrors.Is(err, errors.ErrRateLimited) || stderrors.Is(err, errors.ErrProviderUnavailable) {
		return RetryClassTransient
	}
	var apiError *errors.APIError
	if stderrors.As(err, &apiError) {
		switch apiError.StatusCode {
		case http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests:
			return RetryClassTransient
		}
		if apiError.StatusCode >= http.StatusInternalServerError {
			return RetryClassTransient
		}
		return RetryClassPermanent
	}
	var networkError net.Error
	if stderrors.As(err, &networkError) && networkError.Timeout() {
		return RetryClassTransient
	}
	if stderrors.Is(err, syscall.ECONNRESET) || stderrors.Is(err, syscall.ECONNREFUSED) ||
		stderrors.Is(err, syscall.EPIPE) || stderrors.Is(err, syscall.ETIMEDOUT) ||
		stderrors.Is(err, syscall.ENETUNREACH) || stderrors.Is(err, syscall.EHOSTUNREACH) {
		return RetryClassTransient
	}
	return RetryClassPermanent
}

// RunScheduledOnce applies bounded pre-acquisition jitter, then executes the
// same leased attempt sequence as RunOnce. Manual callers use RunOnce directly.
func (r *Runner) RunScheduledOnce(ctx context.Context, jitterWindow time.Duration, options ...sync.Option) (RunResult, error) {
	if jitterWindow < 0 {
		return RunResult{}, &errors.ValidationError{Field: "catalog_scheduler.jitter_window", Value: jitterWindow, Message: "must not be negative"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if jitterWindow > 0 {
		delay := time.Duration(clampUnit(r.random()) * float64(jitterWindow))
		if err := r.sleep(ctx, delay); err != nil {
			return RunResult{Status: RunStatusFailed, LeaseOwner: r.request.Owner}, err
		}
	}
	return r.run(ctx, TriggerScheduled, options...)
}

func (p RetryPolicy) delay(retry int, random float64) time.Duration {
	random = clampUnit(random)
	delay := float64(p.BaseDelay) * math.Pow(2, float64(retry-1))
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}
	if p.JitterFraction > 0 {
		factor := 1 - p.JitterFraction + 2*p.JitterFraction*random
		delay *= factor
	}
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}
	if delay < 0 {
		delay = 0
	}
	return time.Duration(delay)
}

type sleepFunc func(context.Context, time.Duration) error
type randomFunc func() float64

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func randomFloat64() float64 {
	//nolint:gosec // Traffic-spreading jitter is not a security boundary.
	return rand.Float64()
}

func clampUnit(value float64) float64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
