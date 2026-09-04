package middleware

import (
	stderrors "errors"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// RouteTimeout describes the write bound of one route group. A streaming route
// needs a different bound from an ordinary JSON route, so the policy splits
// them instead of forcing one server-wide value on both.
type RouteTimeout struct {
	// Path matches the request path. An exact entry matches the whole path, and
	// a prefix entry matches every path below it.
	Path string

	// Prefix selects prefix matching instead of whole-path matching.
	Prefix bool

	// Write bounds one response write. A zero value clears the deadline, so a
	// streaming handler keeps its own per-chunk or per-frame bound.
	Write time.Duration
}

// matches reports whether the route entry covers one request path.
func (t RouteTimeout) matches(path string) bool {
	if t.Prefix {
		return strings.HasPrefix(path, t.Path)
	}
	return path == t.Path
}

// RouteTimeouts applies the write bound of the first matching route before the
// handler runs. A route without an entry keeps the server-wide write timeout.
//
// The middleware runs before the handlers, so a streaming handler never races
// the server-wide bound that a short JSON response needs.
func RouteTimeouts(routes []RouteTimeout, logger *zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, found := matchRoute(routes, r.URL.Path)
			if !found {
				next.ServeHTTP(w, r)
				return
			}
			applyWriteDeadline(w, route.Write, logger)
			next.ServeHTTP(w, r)
		})
	}
}

// matchRoute returns the first route entry that covers one request path.
func matchRoute(routes []RouteTimeout, path string) (RouteTimeout, bool) {
	for _, route := range routes {
		if route.matches(path) {
			return route, true
		}
	}
	return RouteTimeout{}, false
}

// applyWriteDeadline sets or clears the connection write deadline. A recorder
// that supports no deadline keeps the response, because a test writer needs no
// connection.
func applyWriteDeadline(w http.ResponseWriter, write time.Duration, logger *zerolog.Logger) {
	deadline := time.Time{}
	if write > 0 {
		deadline = time.Now().Add(write)
	}
	err := http.NewResponseController(w).SetWriteDeadline(deadline)
	if err == nil || stderrors.Is(err, http.ErrNotSupported) {
		return
	}
	if logger != nil {
		logger.Warn().Err(err).Msg("Route write deadline was refused")
	}
}
