package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/constants"
)

// TestCatalogPayloadResetsWriteDeadlinePerChunk proves that one payload
// transfer sets one write deadline per chunk. The deadline then bounds one
// chunk, so a slow reader keeps a large download and a stalled reader still
// releases the connection.
func TestCatalogPayloadResetsWriteDeadlinePerChunk(t *testing.T) {
	client, err := starmap.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	generationID := client.CurrentGenerationID()
	generation, err := client.Generation(context.Background(), generationID)
	if err != nil {
		t.Fatalf("Generation(%s): %v", generationID, err)
	}
	if len(generation.Payload) <= constants.CatalogPayloadChunkBytes {
		t.Fatalf(
			"payload = %d bytes, want more than one %d byte chunk",
			len(generation.Payload), constants.CatalogPayloadChunkBytes,
		)
	}

	handlers := &Handlers{app: &testApplication{
		StarmapFunc: func(...starmap.Option) (*starmap.Client, error) {
			return client, nil
		},
	}}
	recorder := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/catalog/generations/"+generationID+"/payload",
		nil,
	)
	start := time.Now()

	handlers.HandleCatalogPayload(recorder, request, generationID)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !bytes.Equal(recorder.Body.Bytes(), generation.Payload) {
		t.Fatalf("body = %d bytes, want the %d byte payload",
			recorder.Body.Len(), len(generation.Payload))
	}
	chunks := (len(generation.Payload) + constants.CatalogPayloadChunkBytes - 1) /
		constants.CatalogPayloadChunkBytes
	if len(recorder.deadlines) != chunks {
		t.Fatalf("write deadlines = %d, want one per chunk (%d)",
			len(recorder.deadlines), chunks)
	}
	for index, deadline := range recorder.deadlines {
		if deadline.Before(start.Add(constants.CatalogPayloadChunkTimeout)) {
			t.Fatalf("deadline %d = %s, want a full chunk budget from the chunk write",
				index, deadline)
		}
	}
}
