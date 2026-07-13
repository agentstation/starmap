package sources

import (
	"context"
	stderrors "errors"
	"net/http"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

func TestCollectPages(t *testing.T) {
	pages := map[string]Page[int]{"": {Records: []int{1, 2}, NextCursor: "a"}, "a": {Records: []int{3}}}
	records, err := CollectPages(context.Background(), PaginationPolicy{MaxPages: 3, MaxRecords: 3}, func(_ context.Context, cursor string) (Page[int], error) {
		return pages[cursor], nil
	})
	if err != nil || len(records) != 3 || records[2] != 3 {
		t.Fatalf("CollectPages = (%v, %v)", records, err)
	}
}

func TestCollectPagesRejectsRepeatedCursorAndLimits(t *testing.T) {
	tests := []struct {
		name   string
		policy PaginationPolicy
		fetch  func(context.Context, string) (Page[int], error)
	}{
		{name: "cursor", policy: PaginationPolicy{MaxPages: 2, MaxRecords: 2}, fetch: func(_ context.Context, _ string) (Page[int], error) {
			return Page[int]{NextCursor: "same"}, nil
		}},
		{name: "records", policy: PaginationPolicy{MaxPages: 1, MaxRecords: 1}, fetch: func(context.Context, string) (Page[int], error) {
			return Page[int]{Records: []int{1, 2}}, nil
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := CollectPages(context.Background(), test.policy, test.fetch); err == nil {
				t.Fatal("CollectPages returned nil error")
			}
		})
	}
}

func TestRetryProviderCallHonorsRetryAfterAndStatusPolicy(t *testing.T) {
	now := time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC)
	policy := ProviderRetryPolicy{MaxAttempts: 3, BaseDelay: time.Second, MaxDelay: 10 * time.Second}
	var attempts int
	var delays []time.Duration
	err := retryProviderCall(context.Background(), policy, func(context.Context) (RetryHint, error) {
		attempts++
		if attempts < 3 {
			return RetryHint{StatusCode: http.StatusTooManyRequests, RetryAfter: "2"}, &errors.APIError{StatusCode: http.StatusTooManyRequests}
		}
		return RetryHint{}, nil
	}, func() time.Time { return now }, func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}, func() float64 { return 0.5 })
	if err != nil || attempts != 3 || len(delays) != 2 || delays[0] != 2*time.Second {
		t.Fatalf("retry = attempts %d delays %v err %v", attempts, delays, err)
	}

	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict} {
		attempts = 0
		err = retryProviderCall(context.Background(), policy, func(context.Context) (RetryHint, error) {
			attempts++
			return RetryHint{StatusCode: status}, &errors.APIError{StatusCode: status}
		}, func() time.Time { return now }, func(context.Context, time.Duration) error { return nil }, func() float64 { return 0 })
		if err == nil || attempts != 1 {
			t.Fatalf("status %d retry = attempts %d err %v", status, attempts, err)
		}
	}
}

func TestRetryProviderCallCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := RetryProviderCall(ctx, DefaultProviderRetryPolicy(), func(context.Context) (RetryHint, error) {
		return RetryHint{}, stderrors.New("must not run")
	})
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("RetryProviderCall error = %v", err)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		value string
		want  time.Duration
	}{
		{value: "3", want: 3 * time.Second},
		{value: now.Add(5 * time.Second).Format(http.TimeFormat), want: 5 * time.Second},
	} {
		got, err := ParseRetryAfter(test.value, now)
		if err != nil || got != test.want {
			t.Fatalf("ParseRetryAfter(%q) = (%v, %v), want %v", test.value, got, err, test.want)
		}
	}
}
