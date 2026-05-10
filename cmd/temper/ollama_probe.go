package main

import (
	"net/http"
	"time"

	"github.com/felixgeelhaar/temper/internal/config"
)

// defaultOllamaURL returns the Ollama base URL from config, or
// http://localhost:11434 if no config is loaded yet.
func defaultOllamaURL(cfg *config.LocalConfig) string {
	if cfg != nil {
		if p, ok := cfg.LLM.Providers["ollama"]; ok && p != nil && p.URL != "" {
			return p.URL
		}
	}
	return "http://localhost:11434"
}

// isOllamaReachable returns true when the /api/tags endpoint responds
// with HTTP 200 within a short timeout. Used during `temper init` to
// pick a zero-friction default when the user already has Ollama up.
func isOllamaReachable(url string) bool {
	client := &http.Client{Timeout: 750 * time.Millisecond}
	resp, err := client.Get(url + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
