package daemon

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/sandbox"
)

// TestSandboxHandlers tests the sandbox HTTP handlers
func TestHandleCreateSandbox_Success(t *testing.T) {
	m := newServerWithMocks()

	// Create a minimal mock that satisfies the interface needed
	m.sandbox.createFn = func(ctx context.Context, sessionID string, cfg sandbox.Config) (*sandbox.Sandbox, error) {
		return &sandbox.Sandbox{
			ID:        "sb-1",
			SessionID: sessionID,
			Status:    sandbox.StatusReady,
			CreatedAt: time.Now(),
		}, nil
	}

	body := []byte(`{"language":"go"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/sandbox", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusCreated)
	}
}

func TestHandleCreateSandbox_AlreadyExists(t *testing.T) {
	m := newServerWithMocks()

	m.sandbox.createFn = func(ctx context.Context, sessionID string, cfg sandbox.Config) (*sandbox.Sandbox, error) {
		return nil, sandbox.ErrSessionHasSandbox
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/sandbox", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusConflict)
	}
}

func TestHandleCreateSandbox_MaxReached(t *testing.T) {
	m := newServerWithMocks()

	m.sandbox.createFn = func(ctx context.Context, sessionID string, cfg sandbox.Config) (*sandbox.Sandbox, error) {
		return nil, sandbox.ErrMaxSandboxes
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/sandbox", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	// ErrMaxSandboxes returns 429 (too many requests)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestHandleGetSandbox_Success(t *testing.T) {
	m := newServerWithMocks()

	m.sandbox.getBySessionFn = func(ctx context.Context, sessionID string) (*sandbox.Sandbox, error) {
		return &sandbox.Sandbox{
			ID:        "sb-1",
			SessionID: sessionID,
			Status:    sandbox.StatusReady,
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/s1/sandbox", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleGetSandbox_NotFound(t *testing.T) {
	m := newServerWithMocks()

	m.sandbox.getBySessionFn = func(ctx context.Context, sessionID string) (*sandbox.Sandbox, error) {
		return nil, errors.New("not found")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/s1/sandbox", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleDestroySandbox_Success(t *testing.T) {
	m := newServerWithMocks()

	m.sandbox.getBySessionFn = func(ctx context.Context, sessionID string) (*sandbox.Sandbox, error) {
		return &sandbox.Sandbox{ID: "sb-1", SessionID: sessionID}, nil
	}
	m.sandbox.destroyFn = func(ctx context.Context, id string) error {
		return nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/s1/sandbox", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleDestroySandbox_Error(t *testing.T) {
	m := newServerWithMocks()

	m.sandbox.getBySessionFn = func(ctx context.Context, sessionID string) (*sandbox.Sandbox, error) {
		return &sandbox.Sandbox{ID: "sb-1", SessionID: sessionID}, nil
	}
	m.sandbox.destroyFn = func(ctx context.Context, id string) error {
		return errors.New("destroy error")
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/s1/sandbox", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandlePauseSandbox_Success(t *testing.T) {
	m := newServerWithMocks()

	m.sandbox.getBySessionFn = func(ctx context.Context, sessionID string) (*sandbox.Sandbox, error) {
		return &sandbox.Sandbox{ID: "sb-1", SessionID: sessionID}, nil
	}
	m.sandbox.pauseFn = func(ctx context.Context, id string) error {
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/sandbox/pause", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleResumeSandbox_Success(t *testing.T) {
	m := newServerWithMocks()

	m.sandbox.getBySessionFn = func(ctx context.Context, sessionID string) (*sandbox.Sandbox, error) {
		return &sandbox.Sandbox{ID: "sb-1", SessionID: sessionID}, nil
	}
	m.sandbox.resumeFn = func(ctx context.Context, id string) error {
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/sandbox/resume", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleSandboxExec_Success(t *testing.T) {
	m := newServerWithMocks()

	m.sandbox.getBySessionFn = func(ctx context.Context, sessionID string) (*sandbox.Sandbox, error) {
		return &sandbox.Sandbox{ID: "sb-1", SessionID: sessionID, Status: sandbox.StatusReady}, nil
	}
	m.sandbox.execFn = func(ctx context.Context, id string, cmd []string, timeout time.Duration) (*sandbox.ExecResult, error) {
		return &sandbox.ExecResult{
			ExitCode: 0,
			Stdout:   "hello",
			Stderr:   "",
		}, nil
	}

	body := []byte(`{"cmd":["echo","hello"]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/sandbox/exec", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleSandboxExec_NotFound(t *testing.T) {
	m := newServerWithMocks()

	m.sandbox.getBySessionFn = func(ctx context.Context, sessionID string) (*sandbox.Sandbox, error) {
		return nil, sandbox.ErrSandboxNotFound
	}

	body := []byte(`{"cmd":["echo","hello"]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/sandbox/exec", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusNotFound)
	}
}
