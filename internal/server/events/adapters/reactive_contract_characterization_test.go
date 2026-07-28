package adapters

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/internal/server/events"
	"github.com/agentstation/starmap/internal/server/sse"
)

// TestF017F034CharacterizationSSEIgnoresResumeIDAndReusesSecondTimestamp pins
// two current transport defects over a real SSE connection. Last-Event-ID does
// not alter the handshake or replay state, and two distinct publications in
// one second receive the same event ID. P7 must use committed generation
// identity plus monotonic sequence and mandatory reconnect catch-up.
func TestF017F034CharacterizationSSEIgnoresResumeIDAndReusesSecondTimestamp(t *testing.T) {
	logger := zerolog.Nop()
	broadcaster := sse.NewBroadcaster(&logger)
	runContext, stop := context.WithCancel(context.Background())
	go broadcaster.Run(runContext)

	server := httptest.NewServer(broadcaster)
	requestContext, cancelRequest := context.WithCancel(context.Background())
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	request.Header.Set("Last-Event-ID", "committed-generation-41")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	reader := bufio.NewReader(response.Body)

	initial := readCharacterizationSSEFrame(t, reader)
	if initial.event != "connected" || initial.id != "" {
		t.Fatalf("Last-Event-ID changed initial frame: %#v", initial)
	}

	deadline := time.Now().Add(time.Second)
	for broadcaster.ClientCount() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if broadcaster.ClientCount() != 1 {
		t.Fatal("SSE client was not registered")
	}

	subscriber := NewSSESubscriber(broadcaster)
	secondTimestamp := time.Unix(1_800_000_000, 100).UTC()
	for sequence := 1; sequence <= 2; sequence++ {
		if err := subscriber.Send(events.Event{
			Type:      events.CatalogPublished,
			Timestamp: secondTimestamp,
			Data:      map[string]any{"sequence": sequence},
		}); err != nil {
			t.Fatalf("Send(%d): %v", sequence, err)
		}
	}

	first := readCharacterizationSSEFrame(t, reader)
	second := readCharacterizationSSEFrame(t, reader)
	wantID := "1800000000"
	if first.id != wantID || second.id != wantID {
		t.Fatalf("F-034 characterization changed: event IDs = %q/%q, want duplicate %q", first.id, second.id, wantID)
	}
	if first.event != string(events.CatalogPublished) || second.event != string(events.CatalogPublished) {
		t.Fatalf("publication event types = %q/%q", first.event, second.event)
	}

	cancelRequest()
	_ = response.Body.Close()
	server.Close()
	stop()
}

type characterizationSSEFrame struct {
	event string
	id    string
	data  string
}

func readCharacterizationSSEFrame(t testing.TB, reader *bufio.Reader) characterizationSSEFrame {
	t.Helper()
	var frame characterizationSSEFrame
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				t.Fatalf("unexpected EOF reading SSE frame: %#v", frame)
			}
			t.Fatalf("ReadString: %v", err)
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			return frame
		}
		if value, found := strings.CutPrefix(line, "event: "); found {
			frame.event = value
		}
		if value, found := strings.CutPrefix(line, "id: "); found {
			frame.id = value
		}
		if value, found := strings.CutPrefix(line, "data: "); found {
			frame.data = value
		}
	}
}
