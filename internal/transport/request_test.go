package transport

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestSchemaDriftMutationMatrix(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantErr    bool
		wantModels int
	}{
		{name: "valid", payload: `{"data":[{"id":"model"}]}`, wantModels: 1},
		{name: "missing collection", payload: `{}`},
		{name: "renamed collection", payload: `{"items":[]}`},
		{name: "null collection", payload: `{"data":null}`},
		{name: "wrong collection type", payload: `{"data":{}}`, wantErr: true},
		{name: "unknown additive field", payload: `{"data":[{"id":"model","new_capability":true}],"new_page":1}`, wantModels: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var target struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			err := DecodeResponse(jsonResponse(test.payload), &target)
			if test.wantErr && err == nil {
				t.Fatal("DecodeResponse returned nil error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("DecodeResponse: %v", err)
			}
			if len(target.Data) != test.wantModels {
				t.Fatalf("models = %d, want %d", len(target.Data), test.wantModels)
			}
		})
	}
}

func TestPayloadLimitReturnsTypedErrorWithoutPanic(t *testing.T) {
	payload := strings.Repeat("x", constants.MaxSourcePayloadBytes+1)
	err := DecodeResponse(jsonResponse(payload), &struct{}{})
	var validationErr *pkgerrors.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %T %v, want *errors.ValidationError", err, err)
	}
}

func TestNonJSONFailureRemainsAPIErrorNotSchemaDrift(t *testing.T) {
	response := jsonResponse(`<html>upstream unavailable</html>`)
	response.StatusCode = http.StatusServiceUnavailable
	err := DecodeResponse(response, &struct{}{})
	var apiErr *pkgerrors.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want *errors.APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, http.StatusServiceUnavailable)
	}
	var parseErr *pkgerrors.ParseError
	if errors.As(err, &parseErr) {
		t.Fatalf("non-JSON HTTP failure was misclassified as schema drift: %v", err)
	}
	if strings.Contains(err.Error(), "upstream unavailable") {
		t.Fatalf("provider response body leaked into error: %v", err)
	}
}

func jsonResponse(payload string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(payload)),
	}
}
