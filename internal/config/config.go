package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the application
type Config struct {
	// Server
	Port  int
	Debug bool

	// Database
	DatabaseURL string

	// RabbitMQ
	RabbitMQURL string

	// LLM
	LLMProvider string // claude, openai, ollama
	LLMAPIKey   string
	LLMModel    string
	OllamaURL   string

	// Runner
	RunnerPoolSize int
	RunnerTimeout  int // seconds
	RunnerMemoryMB int
	RunnerCPULimit float64
	RunnerImage    string

	// Session
	SessionSecret string
	SessionMaxAge int // seconds

	// Exercises
	ExercisesPath string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:             getEnvInt("PORT", 8080),
		Debug:            getEnvBool("DEBUG", false),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://temper:temper@localhost:5432/temper?sslmode=disable"),
		RabbitMQURL:      getEnv("RABBITMQ_URL", "amqp://temper:temper@localhost:5672/"),
		LLMProvider:      getEnv("LLM_PROVIDER", "claude"),
		LLMAPIKey:        getEnv("LLM_API_KEY", ""),
		LLMModel:         getEnv("LLM_MODEL", "claude-sonnet-4-20250514"),
		OllamaURL:        getEnv("OLLAMA_URL", "http://localhost:11434"),
		RunnerPoolSize: getEnvInt("RUNNER_POOL_SIZE", 3),
		RunnerTimeout:  getEnvInt("RUNNER_TIMEOUT", 30),
		RunnerMemoryMB: getEnvInt("RUNNER_MEMORY_MB", 256),
		RunnerCPULimit: getEnvFloat("RUNNER_CPU_LIMIT", 0.5),
		RunnerImage:    getEnv("RUNNER_IMAGE", "temper-runner-sandbox:latest"),
		SessionSecret:    getEnv("SESSION_SECRET", "change-me-in-production"),
		SessionMaxAge:    getEnvInt("SESSION_MAX_AGE", 86400*7), // 7 days
		ExercisesPath:    getEnv("EXERCISES_PATH", "./exercises"),
	}

	// Validate required settings
	if cfg.SessionSecret == "change-me-in-production" && !cfg.Debug {
		return nil, fmt.Errorf("SESSION_SECRET must be set in production")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}
