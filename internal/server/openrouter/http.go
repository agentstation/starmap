package openrouter

import (
	"encoding/json"
	"net/http"
	"strings"
)

// WriteModel writes a successful OpenRouter-compatible model response.
func WriteModel(w http.ResponseWriter, model Model) {
	writeJSON(w, http.StatusOK, ModelEnvelope{Data: model})
}

// WriteEndpoints writes a successful OpenRouter-compatible endpoint response.
func WriteEndpoints(w http.ResponseWriter, endpoints Endpoints) {
	writeJSON(w, http.StatusOK, EndpointsEnvelope{Data: endpoints})
}

// WriteError writes an OpenRouter-compatible numeric error envelope.
func WriteError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorEnvelope{Error: Error{
		Code: status, Message: message,
	}})
}

// WriteProjectionError maps typed catalog lookup failures to the deliberately
// small OpenRouter discovery error surface.
func WriteProjectionError(w http.ResponseWriter, err error) {
	if isNotFoundOrConflict(err) {
		WriteError(w, http.StatusNotFound, "Resource not found")
		return
	}
	WriteError(w, http.StatusInternalServerError, "Internal Server Error")
}

// IsCompatibilityPath reports whether path names one of Starmap's exact
// OpenRouter-compatible discovery route shapes for the supplied prefix.
func IsCompatibilityPath(path, prefix string) bool {
	singular := strings.TrimPrefix(path, prefix+"/model/")
	if singular != path && exactSegments(singular, 2) {
		return true
	}
	endpoints := strings.TrimPrefix(path, prefix+"/models/")
	parts := strings.Split(endpoints, "/")
	return endpoints != path && len(parts) == 3 &&
		parts[0] != "" && parts[1] != "" && parts[2] == "endpoints"
}

func exactSegments(path string, count int) bool {
	parts := strings.Split(path, "/")
	if len(parts) != count {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
