package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/felixgeelhaar/temper/internal/config"
)

func TestDefaultOllamaURL_FallbackWhenNoConfig(t *testing.T) {
	if got := defaultOllamaURL(nil); got != "http://localhost:11434" {
		t.Errorf("nil config → got %q, want default", got)
	}
}

func TestDefaultOllamaURL_FromConfig(t *testing.T) {
	cfg := &config.LocalConfig{
		LLM: config.LLMConfig{
			Providers: map[string]*config.ProviderConfig{
				"ollama": {URL: "http://192.168.1.10:11434"},
			},
		},
	}
	if got := defaultOllamaURL(cfg); got != "http://192.168.1.10:11434" {
		t.Errorf("config URL → got %q", got)
	}
}

func TestDefaultOllamaURL_FallbackWhenURLEmpty(t *testing.T) {
	cfg := &config.LocalConfig{
		LLM: config.LLMConfig{
			Providers: map[string]*config.ProviderConfig{
				"ollama": {URL: ""},
			},
		},
	}
	if got := defaultOllamaURL(cfg); got != "http://localhost:11434" {
		t.Errorf("empty URL → got %q, want default", got)
	}
}

func TestIsOllamaReachable_TrueOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if !isOllamaReachable(srv.URL) {
		t.Error("expected reachable when /api/tags returns 200")
	}
}

func TestIsOllamaReachable_FalseOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if isOllamaReachable(srv.URL) {
		t.Error("expected unreachable when status != 200")
	}
}

func TestIsOllamaReachable_FalseOnNoServer(t *testing.T) {
	// Pick a port nobody listens on locally.
	if isOllamaReachable("http://127.0.0.1:1") {
		t.Error("expected unreachable for closed port")
	}
}
