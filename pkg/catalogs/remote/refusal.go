package remote

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// headerRetryAfter is the standard refusal boundary header.
const headerRetryAfter = "Retry-After"

// RefusalError reports that the publisher refused a request and named the
// earliest time a client may try again. The boundary is a hard floor: a client
// waits for it instead of applying its own backoff, and it adds jitter so a
// fleet does not return at one instant.
type RefusalError struct {
	// StatusCode is the refusal status, such as 429 or 503.
	StatusCode int

	// Resource is the safe label of the refused resource. It names no URL.
	Resource string

	// NotBefore is the earliest time the publisher accepts another request.
	NotBefore time.Time

	// Err is the underlying transport or status error.
	Err error
}

// Error returns the safe refusal text. It names the resource and the status,
// never the endpoint.
func (e *RefusalError) Error() string {
	message := "remote catalog publisher refused " + e.Resource +
		" with status " + strconv.Itoa(e.StatusCode)
	if !e.NotBefore.IsZero() {
		message += " until " + e.NotBefore.UTC().Format(time.RFC3339)
	}
	return message
}

// Unwrap returns the underlying error.
func (e *RefusalError) Unwrap() error { return e.Err }

// RetryBoundary returns the hard not-before boundary a reply declared. It
// accepts the delta-seconds and the HTTP-date forms of Retry-After.
func RetryBoundary(header http.Header, now time.Time) (time.Time, bool) {
	if header == nil {
		return time.Time{}, false
	}
	value := strings.TrimSpace(header.Get(headerRetryAfter))
	if value == "" {
		return time.Time{}, false
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return now.Add(time.Duration(seconds) * time.Second).UTC(), true
	}
	if stamp, err := http.ParseTime(value); err == nil {
		return stamp.UTC(), true
	}
	return time.Time{}, false
}

// refused reports whether a status is a refusal a client may retry after the
// declared boundary.
func refused(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == http.StatusServiceUnavailable
}

// asRefusal wraps err when the reply refused the request and named a boundary.
// A refusal without a boundary stays an ordinary status error, because the
// client then owns the delay.
func asRefusal(response *http.Response, resource string, now time.Time, err error) error {
	if response == nil || !refused(response.StatusCode) {
		return err
	}
	boundary, found := RetryBoundary(response.Header, now)
	if !found {
		return err
	}
	return &RefusalError{
		StatusCode: response.StatusCode,
		Resource:   resource,
		NotBefore:  boundary,
		Err:        err,
	}
}
