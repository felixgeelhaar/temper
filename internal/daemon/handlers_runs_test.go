package daemon

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestHandleCreateRun_SessionWithAppreciation(t *testing.T) {
	m := newServerWithMocks()
	m.server.appreciationService = appreciation.NewService()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return &session.Run{
			ID:        "run-1",
			SessionID: sessionID,
			Result:    &session.RunResult{TestOK: true, BuildOK: true},
		}, nil
	}
	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{ID: id, Status: session.StatusActive, CreatedAt: time.Now()}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"build":true}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleCreateRun_SessionGetFails(t *testing.T) {
	m := newServerWithMocks()
	m.server.appreciationService = appreciation.NewService()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return &session.Run{
			ID:        "run-1",
			SessionID: sessionID,
			Result:    &session.RunResult{TestOK: true, BuildOK: true},
		}, nil
	}
	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return nil, errors.New("get error")
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"build":true}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	// Should still succeed even if session.Get fails in appreciation block
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
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

func TestHandleCreateRun_BuildFails(t *testing.T) {
	m := newServerWithMocks()
	m.server.appreciationService = appreciation.NewService()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return &session.Run{
			ID:        "run-1",
			SessionID: sessionID,
			Result: &session.RunResult{
				TestOK:      false,
				BuildOK:     false,
				BuildOutput: "build failed",
			},
		}, nil
	}
	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{ID: id, Status: session.StatusActive}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"build":true,"code":{"main.go":"invalid"}}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleCreateRun_FormatFails(t *testing.T) {
	m := newServerWithMocks()
	m.server.appreciationService = appreciation.NewService()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return &session.Run{
			ID:        "run-1",
			SessionID: sessionID,
			Result: &session.RunResult{
				FormatOK:   false,
				FormatDiff: "fix formatting",
			},
		}, nil
	}
	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{ID: id, Status: session.StatusActive}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"format":true,"code":{"main.go":"bad"}}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleCreateRun_TestFails(t *testing.T) {
	m := newServerWithMocks()
	m.server.appreciationService = appreciation.NewService()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return &session.Run{
			ID:        "run-1",
			SessionID: sessionID,
			Result: &session.RunResult{
				TestOK:     false,
				TestOutput: "tests failed",
				BuildOK:    true,
			},
		}, nil
	}
	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{ID: id, Status: session.StatusActive}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"test":true,"code":{"main.go":"package main"}}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleCreateRun_FormatWithDiff(t *testing.T) {
	m := newServerWithMocks()
	m.server.appreciationService = appreciation.NewService()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return &session.Run{
			ID:        "run-1",
			SessionID: sessionID,
			Result: &session.RunResult{
				FormatOK:   false,
				FormatDiff: "diff here",
				BuildOK:    true,
				TestOK:     true,
			},
		}, nil
	}
	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{ID: id, Status: session.StatusActive}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"format":true,"code":{"main.go":"package main"}}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleCreateRun_AllOptions(t *testing.T) {
	m := newServerWithMocks()
	m.server.appreciationService = appreciation.NewService()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return &session.Run{
			ID:        "run-1",
			SessionID: sessionID,
			Result: &session.RunResult{
				FormatOK:   true,
				BuildOK:    true,
				TestOK:     true,
				TestOutput: "all tests passed",
			},
		}, nil
	}
	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{ID: id, Status: session.StatusActive}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"format":true,"build":true,"test":true,"code":{"main.go":"package main"}}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleCreateRun_WithResultNil(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return &session.Run{
			ID:        "run-1",
			SessionID: sessionID,
			Result:    nil,
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"code":{"main.go":"package main"}}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleCreateRun_TestOKFalse(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return &session.Run{
			ID:        "run-1",
			SessionID: sessionID,
			Result: &session.RunResult{
				TestOK: false,
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"code":{"main.go":"package main"}}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleCreateRun_EmptyCode(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return &session.Run{
			ID:        "run-1",
			SessionID: sessionID,
			Result:    nil,
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"code":{}}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleGetSession_Complete(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{
			ID:     id,
			Status: session.StatusActive,
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/s1", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleGetSession_Error(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return nil, errors.New("session error")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/s1", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}
