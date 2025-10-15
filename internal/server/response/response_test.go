package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	starmapErrors "github.com/agentstation/starmap/pkg/errors"
)

// TestSuccess tests the Success helper function.
func TestSuccess(t *testing.T) {
	data := map[string]string{"message": "success"}
	resp := Success(data)

	if resp.Data == nil {
		t.Error("expected Data to be set")
	}
	if resp.Error != nil {
		t.Error("expected Error to be nil")
	}
}

// TestFail tests the Fail helper function.
func TestFail(t *testing.T) {
	resp := Fail("TEST_ERROR", "Test error message", "Additional details")

	if resp.Data != nil {
		t.Error("expected Data to be nil")
	}
	if resp.Error == nil {
		t.Fatal("expected Error to be set")
	}
	if resp.Error.Code != "TEST_ERROR" {
		t.Errorf("expected Code=TEST_ERROR, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Test error message" {
		t.Errorf("expected Message=Test error message, got %s", resp.Error.Message)
	}
	if resp.Error.Details != "Additional details" {
		t.Errorf("expected Details=Additional details, got %s", resp.Error.Details)
	}
}

// TestJSON tests the JSON helper function.
func TestJSON(t *testing.T) {
	w := httptest.NewRecorder()
	resp := Success(map[string]string{"test": "data"})

	JSON(w, http.StatusOK, resp)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %s", contentType)
	}

	// Verify JSON is valid
	var decoded Response
	if err := json.NewDecoder(w.Body).Decode(&decoded); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if decoded.Data == nil {
		t.Error("expected decoded Data to be set")
	}
	if decoded.Error != nil {
		t.Error("expected decoded Error to be nil")
	}
}

// TestOK tests the OK helper function.
func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]int{"count": 42}

	OK(w, data)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Error("expected no error in response")
	}
}

// TestCreated tests the Created helper function.
func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"id": "new-resource"}

	Created(w, data)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

// TestErrorHelpers tests all error response helpers.
func TestErrorHelpers(t *testing.T) {
	tests := []struct {
		name           string
		fn             func(w http.ResponseWriter)
		expectedStatus int
		expectedCode   string
	}{
		{
			name: "BadRequest",
			fn: func(w http.ResponseWriter) {
				BadRequest(w, "Invalid request", "Missing field")
			},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "BAD_REQUEST",
		},
		{
			name: "Unauthorized",
			fn: func(w http.ResponseWriter) {
				Unauthorized(w, "Auth failed", "Invalid key")
			},
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "UNAUTHORIZED",
		},
		{
			name: "NotFound",
			fn: func(w http.ResponseWriter) {
				NotFound(w, "Resource not found", "ID not found")
			},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "NOT_FOUND",
		},
		{
			name: "MethodNotAllowed",
			fn: func(w http.ResponseWriter) {
				MethodNotAllowed(w, "POST")
			},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedCode:   "METHOD_NOT_ALLOWED",
		},
		{
			name: "RateLimited",
			fn: func(w http.ResponseWriter) {
				RateLimited(w, "Too many requests")
			},
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   "RATE_LIMITED",
		},
		{
			name: "InternalError",
			fn: func(w http.ResponseWriter) {
				InternalError(w, errors.New("internal error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
		{
			name: "ServiceUnavailable",
			fn: func(w http.ResponseWriter) {
				ServiceUnavailable(w, "Service down")
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedCode:   "SERVICE_UNAVAILABLE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.fn(w)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp Response
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Data != nil {
				t.Error("expected Data to be nil for error response")
			}
			if resp.Error == nil {
				t.Fatal("expected Error to be set")
			}
			if resp.Error.Code != tt.expectedCode {
				t.Errorf("expected Code=%s, got %s", tt.expectedCode, resp.Error.Code)
			}
		})
	}
}

// TestErrorFromType tests typed error mapping.
func TestErrorFromType(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "NotFoundError",
			err:            &starmapErrors.NotFoundError{Resource: "model", ID: "gpt-4"},
			expectedStatus: http.StatusNotFound,
			expectedCode:   "NOT_FOUND",
		},
		{
			name:           "ValidationError",
			err:            &starmapErrors.ValidationError{Field: "name", Value: "", Message: "required"},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "BAD_REQUEST",
		},
		{
			name:           "SyncError",
			err:            &starmapErrors.SyncError{Provider: "openai", Err: errors.New("sync failed")},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
		{
			name:           "APIError - 4xx",
			err:            &starmapErrors.APIError{Provider: "openai", Endpoint: "/models", StatusCode: 400},
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "BAD_REQUEST",
		},
		{
			name:           "APIError - 5xx",
			err:            &starmapErrors.APIError{Provider: "openai", Endpoint: "/models", StatusCode: 503},
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
		{
			name:           "Generic error",
			err:            errors.New("generic error"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ErrorFromType(w, tt.err)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var resp Response
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Data != nil {
				t.Error("expected Data to be nil for error response")
			}
			if resp.Error == nil {
				t.Fatal("expected Error to be set")
			}
			if resp.Error.Code != tt.expectedCode {
				t.Errorf("expected Code=%s, got %s", tt.expectedCode, resp.Error.Code)
			}
		})
	}
}

// TestResponseStructure tests the Response struct marshaling.
func TestResponseStructure(t *testing.T) {
	t.Run("success response structure", func(t *testing.T) {
		resp := Success(map[string]string{"key": "value"})
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled map[string]any
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		// Check structure
		if _, ok := unmarshaled["data"]; !ok {
			t.Error("expected 'data' field in JSON")
		}
		if _, ok := unmarshaled["error"]; !ok {
			t.Error("expected 'error' field in JSON")
		}
	})

	t.Run("error response structure", func(t *testing.T) {
		resp := Fail("TEST", "message", "details")
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled map[string]any
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		// Check error structure
		if unmarshaled["data"] != nil {
			t.Error("expected 'data' to be null")
		}

		errorField, ok := unmarshaled["error"].(map[string]any)
		if !ok {
			t.Fatal("expected 'error' to be an object")
		}

		if errorField["code"] != "TEST" {
			t.Errorf("expected code=TEST, got %v", errorField["code"])
		}
		if errorField["message"] != "message" {
			t.Errorf("expected message=message, got %v", errorField["message"])
		}
		if errorField["details"] != "details" {
			t.Errorf("expected details=details, got %v", errorField["details"])
		}
	})
}

// TestErrorDetails tests error details omitempty behavior.
func TestErrorDetails(t *testing.T) {
	t.Run("with details", func(t *testing.T) {
		resp := Fail("TEST", "message", "details")
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled map[string]any
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		errorField := unmarshaled["error"].(map[string]any)
		if _, ok := errorField["details"]; !ok {
			t.Error("expected 'details' field when provided")
		}
	})

	t.Run("without details", func(t *testing.T) {
		resp := Fail("TEST", "message", "")
		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var unmarshaled map[string]any
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		errorField := unmarshaled["error"].(map[string]any)
		// omitempty should exclude empty details
		if details, ok := errorField["details"]; ok && details != "" {
			t.Errorf("expected 'details' to be omitted when empty, got %v", details)
		}
	})
}

// TestComplexDataTypes tests response with various data types.
func TestComplexDataTypes(t *testing.T) {
	type TestStruct struct {
		Name   string   `json:"name"`
		Count  int      `json:"count"`
		Active bool     `json:"active"`
		Tags   []string `json:"tags"`
	}

	tests := []struct {
		name string
		data any
	}{
		{"string", "hello"},
		{"int", 42},
		{"bool", true},
		{"slice", []string{"a", "b", "c"}},
		{"map", map[string]int{"one": 1, "two": 2}},
		{"struct", TestStruct{Name: "test", Count: 123, Active: true, Tags: []string{"tag1"}}},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			OK(w, tt.data)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			var resp Response
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
		})
	}
}
