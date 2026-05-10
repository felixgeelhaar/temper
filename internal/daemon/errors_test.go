package daemon

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultErrorCodeForStatus(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{http.StatusBadRequest, ErrCodeBadRequest},
		{http.StatusUnauthorized, ErrCodeUnauthorized},
		{http.StatusForbidden, ErrCodeForbidden},
		{http.StatusNotFound, ErrCodeNotFound},
		{http.StatusConflict, ErrCodeConflict},
		{http.StatusGone, ErrCodeSandboxExpired},
		{http.StatusRequestEntityTooLarge, ErrCodePayloadTooLarge},
		{http.StatusUnprocessableEntity, ErrCodeUnprocessable},
		{http.StatusTooManyRequests, ErrCodeRateLimited},
		{http.StatusServiceUnavailable, ErrCodeServiceUnavailable},
		{http.StatusInternalServerError, ErrCodeInternal},
		{418, ErrCodeInternal}, // unmapped → INTERNAL_ERROR
	}
	for _, tc := range cases {
		if got := defaultErrorCodeForStatus(tc.status); got != tc.want {
			t.Errorf("defaultErrorCodeForStatus(%d) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestJsonError_AutoMapsErrorCode(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()

	s.jsonError(rec, http.StatusNotFound, "missing", nil)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error_code"] != ErrCodeNotFound {
		t.Errorf("error_code = %v, want %s", body["error_code"], ErrCodeNotFound)
	}
}

func TestJsonErrorCode_OverridesDefault(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()

	s.jsonErrorCode(rec, http.StatusNotFound, ErrCodeSessionNotFound, "session missing", nil)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error_code"] != ErrCodeSessionNotFound {
		t.Errorf("error_code = %v, want %s", body["error_code"], ErrCodeSessionNotFound)
	}
}

func TestJsonErrorCode_PayloadErrorWins(t *testing.T) {
	// PayloadError already carries an authoritative code; jsonErrorCode
	// should surface it even if a different default code was passed.
	s := &Server{}
	rec := httptest.NewRecorder()

	pe := &PayloadError{Code: ErrCodeFileTooLarge, Message: "f.go is huge"}
	s.jsonErrorCode(rec, http.StatusRequestEntityTooLarge, ErrCodePayloadTooLarge, "too big", pe)

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error_code"] != ErrCodeFileTooLarge {
		t.Errorf("error_code = %v, want %s (payload error must win)",
			body["error_code"], ErrCodeFileTooLarge)
	}
}

func TestJsonErrorCode_PassesDetailsAlong(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()

	s.jsonErrorCode(rec, http.StatusInternalServerError, "", "boom", errors.New("inner cause"))

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["details"] != "inner cause" {
		t.Errorf("details = %v, want %q", body["details"], "inner cause")
	}
	if body["error_code"] != ErrCodeInternal {
		t.Errorf("error_code = %v, want %s (empty code → default)",
			body["error_code"], ErrCodeInternal)
	}
}
