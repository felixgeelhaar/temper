package daemon

import "net/http"

// Stable error codes returned in the daemon's JSON error responses.
// Editor clients switch on these, never on message strings.
//
// Adding a new code: append to the matching status block; do NOT renumber
// or rename existing entries — they form a public contract with editor
// clients.
const (
	// 400 Bad Request
	ErrCodeBadRequest      = "BAD_REQUEST"
	ErrCodeInvalidPayload  = "INVALID_PAYLOAD"
	ErrCodeSpecInvalid     = "SPEC_INVALID"
	ErrCodeFileTooLarge    = "FILE_TOO_LARGE"
	ErrCodeTooManyFiles    = "TOO_MANY_FILES"
	ErrCodePayloadTooLarge = "PAYLOAD_TOO_LARGE"

	// 401 Unauthorized / 403 Forbidden
	ErrCodeUnauthorized = "UNAUTHORIZED"
	ErrCodeForbidden    = "FORBIDDEN"

	// 404 Not Found
	ErrCodeNotFound          = "NOT_FOUND"
	ErrCodeSessionNotFound   = "SESSION_NOT_FOUND"
	ErrCodeExerciseNotFound  = "EXERCISE_NOT_FOUND"
	ErrCodeSpecNotFound      = "SPEC_NOT_FOUND"
	ErrCodeCriterionNotFound = "CRITERION_NOT_FOUND"
	ErrCodeTrackNotFound     = "TRACK_NOT_FOUND"
	ErrCodeSandboxNotFound   = "SANDBOX_NOT_FOUND"
	ErrCodePatchNotFound     = "PATCH_NOT_FOUND"

	// 409 Conflict
	ErrCodeConflict          = "CONFLICT"
	ErrCodeSessionConflict   = "SESSION_CONFLICT"
	ErrCodeSandboxLimitHit   = "SANDBOX_LIMIT_REACHED"

	// 410 Gone
	ErrCodeSandboxExpired = "SANDBOX_EXPIRED"
	ErrCodePatchExpired   = "PATCH_EXPIRED"

	// 422 Unprocessable Entity
	ErrCodeUnprocessable = "UNPROCESSABLE"

	// 429 Too Many Requests
	ErrCodeRateLimited = "RATE_LIMITED"

	// 500 Internal Server Error
	ErrCodeInternal       = "INTERNAL_ERROR"
	ErrCodeClampViolation = "CLAMP_VIOLATION"

	// 503 Service Unavailable
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeLLMUnavailable     = "LLM_UNAVAILABLE"
)

// defaultErrorCodeForStatus maps an HTTP status to a fallback error code.
// Used by the legacy jsonError path for handlers that have not yet been
// updated to pass an explicit code.
func defaultErrorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return ErrCodeBadRequest
	case http.StatusUnauthorized:
		return ErrCodeUnauthorized
	case http.StatusForbidden:
		return ErrCodeForbidden
	case http.StatusNotFound:
		return ErrCodeNotFound
	case http.StatusConflict:
		return ErrCodeConflict
	case http.StatusGone:
		return ErrCodeSandboxExpired
	case http.StatusRequestEntityTooLarge:
		return ErrCodePayloadTooLarge
	case http.StatusUnprocessableEntity:
		return ErrCodeUnprocessable
	case http.StatusTooManyRequests:
		return ErrCodeRateLimited
	case http.StatusServiceUnavailable:
		return ErrCodeServiceUnavailable
	default:
		return ErrCodeInternal
	}
}
