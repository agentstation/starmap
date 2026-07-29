// Package response provides standardized HTTP response structures and helpers
// for the Starmap API server. All API responses follow a consistent format
// with a data field for successful responses and an error field for failures.
package response

import (
	"encoding/json"
	stderrors "errors"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/agentstation/starmap/pkg/errors"
)

// Response represents the standardized API response structure.
// All endpoints return this format for consistency.
type Response struct {
	Data  any    `json:"data" swaggertype:"object"`
	Error *Error `json:"error,omitempty"`
}

// Error represents an API error with code, message, and optional details.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Success creates a successful response with data.
func Success(data any) Response {
	return Response{
		Data:  data,
		Error: nil,
	}
}

// Fail creates an error response.
func Fail(code, message, details string) Response {
	return Response{
		Data: nil,
		Error: &Error{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Encoding errors are ignored as headers are already sent (best effort)
	_ = json.NewEncoder(w).Encode(resp)
}

// OK writes a successful response with 200 status.
func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, Success(data))
}

// Created writes a successful response with 201 status.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, Success(data))
}

// BadRequest writes a 400 error response.
func BadRequest(w http.ResponseWriter, message, details string) {
	JSON(w, http.StatusBadRequest, Fail("BAD_REQUEST", message, details))
}

// Unauthorized writes a 401 error response.
func Unauthorized(w http.ResponseWriter, message, details string) {
	JSON(w, http.StatusUnauthorized, Fail("UNAUTHORIZED", message, details))
}

// NotFound writes a 404 error response.
func NotFound(w http.ResponseWriter, message, details string) {
	JSON(w, http.StatusNotFound, Fail("NOT_FOUND", message, details))
}

// MethodNotAllowed writes a 405 error response.
func MethodNotAllowed(w http.ResponseWriter, method string) {
	JSON(w, http.StatusMethodNotAllowed, Fail(
		"METHOD_NOT_ALLOWED",
		"Method not allowed",
		"Method "+method+" is not supported for this endpoint",
	))
}

// RateLimited writes a 429 error response.
func RateLimited(w http.ResponseWriter, message string) {
	JSON(w, http.StatusTooManyRequests, Fail(
		"RATE_LIMITED",
		"Rate limit exceeded",
		message,
	))
}

// InternalError writes a 500 error response.
func InternalError(w http.ResponseWriter, logger *zerolog.Logger, err error) {
	// Log the actual error but don't expose details to client
	if logger != nil {
		logger.Error().Err(err).Msg("Internal server error")
	}
	JSON(w, http.StatusInternalServerError, Fail(
		"INTERNAL_ERROR",
		"Internal server error",
		"An unexpected error occurred",
	))
}

// ServiceUnavailable writes a 503 error response.
func ServiceUnavailable(w http.ResponseWriter, message string) {
	JSON(w, http.StatusServiceUnavailable, Fail(
		"SERVICE_UNAVAILABLE",
		"Service unavailable",
		message,
	))
}

// ErrorFromType maps typed errors to appropriate HTTP responses.
func ErrorFromType(w http.ResponseWriter, logger *zerolog.Logger, err error) {
	var notFound *errors.NotFoundError
	var validation *errors.ValidationError
	var syncErr *errors.SyncError
	var apiErr *errors.APIError

	switch {
	case stderrors.As(err, &notFound):
		NotFound(w, notFound.Error(), "")
	case stderrors.As(err, &validation):
		BadRequest(w, validation.Error(), "")
	case stderrors.As(err, &syncErr):
		InternalError(w, logger, err)
	case stderrors.As(err, &apiErr):
		if apiErr.StatusCode >= 500 {
			InternalError(w, logger, err)
		} else {
			BadRequest(w, apiErr.Error(), "")
		}
	default:
		InternalError(w, logger, err)
	}
}
