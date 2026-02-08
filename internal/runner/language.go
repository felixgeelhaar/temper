package runner

import (
	"context"
	"fmt"
)

// Language represents a supported programming language
type Language string

const (
	LanguageGo         Language = "go"
	LanguagePython     Language = "python"
	LanguageTypeScript Language = "typescript"
	LanguageRust       Language = "rust"
	LanguageJava       Language = "java"
	LanguageC          Language = "c"
	LanguageCPP        Language = "cpp"
)

// IsValid checks if the language is supported
func (l Language) IsValid() bool {
	switch l {
	case LanguageGo, LanguagePython, LanguageTypeScript, LanguageRust, LanguageJava, LanguageC, LanguageCPP:
		return true
	default:
		return false
	}
}

// String returns the language as a string
func (l Language) String() string {
	return string(l)
}

// ParseLanguage converts a string to a Language
func ParseLanguage(s string) (Language, error) {
	lang := Language(s)
	if !lang.IsValid() {
		return "", fmt.Errorf("unsupported language: %s", s)
	}
	return lang, nil
}

// LanguageConfig contains language-specific configuration
type LanguageConfig struct {
	DockerImage    string
	FormatCommand  []string
	BuildCommand   []string
	TestCommand    []string
	FileExtensions []string
	InitFiles      map[string]string // e.g., go.mod, package.json
}

// DefaultLanguageConfigs returns default configurations for all supported languages
func DefaultLanguageConfigs() map[Language]LanguageConfig {
	return map[Language]LanguageConfig{
		LanguageGo: {
			DockerImage:    "golang:1.23-alpine",
			FormatCommand:  []string{"gofmt", "-d"},
			BuildCommand:   []string{"go", "build", "./..."},
			TestCommand:    []string{"go", "test", "-json", "./..."},
			FileExtensions: []string{".go"},
			InitFiles: map[string]string{
				"go.mod": "module exercise\n\ngo 1.22\n",
			},
		},
		LanguagePython: {
			DockerImage:    "python:3.12-alpine",
			FormatCommand:  []string{"python", "-m", "ruff", "format", "--check", "--diff"},
			BuildCommand:   []string{"python", "-m", "py_compile"},
			TestCommand:    []string{"python", "-m", "pytest", "--tb=short", "-v"},
			FileExtensions: []string{".py"},
			InitFiles: map[string]string{
				"requirements.txt": "pytest>=8.0\nruff>=0.1\n",
			},
		},
		LanguageTypeScript: {
			DockerImage:    "node:22-alpine",
			FormatCommand:  []string{"npx", "prettier", "--check"},
			BuildCommand:   []string{"npx", "tsc", "--noEmit"},
			TestCommand:    []string{"npx", "vitest", "run", "--reporter=json"},
			FileExtensions: []string{".ts", ".tsx"},
			InitFiles: map[string]string{
				"package.json":  `{"type":"module","devDependencies":{"typescript":"^5.0","vitest":"^1.0","prettier":"^3.0"}}`,
				"tsconfig.json": `{"compilerOptions":{"target":"ES2022","module":"ESNext","strict":true,"moduleResolution":"bundler"}}`,
			},
		},
		LanguageRust: {
			DockerImage:    "rust:1.75-alpine",
			FormatCommand:  []string{"rustfmt", "--check"},
			BuildCommand:   []string{"cargo", "build"},
			TestCommand:    []string{"cargo", "test", "--", "--format=json", "-Z", "unstable-options"},
			FileExtensions: []string{".rs"},
			InitFiles: map[string]string{
				"Cargo.toml": "[package]\nname = \"exercise\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
			},
		},
		LanguageJava: {
			DockerImage:    "eclipse-temurin:21-alpine",
			FormatCommand:  []string{"google-java-format", "--dry-run", "--set-exit-if-changed"},
			BuildCommand:   []string{"javac", "-d", "out"},
			TestCommand:    []string{"java", "-jar", "junit-platform-console-standalone.jar", "--class-path", "out", "--scan-classpath"},
			FileExtensions: []string{".java"},
			InitFiles:      map[string]string{},
		},
		LanguageC: {
			DockerImage:    "gcc:13-alpine",
			FormatCommand:  []string{"clang-format", "--dry-run", "-Werror"},
			BuildCommand:   []string{"gcc", "-Wall", "-Wextra", "-o", "exercise"},
			TestCommand:    []string{"./exercise"},
			FileExtensions: []string{".c", ".h"},
			InitFiles:      map[string]string{},
		},
		LanguageCPP: {
			DockerImage:    "gcc:13-alpine",
			FormatCommand:  []string{"clang-format", "--dry-run", "-Werror"},
			BuildCommand:   []string{"g++", "-std=c++17", "-Wall", "-Wextra", "-o", "exercise"},
			TestCommand:    []string{"./exercise"},
			FileExtensions: []string{".cpp", ".hpp", ".cc", ".h"},
			InitFiles:      map[string]string{},
		},
	}
}

// LanguageExecutor handles execution for a specific language
type LanguageExecutor interface {
	// Language returns the language this executor handles
	Language() Language

	// Format checks code formatting
	Format(ctx context.Context, code map[string]string) (*FormatResult, error)

	// FormatFix formats code and returns the fixed version
	FormatFix(ctx context.Context, code map[string]string) (map[string]string, error)

	// Build compiles/type-checks the code
	Build(ctx context.Context, code map[string]string) (*BuildResult, error)

	// Test runs tests and returns results
	Test(ctx context.Context, code map[string]string, flags []string) (*TestResult, error)
}

// ExecutorRegistry manages language executors
type ExecutorRegistry struct {
	executors map[Language]LanguageExecutor
	configs   map[Language]LanguageConfig
}

// NewExecutorRegistry creates a new executor registry
func NewExecutorRegistry() *ExecutorRegistry {
	return &ExecutorRegistry{
		executors: make(map[Language]LanguageExecutor),
		configs:   DefaultLanguageConfigs(),
	}
}

// Register adds an executor to the registry
func (r *ExecutorRegistry) Register(exec LanguageExecutor) {
	r.executors[exec.Language()] = exec
}

// Get returns the executor for a language
func (r *ExecutorRegistry) Get(lang Language) (LanguageExecutor, error) {
	exec, ok := r.executors[lang]
	if !ok {
		return nil, fmt.Errorf("no executor registered for language: %s", lang)
	}
	return exec, nil
}

// Config returns the configuration for a language
func (r *ExecutorRegistry) Config(lang Language) (LanguageConfig, bool) {
	cfg, ok := r.configs[lang]
	return cfg, ok
}

// SupportedLanguages returns all languages with registered executors
func (r *ExecutorRegistry) SupportedLanguages() []Language {
	langs := make([]Language, 0, len(r.executors))
	for lang := range r.executors {
		langs = append(langs, lang)
	}
	return langs
}
