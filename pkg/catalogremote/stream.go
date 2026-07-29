package catalogremote

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// EventStreamMediaType identifies the catalog publication SSE stream.
	EventStreamMediaType = "text/event-stream"
	maxEventLineBytes    = 64 << 10
)

// StreamEvent is one parsed SSE publication or comment activity frame.
// Comments establish transport activity but never contain publication data.
type StreamEvent struct {
	Publication *Publication
	Comment     string
	EventID     string
}

// EventStream is one caller-owned catalog publication stream.
type EventStream struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
}

// OpenEventStream opens the publication stream. lastEventID is sent as the
// standard Last-Event-ID request header when nonempty.
func (c *Client) OpenEventStream(
	ctx context.Context,
	lastEventID string,
) (*EventStream, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	target := *c.baseURL
	target.Path = path.Join(strings.TrimSuffix(c.baseURL.Path, "/"), EventStreamPath)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, errors.WrapResource(
			"create",
			"remote catalog event stream",
			target.String(),
			err,
		)
	}
	request.Header.Set("Accept", EventStreamMediaType)
	if lastEventID != "" {
		request.Header.Set("Last-Event-ID", lastEventID)
	}

	// Fetch timeouts must not bound a healthy long-lived stream. Stream
	// liveness is owned by the reactive subscriber and its caller context.
	streamClient := *c.httpClient
	streamClient.Timeout = 0
	response, err := streamClient.Do(request)
	if err != nil {
		return nil, &errors.APIError{
			Provider: "starmap-server",
			Endpoint: target.String(),
			Message:  "event stream request failed",
			Err:      err,
		}
	}
	if err := c.verifyPublisher(response); err != nil {
		_ = response.Body.Close()
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		defer func() { _ = response.Body.Close() }()
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, &errors.APIError{
			Provider:   "starmap-server",
			Endpoint:   target.String(),
			StatusCode: response.StatusCode,
			Message:    "unexpected event stream response status",
		}
	}
	actualMediaType, _, mediaErr := mime.ParseMediaType(
		response.Header.Get("Content-Type"),
	)
	if mediaErr != nil || actualMediaType != EventStreamMediaType {
		_ = response.Body.Close()
		return nil, &errors.ValidationError{
			Field:   "catalog_remote.stream_content_type",
			Value:   response.Header.Get("Content-Type"),
			Message: "does not match " + EventStreamMediaType,
		}
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 4096), maxEventLineBytes)
	return &EventStream{body: response.Body, scanner: scanner}, nil
}

// Next returns the next complete publication or comment activity frame.
func (s *EventStream) Next() (StreamEvent, error) {
	if s == nil || s.scanner == nil {
		return StreamEvent{}, &errors.ValidationError{
			Field: "catalog_remote.event_stream", Message: "is required",
		}
	}

	var frame streamFrame
	for s.scanner.Scan() {
		line := strings.TrimSuffix(s.scanner.Text(), "\r")
		if line == "" {
			if frame.empty() {
				continue
			}
			return frame.event()
		}
		if strings.HasPrefix(line, ":") {
			if frame.hasFields() {
				return StreamEvent{}, streamParseError(
					"comment cannot share a frame with event fields",
				)
			}
			comment := strings.TrimSpace(strings.TrimPrefix(line, ":"))
			if frame.comment != "" {
				frame.comment += "\n"
			}
			frame.comment += comment
			continue
		}

		field, value, found := strings.Cut(line, ":")
		if !found {
			value = ""
		} else {
			value = strings.TrimPrefix(value, " ")
		}
		switch field {
		case "event":
			if frame.eventType != "" {
				return StreamEvent{}, streamParseError("duplicate event field")
			}
			frame.eventType = value
		case "id":
			if frame.eventID != "" {
				return StreamEvent{}, streamParseError("duplicate id field")
			}
			frame.eventID = value
		case "data":
			frame.data = append(frame.data, value)
		default:
			return StreamEvent{}, streamParseError(
				"unsupported SSE field " + strconv.Quote(field),
			)
		}
	}
	if err := s.scanner.Err(); err != nil {
		return StreamEvent{}, errors.WrapIO("read", "catalog publication stream", err)
	}
	if !frame.empty() {
		return frame.event()
	}
	return StreamEvent{}, io.EOF
}

// Close closes the underlying response body and unblocks Next.
func (s *EventStream) Close() error {
	if s == nil || s.body == nil {
		return nil
	}
	return s.body.Close()
}

type streamFrame struct {
	eventType string
	eventID   string
	data      []string
	comment   string
}

func (f streamFrame) empty() bool {
	return !f.hasFields() && f.comment == ""
}

func (f streamFrame) hasFields() bool {
	return f.eventType != "" || f.eventID != "" || len(f.data) != 0
}

func (f streamFrame) event() (StreamEvent, error) {
	if f.comment != "" {
		if f.hasFields() {
			return StreamEvent{}, streamParseError(
				"comment cannot share a frame with event fields",
			)
		}
		return StreamEvent{Comment: f.comment}, nil
	}
	if f.eventType != CatalogPublishedEvent {
		return StreamEvent{}, streamParseError(
			"event type must be " + CatalogPublishedEvent,
		)
	}
	if f.eventID == "" {
		return StreamEvent{}, streamParseError("publication event id is required")
	}
	sequence, err := strconv.ParseUint(f.eventID, 10, 64)
	if err != nil || sequence == 0 {
		return StreamEvent{}, streamParseError(
			"publication event id must be a positive integer",
		)
	}
	if len(f.data) == 0 {
		return StreamEvent{}, streamParseError("publication data is required")
	}

	decoder := json.NewDecoder(bytes.NewBufferString(strings.Join(f.data, "\n")))
	decoder.DisallowUnknownFields()
	var publication Publication
	if err := decoder.Decode(&publication); err != nil {
		return StreamEvent{}, errors.WrapParse(
			"json",
			"SSE catalog publication",
			err,
		)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return StreamEvent{}, err
	}
	if err := validateGenerationID(publication.GenerationID); err != nil {
		return StreamEvent{}, err
	}
	if publication.Sequence != sequence {
		return StreamEvent{}, streamParseError(
			"publication sequence does not match event id",
		)
	}
	return StreamEvent{
		Publication: &publication,
		EventID:     f.eventID,
	}, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return streamParseError("publication contains trailing JSON")
		}
		return errors.WrapParse("json", "SSE catalog publication", err)
	}
	return nil
}

func streamParseError(message string) error {
	return &errors.ValidationError{
		Field: "catalog_remote.event_stream", Message: message,
	}
}
