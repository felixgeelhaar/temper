package daemon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/profile"
	"github.com/felixgeelhaar/temper/internal/session"
	"github.com/felixgeelhaar/temper/internal/spec"
	"github.com/google/uuid"
)

// Tests using mock dependencies for isolated handler testing
// These tests cover error paths that are difficult to trigger with real services

func TestMock_Profile_ServiceError(t *testing.T) {
	m := newServerWithMocks()

	m.profiles.getProfileFn = func(ctx context.Context) (*profile.StoredProfile, error) {
		return nil, errors.New("database connection failed")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/profile", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_AnalyticsOverview_ServiceError(t *testing.T) {
	m := newServerWithMocks()

	m.profiles.getOverviewFn = func(ctx context.Context) (*profile.AnalyticsOverview, error) {
		return nil, errors.New("analytics service unavailable")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/overview", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_AnalyticsSkills_ServiceError(t *testing.T) {
	m := newServerWithMocks()

	m.profiles.getSkillBreakdownFn = func(ctx context.Context) (*profile.SkillBreakdown, error) {
		return nil, errors.New("skills service unavailable")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/skills", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_AnalyticsErrors_ServiceError(t *testing.T) {
	m := newServerWithMocks()

	m.profiles.getErrorPatternsFn = func(ctx context.Context) ([]profile.ErrorPattern, error) {
		return nil, errors.New("error patterns service unavailable")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/errors", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_AnalyticsTrend_ServiceError(t *testing.T) {
	m := newServerWithMocks()

	m.profiles.getHintTrendFn = func(ctx context.Context) ([]profile.HintDependencyPoint, error) {
		return nil, errors.New("trend service unavailable")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/trend", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_Session_GetSuccess(t *testing.T) {
	m := newServerWithMocks()

	sessionID := uuid.New().String()
	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{
			ID:     sessionID,
			Status: session.StatusActive,
			Code:   map[string]string{"main.go": "package main"},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMock_Session_GetNotFound(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return nil, session.ErrSessionNotFound
	}

	sessionID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestMock_Session_DeleteError(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.deleteFn = func(ctx context.Context, id string) error {
		return errors.New("delete failed")
	}

	sessionID := uuid.New().String()
	req := httptest.NewRequest(http.MethodDelete, "/v1/sessions/"+sessionID, nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_Spec_ListError(t *testing.T) {
	m := newServerWithMocks()

	m.specs.listFn = func(ctx context.Context) ([]*domain.ProductSpec, error) {
		return nil, errors.New("failed to list specs")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/specs", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_Spec_GetProgressError(t *testing.T) {
	m := newServerWithMocks()

	m.specs.getProgressFn = func(ctx context.Context, path string) (*domain.SpecProgress, error) {
		return nil, errors.New("progress calculation failed")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/file/test.spec.yaml/progress", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_Spec_GetDriftError(t *testing.T) {
	m := newServerWithMocks()

	m.specs.getDriftFn = func(ctx context.Context, path string) (*spec.DriftReport, error) {
		return nil, errors.New("drift detection failed")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/file/test.spec.yaml/drift", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

// Run handler tests

func TestMock_CreateRun_SessionNotFound(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return nil, session.ErrSessionNotFound
	}

	sessionID := uuid.New().String()
	body := `{"code":{"main.go":"package main"},"format":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestMock_CreateRun_SessionNotActive(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return nil, session.ErrSessionNotActive
	}

	sessionID := uuid.New().String()
	body := `{"code":{"main.go":"package main"},"build":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestMock_CreateRun_ServiceError(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.runCodeFn = func(ctx context.Context, sessionID string, req session.RunRequest) (*session.Run, error) {
		return nil, errors.New("run execution failed")
	}

	sessionID := uuid.New().String()
	body := `{"code":{"main.go":"package main"},"test":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

// Patch handler tests

func TestMock_PatchApply_SessionNotFound(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return nil, session.ErrSessionNotFound
	}

	sessionID := uuid.New().String()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patches/apply", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

func TestMock_PatchReject_SessionNotFound(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return nil, session.ErrSessionNotFound
	}

	sessionID := uuid.New().String()
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/patches/reject", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d: %s", http.StatusNotFound, w.Code, w.Body.String())
	}
}

// Spec handler tests

func TestMock_MarkCriterionSatisfied_Error(t *testing.T) {
	m := newServerWithMocks()

	m.specs.markCriterionSatisfiedFn = func(ctx context.Context, path, criterionID, evidence string) error {
		return errors.New("failed to mark criterion")
	}

	body := `{"path":"test.spec.yaml","evidence":"test passed"}`
	req := httptest.NewRequest(http.MethodPut, "/v1/specs/criteria/crit-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_LockSpec_Error(t *testing.T) {
	m := newServerWithMocks()

	m.specs.lockFn = func(ctx context.Context, path string) (*domain.SpecLock, error) {
		return nil, errors.New("failed to lock spec")
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/lock/test.spec.yaml", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_ValidateSpec_Error(t *testing.T) {
	m := newServerWithMocks()

	m.specs.validateFn = func(ctx context.Context, path string) (*domain.SpecValidation, error) {
		return nil, errors.New("validation failed")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/specs/file/test.spec.yaml/validate", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_CreateSpec_Error(t *testing.T) {
	m := newServerWithMocks()

	m.specs.createFn = func(ctx context.Context, name string) (*domain.ProductSpec, error) {
		return nil, errors.New("failed to create spec")
	}

	body := `{"name":"test-spec"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/specs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

// Session create error path

func TestMock_Session_CreateError(t *testing.T) {
	m := newServerWithMocks()

	m.sessions.createFn = func(ctx context.Context, req session.CreateRequest) (*session.Session, error) {
		return nil, errors.New("failed to create session")
	}

	body := `{"exercise_id":"test-exercise"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

// Pairing service error paths

func TestMock_Pairing_HintError(t *testing.T) {
	m := newServerWithMocks()

	sessionID := uuid.New().String()

	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{
			ID:     sessionID,
			Status: session.StatusActive,
			Code:   map[string]string{"main.go": "package main"},
		}, nil
	}

	m.pairing.interveneFn = func(ctx context.Context, req pairing.InterventionRequest) (*domain.Intervention, error) {
		return nil, errors.New("pairing service unavailable")
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/hint", nil)
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

// Format handler tests

func TestMock_Format_Error(t *testing.T) {
	m := newServerWithMocks()

	sessionID := uuid.New().String()

	m.executor.runFormatFixFn = func(ctx context.Context, code map[string]string) (map[string]string, error) {
		return nil, errors.New("format check failed")
	}

	body := `{"code":{"main.go":"package main"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

func TestMock_Format_Success(t *testing.T) {
	m := newServerWithMocks()

	sessionID := uuid.New().String()

	m.executor.runFormatFixFn = func(ctx context.Context, code map[string]string) (map[string]string, error) {
		return map[string]string{"main.go": "package main\n"}, nil
	}

	body := `{"code":{"main.go":"package main"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/format", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	m.server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}
