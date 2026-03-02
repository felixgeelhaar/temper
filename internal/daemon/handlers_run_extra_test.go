package daemon

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/temper/internal/runner"
)

func TestHandleFormat_Success(t *testing.T) {
	m := newServerWithMocks()

	m.executor.runFormatFixFn = func(ctx context.Context, code map[string]string) (map[string]string, error) {
		return map[string]string{"main.go": "package main\n"}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/format", bytes.NewReader([]byte(`{"code":{"main.go":"package main"}}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleFormat_Error(t *testing.T) {
	m := newServerWithMocks()

	m.executor.runFormatFixFn = func(ctx context.Context, code map[string]string) (map[string]string, error) {
		return nil, errors.New("format error")
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/format", bytes.NewReader([]byte(`{"code":{"main.go":"package main"}}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleFormat_InvalidBody(t *testing.T) {
	m := newServerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/format", bytes.NewReader([]byte("{invalid")))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateRun_RunnerError(t *testing.T) {
	m := newServerWithMocks()

	m.executor.runFormatFn = func(ctx context.Context, code map[string]string) (*runner.FormatResult, error) {
		return nil, errors.New("format error")
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/s1/runs", bytes.NewReader([]byte(`{"format":true}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}
