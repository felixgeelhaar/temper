package daemon

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/temper/internal/appreciation"
	"github.com/felixgeelhaar/temper/internal/session"
)

func TestHandleCreateRun_SessionErrors(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return nil, session.ErrSessionNotFound
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"build":true}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusNotFound)
	}

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return nil, session.ErrSessionNotActive
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"build":true}`)))
	rec = httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return nil, errNotImplemented
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"build":true}`)))
	rec = httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleCreateRun_InvalidBody(t *testing.T) {
	m := newServerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte("{invalid")))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateRun_Success(t *testing.T) {
	m := newServerWithMocks()
	m.server.appreciationService = appreciation.NewService()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return &session.Run{
			ID:        "run-1",
			SessionID: sessionID,
			Result: &session.RunResult{
				TestOK:  true,
				BuildOK: true,
			},
		}, nil
	}
	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{ID: id, Status: session.StatusActive}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"build":true}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}
