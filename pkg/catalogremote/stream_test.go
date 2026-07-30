package catalogremote

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEventStreamParsesCommentsAndStablePublication(t *testing.T) {
	t.Parallel()

	var lastEventID string
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			lastEventID = request.Header.Get("Last-Event-ID")
			writer.Header().Set("Content-Type", EventStreamMediaType)
			_, _ = fmt.Fprint(
				writer,
				": heartbeat\n\n"+
					"id: 42\n"+
					"event: catalog.published\n"+
					"data: {\"generation_id\":\"generation-42\",\"sequence\":42}\n\n",
			)
		},
	))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client(), 1)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	stream, err := client.OpenEventStream(context.Background(), "41")
	if err != nil {
		t.Fatalf("OpenEventStream: %v", err)
	}
	defer func() { _ = stream.Close() }()

	comment, err := stream.Next()
	if err != nil {
		t.Fatalf("read heartbeat: %v", err)
	}
	if comment.Comment != "heartbeat" || comment.Publication != nil ||
		comment.EventID != "" {
		t.Fatalf("comment frame = %#v", comment)
	}
	publication, err := stream.Next()
	if err != nil {
		t.Fatalf("read publication: %v", err)
	}
	if publication.Publication == nil ||
		publication.Publication.GenerationID != "generation-42" ||
		publication.Publication.Sequence != 42 ||
		publication.EventID != "42" {
		t.Fatalf("publication frame = %#v", publication)
	}
	if lastEventID != "41" {
		t.Fatalf("Last-Event-ID = %q, want 41", lastEventID)
	}
}

func TestEventStreamRejectsSequenceMismatch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", EventStreamMediaType)
			_, _ = fmt.Fprint(
				writer,
				"id: 7\n"+
					"event: catalog.published\n"+
					"data: {\"generation_id\":\"generation-7\",\"sequence\":8}\n\n",
			)
		},
	))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client(), 1)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	stream, err := client.OpenEventStream(context.Background(), "")
	if err != nil {
		t.Fatalf("OpenEventStream: %v", err)
	}
	defer func() { _ = stream.Close() }()
	if event, err := stream.Next(); err == nil {
		t.Fatalf("sequence mismatch accepted: %#v", event)
	}
}

func TestEventStreamRejectsUnsafeGenerationIdentity(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", EventStreamMediaType)
			_, _ = fmt.Fprint(
				writer,
				"id: 7\n"+
					"event: catalog.published\n"+
					"data: {\"generation_id\":\"..\",\"sequence\":7}\n\n",
			)
		},
	))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client(), 1)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	stream, err := client.OpenEventStream(context.Background(), "")
	if err != nil {
		t.Fatalf("OpenEventStream: %v", err)
	}
	defer func() { _ = stream.Close() }()
	if event, err := stream.Next(); err == nil {
		t.Fatalf("unsafe generation identity accepted: %#v", event)
	}
}

func TestEventStreamRejectsOversizedCumulativeFrame(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", EventStreamMediaType)
			for range 40 {
				_, _ = fmt.Fprintf(
					writer,
					"data: %s\n",
					strings.Repeat("x", 8<<10),
				)
			}
			_, _ = fmt.Fprint(writer, "\n")
		},
	))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client(), 1)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	stream, err := client.OpenEventStream(context.Background(), "")
	if err != nil {
		t.Fatalf("OpenEventStream: %v", err)
	}
	defer func() { _ = stream.Close() }()
	if event, err := stream.Next(); err == nil {
		t.Fatalf("oversized cumulative frame accepted: %#v", event)
	}
}

func TestEventStreamRejectsInvalidLastEventIDBeforeNetwork(t *testing.T) {
	t.Parallel()

	client, err := NewClient("http://127.0.0.1:1", nil, 1)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if stream, err := client.OpenEventStream(
		context.Background(),
		"not-a-sequence",
	); err == nil || stream != nil {
		t.Fatalf("OpenEventStream = (%#v, %v), want validation failure", stream, err)
	}
}
