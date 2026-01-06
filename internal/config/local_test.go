package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTemperDir(t *testing.T) {
	dir, err := TemperDir()
	if err != nil {
		t.Fatalf("TemperDir() error = %v", err)
	}

	// Should end with .temper
	if filepath.Base(dir) != ".temper" {
		t.Errorf("TemperDir() = %q, want ending with .temper", dir)
	}

	// Should be an absolute path
	if !filepath.IsAbs(dir) {
		t.Errorf("TemperDir() = %q, want absolute path", dir)
	}
}

func TestEnsureTemperDir(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	dir, err := EnsureTemperDir()
	if err != nil {
		t.Fatalf("EnsureTemperDir() error = %v", err)
	}

	// Verify directory was created
	expectedDir := filepath.Join(tmpHome, ".temper")
	if dir != expectedDir {
		t.Errorf("EnsureTemperDir() = %q, want %q", dir, expectedDir)
	}

	// Verify subdirectories exist
	subdirs := []string{"logs", "profiles", "sessions", "exercises", "cache"}
	for _, subdir := range subdirs {
		path := filepath.Join(dir, subdir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("EnsureTemperDir() should create %s", subdir)
		}
	}
}

func TestDefaultLocalConfig(t *testing.T) {
	cfg := DefaultLocalConfig()
	if cfg == nil {
		t.Fatal("DefaultLocalConfig() returned nil")
	}

	// Verify daemon defaults
	if cfg.Daemon.Port != 7432 {
		t.Errorf("Daemon.Port = %d, want 7432", cfg.Daemon.Port)
	}
	if cfg.Daemon.Bind != "127.0.0.1" {
		t.Errorf("Daemon.Bind = %q, want 127.0.0.1", cfg.Daemon.Bind)
	}
	if cfg.Daemon.LogLevel != "info" {
		t.Errorf("Daemon.LogLevel = %q, want info", cfg.Daemon.LogLevel)
	}

	// Verify LLM defaults
	if cfg.LLM.DefaultProvider != "auto" {
		t.Errorf("LLM.DefaultProvider = %q, want auto", cfg.LLM.DefaultProvider)
	}
	if len(cfg.LLM.Providers) != 3 {
		t.Errorf("LLM.Providers count = %d, want 3", len(cfg.LLM.Providers))
	}
	if claude, ok := cfg.LLM.Providers["claude"]; !ok {
		t.Error("LLM.Providers should include claude")
	} else if !claude.Enabled {
		t.Error("claude provider should be enabled by default")
	}

	// Verify learning defaults
	if cfg.Learning.DefaultTrack != "practice" {
		t.Errorf("Learning.DefaultTrack = %q, want practice", cfg.Learning.DefaultTrack)
	}
	if _, ok := cfg.Learning.Tracks["practice"]; !ok {
		t.Error("Learning.Tracks should include practice")
	}
	if cfg.Learning.Tracks["practice"].MaxLevel != 3 {
		t.Errorf("practice track MaxLevel = %d, want 3", cfg.Learning.Tracks["practice"].MaxLevel)
	}

	// Verify runner defaults
	if cfg.Runner.Executor != "docker" {
		t.Errorf("Runner.Executor = %q, want docker", cfg.Runner.Executor)
	}
	if cfg.Runner.AllowLocalFallback {
		t.Error("Runner.AllowLocalFallback should be false by default")
	}
	if cfg.Runner.Docker.MemoryMB != 384 {
		t.Errorf("Runner.Docker.MemoryMB = %d, want 384", cfg.Runner.Docker.MemoryMB)
	}
	if !cfg.Runner.Docker.NetworkOff {
		t.Error("Runner.Docker.NetworkOff should be true by default")
	}
}

func TestDefaultLocalConfig_ProviderDetails(t *testing.T) {
	cfg := DefaultLocalConfig()

	tests := []struct {
		name    string
		enabled bool
		model   string
		url     string
	}{
		{"claude", true, "claude-sonnet-4-20250514", ""},
		{"openai", false, "gpt-4o", ""},
		{"ollama", true, "llama2", "http://localhost:11434"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, ok := cfg.LLM.Providers[tt.name]
			if !ok {
				t.Fatalf("Provider %q not found", tt.name)
			}
			if provider.Enabled != tt.enabled {
				t.Errorf("Provider.Enabled = %v, want %v", provider.Enabled, tt.enabled)
			}
			if provider.Model != tt.model {
				t.Errorf("Provider.Model = %q, want %q", provider.Model, tt.model)
			}
			if provider.URL != tt.url {
				t.Errorf("Provider.URL = %q, want %q", provider.URL, tt.url)
			}
		})
	}
}

func TestDefaultLocalConfig_TrackDetails(t *testing.T) {
	cfg := DefaultLocalConfig()

	tests := []struct {
		name     string
		maxLevel int
		cooldown int
	}{
		{"practice", 3, 60},
		{"interview_prep", 2, 120},
		{"learning", 4, 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			track, ok := cfg.Learning.Tracks[tt.name]
			if !ok {
				t.Fatalf("Track %q not found", tt.name)
			}
			if track.MaxLevel != tt.maxLevel {
				t.Errorf("Track.MaxLevel = %d, want %d", track.MaxLevel, tt.maxLevel)
			}
			if track.CooldownSeconds != tt.cooldown {
				t.Errorf("Track.CooldownSeconds = %d, want %d", track.CooldownSeconds, tt.cooldown)
			}
		})
	}
}

func TestLoadSecrets(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test config
	cfg := DefaultLocalConfig()

	// Create secrets file
	secretsContent := `providers:
  claude:
    api_key: sk-claude-test-key
  openai:
    api_key: sk-openai-test-key
`
	secretsPath := filepath.Join(tmpDir, "secrets.yaml")
	if err := os.WriteFile(secretsPath, []byte(secretsContent), 0600); err != nil {
		t.Fatalf("Failed to write secrets file: %v", err)
	}

	// Call loadSecrets
	if err := loadSecrets(tmpDir, cfg); err != nil {
		t.Fatalf("loadSecrets() error = %v", err)
	}

	// Verify API keys were loaded
	if cfg.LLM.Providers["claude"].APIKey != "sk-claude-test-key" {
		t.Errorf("claude APIKey = %q, want sk-claude-test-key", cfg.LLM.Providers["claude"].APIKey)
	}
	if cfg.LLM.Providers["openai"].APIKey != "sk-openai-test-key" {
		t.Errorf("openai APIKey = %q, want sk-openai-test-key", cfg.LLM.Providers["openai"].APIKey)
	}
	// Ollama should have no API key
	if cfg.LLM.Providers["ollama"].APIKey != "" {
		t.Errorf("ollama APIKey = %q, want empty", cfg.LLM.Providers["ollama"].APIKey)
	}
}

func TestLoadSecrets_NoSecretsFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultLocalConfig()

	// No secrets file exists
	if err := loadSecrets(tmpDir, cfg); err != nil {
		t.Errorf("loadSecrets() should not error when secrets file is missing: %v", err)
	}
}

func TestLoadSecrets_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultLocalConfig()

	// Create invalid YAML secrets file
	secretsPath := filepath.Join(tmpDir, "secrets.yaml")
	if err := os.WriteFile(secretsPath, []byte("invalid: yaml: content:"), 0600); err != nil {
		t.Fatalf("Failed to write secrets file: %v", err)
	}

	if err := loadSecrets(tmpDir, cfg); err == nil {
		t.Error("loadSecrets() should error on invalid YAML")
	}
}

func TestLoadSecrets_UnknownProvider(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultLocalConfig()

	// Create secrets with unknown provider
	secretsContent := `providers:
  unknown_provider:
    api_key: some-key
`
	secretsPath := filepath.Join(tmpDir, "secrets.yaml")
	if err := os.WriteFile(secretsPath, []byte(secretsContent), 0600); err != nil {
		t.Fatalf("Failed to write secrets file: %v", err)
	}

	// Should not error, just ignore unknown providers
	if err := loadSecrets(tmpDir, cfg); err != nil {
		t.Errorf("loadSecrets() should not error on unknown provider: %v", err)
	}
}

func TestLoadLocalConfig_DefaultsWhenNoFile(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	// Create .temper directory (but no config.yaml)
	if err := os.MkdirAll(filepath.Join(tmpHome, ".temper"), 0755); err != nil {
		t.Fatalf("Failed to create .temper dir: %v", err)
	}

	cfg, err := LoadLocalConfig()
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	// Should return defaults
	if cfg.Daemon.Port != 7432 {
		t.Errorf("Daemon.Port = %d, want 7432 (default)", cfg.Daemon.Port)
	}
}

func TestLoadLocalConfig_WithConfigFile(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	// Create .temper directory and config file
	temperDir := filepath.Join(tmpHome, ".temper")
	if err := os.MkdirAll(temperDir, 0755); err != nil {
		t.Fatalf("Failed to create .temper dir: %v", err)
	}

	configContent := `daemon:
  port: 9999
  bind: "0.0.0.0"
  log_level: debug
llm:
  default_provider: openai
`
	configPath := filepath.Join(temperDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadLocalConfig()
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	if cfg.Daemon.Port != 9999 {
		t.Errorf("Daemon.Port = %d, want 9999", cfg.Daemon.Port)
	}
	if cfg.Daemon.Bind != "0.0.0.0" {
		t.Errorf("Daemon.Bind = %q, want 0.0.0.0", cfg.Daemon.Bind)
	}
	if cfg.Daemon.LogLevel != "debug" {
		t.Errorf("Daemon.LogLevel = %q, want debug", cfg.Daemon.LogLevel)
	}
	if cfg.LLM.DefaultProvider != "openai" {
		t.Errorf("LLM.DefaultProvider = %q, want openai", cfg.LLM.DefaultProvider)
	}
}

func TestLoadLocalConfig_WithSecrets(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	// Create .temper directory
	temperDir := filepath.Join(tmpHome, ".temper")
	if err := os.MkdirAll(temperDir, 0755); err != nil {
		t.Fatalf("Failed to create .temper dir: %v", err)
	}

	// Create config file
	configPath := filepath.Join(temperDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("daemon:\n  port: 7432\n"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create secrets file
	secretsContent := `providers:
  claude:
    api_key: test-api-key
`
	secretsPath := filepath.Join(temperDir, "secrets.yaml")
	if err := os.WriteFile(secretsPath, []byte(secretsContent), 0600); err != nil {
		t.Fatalf("Failed to write secrets file: %v", err)
	}

	cfg, err := LoadLocalConfig()
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	if cfg.LLM.Providers["claude"].APIKey != "test-api-key" {
		t.Errorf("claude APIKey = %q, want test-api-key", cfg.LLM.Providers["claude"].APIKey)
	}
}

func TestLoadLocalConfig_InvalidConfigYAML(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	// Create .temper directory and invalid config
	temperDir := filepath.Join(tmpHome, ".temper")
	if err := os.MkdirAll(temperDir, 0755); err != nil {
		t.Fatalf("Failed to create .temper dir: %v", err)
	}

	configPath := filepath.Join(temperDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("invalid: yaml: [broken"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := LoadLocalConfig()
	if err == nil {
		t.Error("LoadLocalConfig() should error on invalid YAML")
	}
}

func TestSaveLocalConfig(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	cfg := DefaultLocalConfig()
	cfg.Daemon.Port = 8888
	cfg.LLM.DefaultProvider = "ollama"

	if err := SaveLocalConfig(cfg); err != nil {
		t.Fatalf("SaveLocalConfig() error = %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tmpHome, ".temper", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	var loaded LocalConfig
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to parse saved config: %v", err)
	}

	if loaded.Daemon.Port != 8888 {
		t.Errorf("Saved Daemon.Port = %d, want 8888", loaded.Daemon.Port)
	}
	if loaded.LLM.DefaultProvider != "ollama" {
		t.Errorf("Saved LLM.DefaultProvider = %q, want ollama", loaded.LLM.DefaultProvider)
	}
}

func TestSaveSecrets(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	secrets := map[string]string{
		"claude": "sk-claude-secret",
		"openai": "sk-openai-secret",
	}

	if err := SaveSecrets(secrets); err != nil {
		t.Fatalf("SaveSecrets() error = %v", err)
	}

	// Verify file was created with correct permissions
	secretsPath := filepath.Join(tmpHome, ".temper", "secrets.yaml")
	info, err := os.Stat(secretsPath)
	if err != nil {
		t.Fatalf("Failed to stat secrets file: %v", err)
	}

	// Check permissions (should be 0600 - owner read/write only)
	if info.Mode().Perm() != 0600 {
		t.Errorf("Secrets file permissions = %o, want 0600", info.Mode().Perm())
	}

	// Verify content
	data, err := os.ReadFile(secretsPath)
	if err != nil {
		t.Fatalf("Failed to read secrets file: %v", err)
	}

	var loaded SecretsConfig
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to parse saved secrets: %v", err)
	}

	if loaded.Providers["claude"].APIKey != "sk-claude-secret" {
		t.Errorf("Saved claude APIKey = %q, want sk-claude-secret", loaded.Providers["claude"].APIKey)
	}
	if loaded.Providers["openai"].APIKey != "sk-openai-secret" {
		t.Errorf("Saved openai APIKey = %q, want sk-openai-secret", loaded.Providers["openai"].APIKey)
	}
}

func TestSaveSecrets_Empty(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	secrets := map[string]string{}

	if err := SaveSecrets(secrets); err != nil {
		t.Fatalf("SaveSecrets() error = %v", err)
	}

	// Verify file was created
	secretsPath := filepath.Join(tmpHome, ".temper", "secrets.yaml")
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		t.Error("SaveSecrets() should create file even for empty secrets")
	}
}

func TestRoundTrip_ConfigAndSecrets(t *testing.T) {
	// Save original HOME
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Use temp directory as HOME
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	// Save config
	cfg := DefaultLocalConfig()
	cfg.Daemon.Port = 7777
	cfg.Daemon.LogLevel = "debug"
	cfg.LLM.DefaultProvider = "openai"

	if err := SaveLocalConfig(cfg); err != nil {
		t.Fatalf("SaveLocalConfig() error = %v", err)
	}

	// Save secrets
	secrets := map[string]string{
		"claude": "roundtrip-claude-key",
		"openai": "roundtrip-openai-key",
	}
	if err := SaveSecrets(secrets); err != nil {
		t.Fatalf("SaveSecrets() error = %v", err)
	}

	// Load and verify
	loaded, err := LoadLocalConfig()
	if err != nil {
		t.Fatalf("LoadLocalConfig() error = %v", err)
	}

	if loaded.Daemon.Port != 7777 {
		t.Errorf("Round-trip Daemon.Port = %d, want 7777", loaded.Daemon.Port)
	}
	if loaded.Daemon.LogLevel != "debug" {
		t.Errorf("Round-trip Daemon.LogLevel = %q, want debug", loaded.Daemon.LogLevel)
	}
	if loaded.LLM.DefaultProvider != "openai" {
		t.Errorf("Round-trip LLM.DefaultProvider = %q, want openai", loaded.LLM.DefaultProvider)
	}
	if loaded.LLM.Providers["claude"].APIKey != "roundtrip-claude-key" {
		t.Errorf("Round-trip claude APIKey = %q, want roundtrip-claude-key", loaded.LLM.Providers["claude"].APIKey)
	}
	if loaded.LLM.Providers["openai"].APIKey != "roundtrip-openai-key" {
		t.Errorf("Round-trip openai APIKey = %q, want roundtrip-openai-key", loaded.LLM.Providers["openai"].APIKey)
	}
}

func TestLocalConfig_Structs(t *testing.T) {
	// Test struct field types and YAML tags
	cfg := &LocalConfig{
		Daemon: DaemonConfig{
			Port:     8080,
			Bind:     "localhost",
			LogLevel: "warn",
		},
		LLM: LLMConfig{
			DefaultProvider: "test",
			Providers: map[string]*ProviderConfig{
				"test": {
					Enabled: true,
					Model:   "test-model",
					URL:     "http://test",
					APIKey:  "test-key",
				},
			},
		},
		Learning: LearningConfig{
			DefaultTrack: "test-track",
			Tracks: map[string]TrackConfig{
				"test-track": {
					MaxLevel:        5,
					CooldownSeconds: 10,
				},
			},
		},
		Runner: RunnerConfig{
			Executor: "local",
			Docker: DockerRunnerConfig{
				Image:          "test-image",
				MemoryMB:       512,
				CPULimit:       1.0,
				TimeoutSeconds: 60,
				NetworkOff:     false,
			},
		},
	}

	// Marshal and unmarshal to verify YAML tags work
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}

	var loaded LocalConfig
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	// Verify fields survived round-trip
	if loaded.Daemon.Port != 8080 {
		t.Errorf("Daemon.Port = %d, want 8080", loaded.Daemon.Port)
	}
	if loaded.Runner.Docker.Image != "test-image" {
		t.Errorf("Runner.Docker.Image = %q, want test-image", loaded.Runner.Docker.Image)
	}

	// APIKey should NOT survive round-trip (yaml:"-" tag)
	// Load secrets would need to re-apply it
	if loaded.LLM.Providers["test"].APIKey != "" {
		t.Error("APIKey should not be serialized to YAML")
	}
}
