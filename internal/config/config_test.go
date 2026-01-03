package config

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{"returns default when not set", "TEST_KEY_UNSET", "default", "", "default"},
		{"returns env value when set", "TEST_KEY_SET", "default", "custom", "custom"},
		{"returns empty string env over default", "TEST_KEY_EMPTY", "default", "", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv(%q, %q) = %q, want %q", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		want         int
	}{
		{"returns default when not set", "TEST_INT_UNSET", 100, "", 100},
		{"parses valid int", "TEST_INT_VALID", 100, "42", 42},
		{"returns default on invalid int", "TEST_INT_INVALID", 100, "not-a-number", 100},
		{"parses negative int", "TEST_INT_NEG", 100, "-5", -5},
		{"parses zero", "TEST_INT_ZERO", 100, "0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvInt(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvInt(%q, %d) = %d, want %d", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestGetEnvFloat(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue float64
		envValue     string
		want         float64
	}{
		{"returns default when not set", "TEST_FLOAT_UNSET", 1.5, "", 1.5},
		{"parses valid float", "TEST_FLOAT_VALID", 1.5, "2.5", 2.5},
		{"returns default on invalid float", "TEST_FLOAT_INVALID", 1.5, "not-a-float", 1.5},
		{"parses int as float", "TEST_FLOAT_INT", 1.5, "3", 3.0},
		{"parses negative float", "TEST_FLOAT_NEG", 1.5, "-0.5", -0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvFloat(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvFloat(%q, %f) = %f, want %f", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue bool
		envValue     string
		want         bool
	}{
		{"returns default when not set", "TEST_BOOL_UNSET", true, "", true},
		{"parses true", "TEST_BOOL_TRUE", false, "true", true},
		{"parses false", "TEST_BOOL_FALSE", true, "false", false},
		{"parses 1 as true", "TEST_BOOL_ONE", false, "1", true},
		{"parses 0 as false", "TEST_BOOL_ZERO", true, "0", false},
		{"returns default on invalid bool", "TEST_BOOL_INVALID", true, "yes", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvBool(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvBool(%q, %v) = %v, want %v", tt.key, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Set DEBUG to true to avoid production validation
	os.Setenv("DEBUG", "true")
	defer os.Unsetenv("DEBUG")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check default values
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if !cfg.Debug {
		t.Error("Debug should be true when DEBUG=true")
	}
	if cfg.LLMProvider != "claude" {
		t.Errorf("LLMProvider = %q, want %q", cfg.LLMProvider, "claude")
	}
	if cfg.RunnerPoolSize != 3 {
		t.Errorf("RunnerPoolSize = %d, want 3", cfg.RunnerPoolSize)
	}
	if cfg.RunnerTimeout != 30 {
		t.Errorf("RunnerTimeout = %d, want 30", cfg.RunnerTimeout)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	// Set custom values
	envVars := map[string]string{
		"DEBUG":            "true",
		"PORT":             "9000",
		"LLM_PROVIDER":     "openai",
		"LLM_MODEL":        "gpt-4",
		"RUNNER_POOL_SIZE": "5",
		"RUNNER_TIMEOUT":   "60",
		"EXERCISES_PATH":   "/custom/exercises",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want 9000", cfg.Port)
	}
	if cfg.LLMProvider != "openai" {
		t.Errorf("LLMProvider = %q, want openai", cfg.LLMProvider)
	}
	if cfg.LLMModel != "gpt-4" {
		t.Errorf("LLMModel = %q, want gpt-4", cfg.LLMModel)
	}
	if cfg.RunnerPoolSize != 5 {
		t.Errorf("RunnerPoolSize = %d, want 5", cfg.RunnerPoolSize)
	}
	if cfg.ExercisesPath != "/custom/exercises" {
		t.Errorf("ExercisesPath = %q, want /custom/exercises", cfg.ExercisesPath)
	}
}

func TestLoad_ProductionValidation(t *testing.T) {
	// Clear DEBUG to simulate production
	os.Unsetenv("DEBUG")
	os.Unsetenv("SESSION_SECRET")

	_, err := Load()
	if err == nil {
		t.Error("Load() should error in production without SESSION_SECRET")
	}
}

func TestLoad_ProductionWithSecret(t *testing.T) {
	os.Unsetenv("DEBUG")
	os.Setenv("SESSION_SECRET", "a-real-production-secret")
	defer os.Unsetenv("SESSION_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SessionSecret != "a-real-production-secret" {
		t.Errorf("SessionSecret = %q, want production secret", cfg.SessionSecret)
	}
}
