package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/llm"
)

type mockProvider struct {
	name string
	resp string
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Generate(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	return &llm.Response{Content: m.resp}, nil
}

func (m *mockProvider) GenerateStream(ctx context.Context, req *llm.Request) (<-chan llm.StreamChunk, error) {
	return nil, errNotImplemented
}

func (m *mockProvider) SupportsStreaming() bool { return false }

func TestHandleGenerateSpec_Validation(t *testing.T) {
	m := newServerWithMocks()

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/generate", bytes.NewReader([]byte(`{"description":"desc"}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGenerateSpec_NoProvider(t *testing.T) {
	m := newServerWithMocks()
	m.registry.defaultFn = func() (llm.Provider, error) {
		return nil, llm.ErrNoDefaultProvider
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/generate", bytes.NewReader([]byte(`{"name":"Demo","description":"Test"}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleGenerateSpec_ParseError(t *testing.T) {
	m := newServerWithMocks()

	provider := &mockProvider{name: "mock", resp: "not json"}
	m.registry.defaultFn = func() (llm.Provider, error) { return provider, nil }

	req := httptest.NewRequest(http.MethodPost, "/v1/specs/generate", bytes.NewReader([]byte(`{"name":"Demo","description":"Test"}`)))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleGenerateSpec_Success(t *testing.T) {
	m := newServerWithMocks()

	provider := &mockProvider{
		name: "mock",
		resp: "result:\n{" +
			"\"name\":\"Demo\"," +
			"\"version\":\"0.1.0\"," +
			"\"goals\":[\"g1\"]," +
			"\"features\":[{" +
			"\"id\":\"feature-1\",\"title\":\"Feature\",\"description\":\"desc\",\"priority\":\"high\",\"success_criteria\":[\"done\"]" +
			"}]," +
			"\"non_functional\":{\"performance\":[\"p\"],\"security\":[\"s\"],\"scalability\":[\"c\"]}," +
			"\"acceptance_criteria\":[{\"id\":\"\",\"description\":\"Given...\",\"satisfied\":false}]," +
			"\"milestones\":[{\"id\":\"ms-001\",\"name\":\"M1\",\"features\":[\"feature-1\"],\"target\":\"soon\",\"description\":\"desc\"}]" +
			"}\nend",
	}

	m.registry.defaultFn = func() (llm.Provider, error) {
		return provider, nil
	}
	m.specs.saveFn = func(ctx context.Context, spec *domain.ProductSpec) error {
		return nil
	}

	body := []byte(`{"name":"Demo","description":"Test spec"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/generate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusCreated)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["saved"] != true {
		t.Errorf("saved = %v; want true", resp["saved"])
	}

	spec, ok := resp["spec"].(map[string]interface{})
	if !ok {
		t.Fatalf("spec missing or wrong type")
	}
	criteria, ok := spec["acceptance_criteria"].([]interface{})
	if !ok || len(criteria) == 0 {
		t.Fatalf("acceptance_criteria missing")
	}
	first, ok := criteria[0].(map[string]interface{})
	if !ok {
		t.Fatalf("acceptance_criteria item invalid")
	}
	if first["id"] == "" {
		t.Errorf("acceptance criteria id should be set")
	}
}

func TestHandleGenerateSpec_WithGoals(t *testing.T) {
	m := newServerWithMocks()

	provider := &mockProvider{
		name: "mock",
		resp: "result:\n{" +
			"\"name\":\"Demo\"," +
			"\"version\":\"0.1.0\"," +
			"\"goals\":[\"goal1\",\"goal2\"]," +
			"\"features\":[{" +
			"\"id\":\"feature-1\",\"title\":\"Feature\",\"description\":\"desc\",\"priority\":\"high\",\"success_criteria\":[\"done\"]" +
			"}]," +
			"\"non_functional\":{\"performance\":[\"p\"],\"security\":[\"s\"],\"scalability\":[\"c\"]}," +
			"\"acceptance_criteria\":[{\"id\":\"ac-1\",\"description\":\"Given...\",\"satisfied\":false}]," +
			"\"milestones\":[{\"id\":\"ms-001\",\"name\":\"M1\",\"features\":[\"feature-1\"],\"target\":\"soon\",\"description\":\"desc\"}]" +
			"}\nend",
	}

	m.registry.defaultFn = func() (llm.Provider, error) {
		return provider, nil
	}
	m.specs.saveFn = func(ctx context.Context, spec *domain.ProductSpec) error {
		return nil
	}

	body := []byte(`{"name":"Demo","description":"Test","goals":["goal1","goal2"]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/generate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusCreated)
	}
}

func TestHandleGenerateSpec_WithContext(t *testing.T) {
	m := newServerWithMocks()

	provider := &mockProvider{
		name: "mock",
		resp: "result:\n{" +
			"\"name\":\"Demo\"," +
			"\"version\":\"0.1.0\"," +
			"\"goals\":[\"g1\"]," +
			"\"features\":[{" +
			"\"id\":\"feature-1\",\"title\":\"Feature\",\"description\":\"desc\",\"priority\":\"high\",\"success_criteria\":[\"done\"]" +
			"}]," +
			"\"non_functional\":{\"performance\":[\"p\"],\"security\":[\"s\"],\"scalability\":[\"c\"]}," +
			"\"acceptance_criteria\":[{\"id\":\"ac-1\",\"description\":\"Given...\",\"satisfied\":false}]," +
			"\"milestones\":[{\"id\":\"ms-001\",\"name\":\"M1\",\"features\":[\"feature-1\"],\"target\":\"soon\",\"description\":\"desc\"}]" +
			"}\nend",
	}

	m.registry.defaultFn = func() (llm.Provider, error) {
		return provider, nil
	}
	m.specs.saveFn = func(ctx context.Context, spec *domain.ProductSpec) error {
		return nil
	}

	body := []byte(`{"name":"Demo","description":"Test","context":"extra context"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/specs/generate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusCreated)
	}
}
