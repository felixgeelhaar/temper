package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/felixgeelhaar/temper/internal/config"
	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/felixgeelhaar/temper/internal/profile"
)

func TestHandleReady_Degraded(t *testing.T) {
	m := newServerWithMocks()
	m.registry.listFn = func() []string { return nil }
	m.server.runnerExecutor = nil
	m.server.cfg = config.DefaultLocalConfig()

	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "degraded" {
		t.Errorf("status = %v; want degraded", resp["status"])
	}
}

func TestHandleReady_Ready(t *testing.T) {
	m := newServerWithMocks()
	m.registry.listFn = func() []string { return []string{"mock"} }
	m.server.cfg = config.DefaultLocalConfig()

	req := httptest.NewRequest(http.MethodGet, "/v1/ready", nil)
	rec := httptest.NewRecorder()
	m.server.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestSetupLLMProviders(t *testing.T) {
	cfg := config.DefaultLocalConfig()
	cfg.LLM.Providers = map[string]*config.ProviderConfig{
		"claude": {Enabled: true, APIKey: ""},
		"ollama": {Enabled: true, URL: "http://localhost:11434", Model: "llama"},
	}

	s := &Server{cfg: cfg}
	reg := llm.NewRegistry()

	if err := s.setupLLMProviders(reg); err != nil {
		t.Fatalf("setupLLMProviders() error = %v", err)
	}

	found := false
	for _, name := range reg.List() {
		if name == "ollama" {
			found = true
		}
	}
	if !found {
		t.Error("setupLLMProviders() should register ollama")
	}
}

func TestSetupLLMProviders_ClaudeDisabled(t *testing.T) {
	cfg := config.DefaultLocalConfig()
	cfg.LLM.Providers = map[string]*config.ProviderConfig{
		"claude": {Enabled: false, APIKey: "key"},
		"ollama": {Enabled: true, URL: "http://localhost:11434", Model: "llama"},
	}

	s := &Server{cfg: cfg}
	reg := llm.NewRegistry()

	if err := s.setupLLMProviders(reg); err != nil {
		t.Fatalf("setupLLMProviders() error = %v", err)
	}

	found := false
	for _, name := range reg.List() {
		if name == "ollama" {
			found = true
		}
	}
	if !found {
		t.Error("setupLLMProviders() should register ollama when claude is disabled")
	}
}

func TestSetupLLMProviders_OpenAI(t *testing.T) {
	cfg := config.DefaultLocalConfig()
	cfg.LLM.Providers = map[string]*config.ProviderConfig{
		"openai": {Enabled: true, APIKey: "test-key", Model: "gpt-4"},
	}

	s := &Server{cfg: cfg}
	reg := llm.NewRegistry()

	if err := s.setupLLMProviders(reg); err != nil {
		t.Fatalf("setupLLMProviders() error = %v", err)
	}

	found := false
	for _, name := range reg.List() {
		if name == "openai" {
			found = true
		}
	}
	if !found {
		t.Error("setupLLMProviders() should register openai")
	}
}

func TestSetupLLMProviders_Claude(t *testing.T) {
	cfg := config.DefaultLocalConfig()
	cfg.LLM.Providers = map[string]*config.ProviderConfig{
		"claude": {Enabled: true, APIKey: "test-key", Model: "claude-3"},
	}

	s := &Server{cfg: cfg}
	reg := llm.NewRegistry()

	if err := s.setupLLMProviders(reg); err != nil {
		t.Fatalf("setupLLMProviders() error = %v", err)
	}

	found := false
	for _, name := range reg.List() {
		if name == "claude" {
			found = true
		}
	}
	if !found {
		t.Error("setupLLMProviders() should register claude")
	}
}

func TestSetupLLMProviders_OpenAIEmptyKey(t *testing.T) {
	cfg := config.DefaultLocalConfig()
	cfg.LLM.Providers = map[string]*config.ProviderConfig{
		"openai": {Enabled: true, APIKey: "", Model: "gpt-4"},
	}

	s := &Server{cfg: cfg}
	reg := llm.NewRegistry()

	if err := s.setupLLMProviders(reg); err != nil {
		t.Fatalf("setupLLMProviders() error = %v", err)
	}

	if len(reg.List()) != 0 {
		t.Errorf("setupLLMProviders() should not register openai without key")
	}
}

func TestConvertToDomainProfile(t *testing.T) {
	m := newServerWithMocks()

	stored := &profile.StoredProfile{
		ID:             "default",
		TotalExercises: 2,
		TotalSessions:  3,
		TotalRuns:      4,
		HintRequests:   1,
		TopicSkills: map[string]profile.StoredSkill{
			"go/basics": {Level: 0.6, Attempts: 2},
		},
	}

	result := m.server.convertToDomainProfile(stored)
	if result.TotalExercises != 2 || result.TotalRuns != 4 {
		t.Errorf("convertToDomainProfile totals mismatch: %#v", result)
	}
	if _, ok := result.TopicSkills["go/basics"]; !ok {
		t.Error("convertToDomainProfile should map topic skills")
	}
	if result.GetSkillLevel("go/basics").Level == 0 {
		t.Error("convertToDomainProfile should preserve skill level")
	}
	if result.SuggestMaxLevel() == domain.L0Clarify {
		// Sanity check that profile is usable
		return
	}
}

func TestNewServer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "temper-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	localCfg := config.DefaultLocalConfig()
	localCfg.Storage.Path = filepath.Join(tmpDir, "temper.db")

	cfg := ServerConfig{
		Config: localCfg,
	}
	ctx := context.Background()

	srv, err := NewServer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if srv == nil {
		t.Fatal("NewServer() returned nil")
	}
}
