package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/session"
)

func setupAuthoringServer(t *testing.T) (*serverWithMocks, string) {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Overview\n\nGoals"), 0644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	m := newServerWithMocks()
	m.specs.getWorkspaceRootFn = func() string { return tmpDir }
	m.specs.loadFn = func(ctx context.Context, path string) (*domain.ProductSpec, error) {
		return &domain.ProductSpec{Name: "Spec", AcceptanceCriteria: []domain.AcceptanceCriterion{}}, nil
	}
	m.sessions.getFn = func(ctx context.Context, id string) (*session.Session, error) {
		return &session.Session{
			ID:            id,
			Intent:        session.IntentSpecAuthoring,
			SpecPath:      "spec.yaml",
			AuthoringDocs: []string{"README.md"},
			CreatedAt:     time.Now(),
		}, nil
	}
	m.pairing.suggestForSectionFn = func(ctx context.Context, authCtx pairing.AuthoringContext) ([]domain.AuthoringSuggestion, error) {
		return []domain.AuthoringSuggestion{{ID: "s1", Section: authCtx.Section, Value: "goal", Confidence: 0.8}}, nil
	}
	m.pairing.authoringHintFn = func(ctx context.Context, authCtx pairing.AuthoringContext) (*domain.Intervention, error) {
		return &domain.Intervention{Type: domain.TypeHint, Content: "Consider scope"}, nil
	}

	return m, tmpDir
}

func TestAuthoringDiscover_DefaultPaths(t *testing.T) {
	m, _ := setupAuthoringServer(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/authoring/discover", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthoringSuggest_Success(t *testing.T) {
	m, _ := setupAuthoringServer(t)

	body := []byte(`{"section":"goals"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/authoring/suggest", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["section"] != "goals" {
		t.Errorf("section = %v; want goals", resp["section"])
	}
}

func TestAuthoringHint_Success(t *testing.T) {
	m, _ := setupAuthoringServer(t)

	body := []byte(`{"section":"goals","question":"What next?"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/session-1/authoring/hint", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}
