package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/temper/internal/patch"
	"github.com/felixgeelhaar/temper/internal/session"
	"github.com/google/uuid"
)

func TestHandlePatchApply_Errors(t *testing.T) {
	m := newServerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/not-a-uuid/patch/apply", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid session ID status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return nil, session.ErrSessionNotFound
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000001/patch/apply", nil)
	rec = httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("session not found status = %d; want %d", rec.Code, http.StatusNotFound)
	}

	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{ID: id, Code: map[string]string{"main.go": "package main"}}, nil
	}
	m.patches.applyPendingFn = func(sessionID uuid.UUID) (string, string, error) {
		return "", "", patch.ErrPatchNotFound
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000002/patch/apply", nil)
	rec = httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("patch not found status = %d; want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlePatchApply_Success(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{ID: id, Code: map[string]string{"main.go": "package main"}}, nil
	}
	m.sessions.updateCodeFn = func(ctx context.Context, id string, code map[string]string) (*session.Session, error) {
		return &session.Session{ID: id, Code: code}, nil
	}
	m.patches.applyPendingFn = func(sessionID uuid.UUID) (string, string, error) {
		return "main.go", "package main\n\nfunc main() {}", nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/00000000-0000-0000-0000-000000000003/patch/apply", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandlePatchStats(t *testing.T) {
	m := newServerWithMocks()

	noLoggerReq := httptest.NewRequest(http.MethodGet, "/v1/patches/stats", nil)
	noLoggerRec := httptest.NewRecorder()
	m.server.router.ServeHTTP(noLoggerRec, noLoggerReq)
	if noLoggerRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want %d", noLoggerRec.Code, http.StatusServiceUnavailable)
	}

	logger, err := patch.NewLogger(t.TempDir())
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	m.patches.getLoggerFn = func() *patch.Logger { return logger }

	req := httptest.NewRequest(http.MethodGet, "/v1/patches/stats", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}
