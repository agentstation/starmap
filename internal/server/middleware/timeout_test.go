package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type deadlineWriter struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (w *deadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	return nil
}

func TestRouteTimeoutsRunBeforeTheHandler(t *testing.T) {
	t.Parallel()

	routes := []RouteTimeout{
		{Path: "/api/v1/updates/stream"},
		{Path: "/api/v1/catalog/generations/", Prefix: true},
		{Path: "/api/v1/models", Write: 3 * time.Second},
	}

	tests := []struct {
		name          string
		path          string
		wantDeadlines int
		wantCleared   bool
	}{
		{
			name:          "the publication stream clears the deadline",
			path:          "/api/v1/updates/stream",
			wantDeadlines: 1,
			wantCleared:   true,
		},
		{
			name:          "a catalog payload clears the deadline",
			path:          "/api/v1/catalog/generations/abc/payload",
			wantDeadlines: 1,
			wantCleared:   true,
		},
		{
			name:          "a bounded route sets its own deadline",
			path:          "/api/v1/models",
			wantDeadlines: 1,
		},
		{
			name:          "an unlisted route keeps the server bound",
			path:          "/api/v1/stats",
			wantDeadlines: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			writer := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
			var observed []time.Time
			handler := RouteTimeouts(routes, nil)(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					// The middleware must finish before the handler writes, so the
					// handler reads the deadlines that already reached the writer.
					observed = append([]time.Time{}, writer.deadlines...)
					w.WriteHeader(http.StatusOK)
				},
			))

			handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, test.path, nil))

			if len(observed) != test.wantDeadlines {
				t.Fatalf(
					"deadlines seen by the handler = %d, want %d",
					len(observed), test.wantDeadlines,
				)
			}
			if test.wantDeadlines == 0 {
				return
			}
			if test.wantCleared != observed[0].IsZero() {
				t.Fatalf("deadline = %s, cleared want %t", observed[0], test.wantCleared)
			}
		})
	}
}
