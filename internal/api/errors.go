package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// Common API errors
var (
	ErrNotFound       = errors.New("not found")
	ErrBadRequest     = errors.New("bad request")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrForbidden      = errors.New("forbidden")
	ErrConflict       = errors.New("conflict")
	ErrInternalServer = errors.New("internal server error")
)

// APIError represents a structured API error
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
	cause   error
}

func (e *APIError) Error() string {
	return e.Message
}

func (e *APIError) Unwrap() error {
	return e.cause
}

// NewAPIError creates a new API error
func NewAPIError(code string, message string) *APIError {
	return &APIError{Code: code, Message: message}
}

// WithDetails adds details to the error
func (e *APIError) WithDetails(details any) *APIError {
	e.Details = details
	return e
}

// WithCause wraps an underlying error
func (e *APIError) WithCause(err error) *APIError {
	e.cause = err
	return e
}

// Common error constructors
func ErrBadRequestWith(message string) *APIError {
	return NewAPIError("BAD_REQUEST", message)
}

func ErrNotFoundWith(resource string) *APIError {
	return NewAPIError("NOT_FOUND", resource+" not found")
}

func ErrUnauthorizedWith(message string) *APIError {
	return NewAPIError("UNAUTHORIZED", message)
}

func ErrForbiddenWith(message string) *APIError {
	return NewAPIError("FORBIDDEN", message)
}

func ErrConflictWith(message string) *APIError {
	return NewAPIError("CONFLICT", message)
}

func ErrInternalWith(message string, cause error) *APIError {
	return NewAPIError("INTERNAL_ERROR", message).WithCause(cause)
}

// ErrorResponse is the JSON structure for error responses
type ErrorResponse struct {
	Error *APIError `json:"error"`
}

// WriteError writes an error response to the response writer
func WriteError(w http.ResponseWriter, r *http.Request, statusCode int, apiErr *APIError) {
	// Log the error with request context
	logger := slog.Default()
	logAttrs := []any{
		"code", apiErr.Code,
		"message", apiErr.Message,
		"status", statusCode,
		"method", r.Method,
		"path", r.URL.Path,
	}

	if apiErr.cause != nil {
		logAttrs = append(logAttrs, "cause", apiErr.cause.Error())
	}

	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		logAttrs = append(logAttrs, "request_id", requestID)
	}

	// Log at appropriate level based on status code
	if statusCode >= 500 {
		logger.Error("api error", logAttrs...)
	} else if statusCode >= 400 {
		logger.Warn("api error", logAttrs...)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: apiErr})
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// Helper functions for common responses
func BadRequest(w http.ResponseWriter, r *http.Request, message string) {
	WriteError(w, r, http.StatusBadRequest, ErrBadRequestWith(message))
}

func NotFound(w http.ResponseWriter, r *http.Request, resource string) {
	WriteError(w, r, http.StatusNotFound, ErrNotFoundWith(resource))
}

func Unauthorized(w http.ResponseWriter, r *http.Request, message string) {
	WriteError(w, r, http.StatusUnauthorized, ErrUnauthorizedWith(message))
}

func Forbidden(w http.ResponseWriter, r *http.Request, message string) {
	WriteError(w, r, http.StatusForbidden, ErrForbiddenWith(message))
}

func Conflict(w http.ResponseWriter, r *http.Request, message string) {
	WriteError(w, r, http.StatusConflict, ErrConflictWith(message))
}

func InternalError(w http.ResponseWriter, r *http.Request, message string, cause error) {
	WriteError(w, r, http.StatusInternalServerError, ErrInternalWith(message, cause))
}
