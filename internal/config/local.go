package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LocalConfig holds configuration for local daemon mode
type LocalConfig struct {
	Daemon   DaemonConfig   `yaml:"daemon"`
	LLM      LLMConfig      `yaml:"llm"`
	Learning LearningConfig `yaml:"learning_contract"`
	Runner   RunnerConfig   `yaml:"runner"`
}

// DaemonConfig holds daemon server settings
type DaemonConfig struct {
	Port     int    `yaml:"port"`
	Bind     string `yaml:"bind"`
	LogLevel string `yaml:"log_level"`
}

// LLMConfig holds LLM provider settings
type LLMConfig struct {
	DefaultProvider string                     `yaml:"default_provider"`
	Providers       map[string]*ProviderConfig `yaml:"providers"`
}

// ProviderConfig holds settings for a single LLM provider
type ProviderConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"`
	URL     string `yaml:"url,omitempty"` // For Ollama
	APIKey  string `yaml:"-"`             // Loaded from secrets.yaml
}

// LearningConfig holds learning contract settings
type LearningConfig struct {
	DefaultTrack string                `yaml:"default_track"`
	Tracks       map[string]TrackConfig `yaml:"tracks"`
}

// TrackConfig holds settings for a learning track
type TrackConfig struct {
	MaxLevel        int `yaml:"max_level"`
	CooldownSeconds int `yaml:"cooldown_seconds"`
}

// RunnerConfig holds code execution settings
type RunnerConfig struct {
	Executor string             `yaml:"executor"`
	Docker   DockerRunnerConfig `yaml:"docker"`
}

// DockerRunnerConfig holds Docker executor settings
type DockerRunnerConfig struct {
	Image          string `yaml:"image"`
	MemoryMB       int    `yaml:"memory_mb"`
	CPULimit       float64 `yaml:"cpu_limit"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	NetworkOff     bool   `yaml:"network_off"`
}

// SecretsConfig holds API keys loaded from secrets.yaml
type SecretsConfig struct {
	Providers map[string]struct {
		APIKey string `yaml:"api_key"`
	} `yaml:"providers"`
}

// TemperDir returns the path to ~/.temper
func TemperDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".temper"), nil
}

// EnsureTemperDir creates ~/.temper and subdirectories if they don't exist
func EnsureTemperDir() (string, error) {
	dir, err := TemperDir()
	if err != nil {
		return "", err
	}

	subdirs := []string{
		"",
		"logs",
		"profiles",
		"sessions",
		"exercises",
		"cache",
	}

	for _, subdir := range subdirs {
		path := filepath.Join(dir, subdir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return "", fmt.Errorf("create dir %s: %w", path, err)
		}
	}

	return dir, nil
}

// DefaultLocalConfig returns sensible defaults for local mode
func DefaultLocalConfig() *LocalConfig {
	return &LocalConfig{
		Daemon: DaemonConfig{
			Port:     7432,
			Bind:     "127.0.0.1",
			LogLevel: "info",
		},
		LLM: LLMConfig{
			DefaultProvider: "auto",
			Providers: map[string]*ProviderConfig{
				"claude": {
					Enabled: true,
					Model:   "claude-sonnet-4-20250514",
				},
				"openai": {
					Enabled: false,
					Model:   "gpt-4o",
				},
				"ollama": {
					Enabled: true,
					URL:     "http://localhost:11434",
					Model:   "llama2",
				},
			},
		},
		Learning: LearningConfig{
			DefaultTrack: "practice",
			Tracks: map[string]TrackConfig{
				"practice": {
					MaxLevel:        3,
					CooldownSeconds: 60,
				},
				"interview_prep": {
					MaxLevel:        2,
					CooldownSeconds: 120,
				},
				"learning": {
					MaxLevel:        4,
					CooldownSeconds: 30,
				},
			},
		},
		Runner: RunnerConfig{
			Executor: "docker",
			Docker: DockerRunnerConfig{
				Image:          "golang:1.23-alpine",
				MemoryMB:       384,
				CPULimit:       0.5,
				TimeoutSeconds: 30,
				NetworkOff:     true,
			},
		},
	}
}

// LoadLocalConfig loads configuration from ~/.temper/config.yaml
func LoadLocalConfig() (*LocalConfig, error) {
	dir, err := TemperDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(dir, "config.yaml")

	// If config doesn't exist, return defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultLocalConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := DefaultLocalConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Load secrets (API keys)
	if err := loadSecrets(dir, cfg); err != nil {
		return nil, fmt.Errorf("load secrets: %w", err)
	}

	return cfg, nil
}

// loadSecrets loads API keys from secrets.yaml
func loadSecrets(dir string, cfg *LocalConfig) error {
	secretsPath := filepath.Join(dir, "secrets.yaml")

	// If secrets file doesn't exist, skip
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(secretsPath)
	if err != nil {
		return fmt.Errorf("read secrets: %w", err)
	}

	var secrets SecretsConfig
	if err := yaml.Unmarshal(data, &secrets); err != nil {
		return fmt.Errorf("parse secrets: %w", err)
	}

	// Apply secrets to config
	for name, secret := range secrets.Providers {
		if provider, ok := cfg.LLM.Providers[name]; ok {
			provider.APIKey = secret.APIKey
		}
	}

	return nil
}

// SaveLocalConfig saves configuration to ~/.temper/config.yaml
func SaveLocalConfig(cfg *LocalConfig) error {
	dir, err := EnsureTemperDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(dir, "config.yaml")

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// SaveSecrets saves API keys to ~/.temper/secrets.yaml
func SaveSecrets(secrets map[string]string) error {
	dir, err := EnsureTemperDir()
	if err != nil {
		return err
	}

	secretsPath := filepath.Join(dir, "secrets.yaml")

	secretsCfg := SecretsConfig{
		Providers: make(map[string]struct {
			APIKey string `yaml:"api_key"`
		}),
	}

	for name, key := range secrets {
		secretsCfg.Providers[name] = struct {
			APIKey string `yaml:"api_key"`
		}{APIKey: key}
	}

	data, err := yaml.Marshal(secretsCfg)
	if err != nil {
		return fmt.Errorf("marshal secrets: %w", err)
	}

	// Write with restricted permissions (owner read/write only)
	if err := os.WriteFile(secretsPath, data, 0600); err != nil {
		return fmt.Errorf("write secrets: %w", err)
	}

	return nil
}
