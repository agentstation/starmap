package github

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// RateLimitWarnPercent is the used share that raises a budget warning.
	RateLimitWarnPercent = 80

	// wholePercent converts a ratio to a percentage.
	wholePercent = 100

	headerRateLimit     = "X-RateLimit-Limit"
	headerRateUsed      = "X-RateLimit-Used"
	headerRateRemaining = "X-RateLimit-Remaining"
	headerRateReset     = "X-RateLimit-Reset"
	headerRetryAfter    = "Retry-After"
)

// RateLimitBudget reports the GitHub request budget that the response headers
// declare, together with the request count of the cycle that read them.
//
// GitHub sends the budget on every REST reply, so one cycle always knows how
// much of its hourly allowance remains before the next cycle starts.
type RateLimitBudget struct {
	// Observed reports whether the reply carried a usable budget.
	Observed bool

	// Limit is the number of requests that the window allows.
	Limit int

	// Used is the number of requests already spent in the window.
	Used int

	// Remaining is the number of requests left in the window.
	Remaining int

	// ResetAt is the time the window restarts.
	ResetAt time.Time

	// Requests is the number of requests this refresh cycle sent.
	Requests int
}

// UsedPercent returns the spent share of the window, from zero to one hundred.
// An unobserved or empty budget returns zero.
func (b RateLimitBudget) UsedPercent() int {
	if !b.Observed || b.Limit <= 0 {
		return 0
	}
	return b.Used * wholePercent / b.Limit
}

// Warn reports whether the cycle should warn about the remaining budget.
func (b RateLimitBudget) Warn() bool {
	return b.Observed && b.UsedPercent() >= RateLimitWarnPercent
}

// Exhausted reports whether the window has no request left.
func (b RateLimitBudget) Exhausted() bool {
	return b.Observed && b.Remaining <= 0
}

// parseRateLimit reads the budget that one reply declares. GitHub omits the
// used header on some paths, so an absent value falls back to the difference
// between the limit and the remainder.
func parseRateLimit(header http.Header) RateLimitBudget {
	limit, limitFound := headerInt(header, headerRateLimit)
	remaining, remainingFound := headerInt(header, headerRateRemaining)
	if !limitFound || !remainingFound {
		return RateLimitBudget{}
	}
	used, usedFound := headerInt(header, headerRateUsed)
	if !usedFound {
		used = limit - remaining
	}
	budget := RateLimitBudget{
		Observed:  true,
		Limit:     limit,
		Used:      used,
		Remaining: remaining,
	}
	if reset, found := headerInt(header, headerRateReset); found {
		budget.ResetAt = time.Unix(int64(reset), 0).UTC()
	}
	return budget
}

// retryBoundary returns the hard not-before time that one refusal declares.
// `Retry-After` wins, because it is the explicit instruction. The rate-limit
// reset time is the fallback. A reply with neither header returns false.
func retryBoundary(header http.Header, now time.Time) (time.Time, bool) {
	if value := strings.TrimSpace(header.Get(headerRetryAfter)); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
			return now.Add(time.Duration(seconds) * time.Second).UTC(), true
		}
		if stamp, err := http.ParseTime(value); err == nil {
			return stamp.UTC(), true
		}
	}
	if reset, found := headerInt(header, headerRateReset); found {
		return time.Unix(int64(reset), 0).UTC(), true
	}
	return time.Time{}, false
}

func headerInt(header http.Header, name string) (int, bool) {
	value := strings.TrimSpace(header.Get(name))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
