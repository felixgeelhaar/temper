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

