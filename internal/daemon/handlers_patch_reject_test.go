package daemon

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/temper/internal/patch"
	"github.com/google/uuid"
)

func TestHandlePatchReject_InvalidSessionID(t *testing.T) {
	m := newServerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/not-a-uuid/patch/reject", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlePatchReject_NotFound(t *testing.T) {
	m := newServerWithMocks()
	m.patches.rejectPendingFn = func(sessionID uuid.UUID) error {
		return patch.ErrPatchNotFound
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/patch/reject", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlePatchReject_Error(t *testing.T) {
	m := newServerWithMocks()
	m.patches.rejectPendingFn = func(sessionID uuid.UUID) error {
		return errors.New("reject error")
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/patch/reject", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandlePatchReject_Success(t *testing.T) {
	m := newServerWithMocks()
	m.patches.rejectPendingFn = func(sessionID uuid.UUID) error {
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/patch/reject", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}
