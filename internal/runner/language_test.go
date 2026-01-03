package runner

import (
	"testing"
)

func TestLanguage_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		lang     Language
		expected bool
	}{
		{"go is valid", LanguageGo, true},
		{"python is valid", LanguagePython, true},
		{"typescript is valid", LanguageTypeScript, true},
		{"rust is valid", LanguageRust, true},
		{"empty is invalid", Language(""), false},
		{"unknown is invalid", Language("java"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.lang.IsValid()
			if got != tc.expected {
				t.Errorf("Language(%q).IsValid() = %v; want %v", tc.lang, got, tc.expected)
			}
		})
	}
}

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  Language
		expectErr bool
	}{
		{"parse go", "go", LanguageGo, false},
		{"parse python", "python", LanguagePython, false},
		{"parse typescript", "typescript", LanguageTypeScript, false},
		{"parse rust", "rust", LanguageRust, false},
		{"parse invalid", "java", "", true},
		{"parse empty", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLanguage(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Errorf("ParseLanguage(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseLanguage(%q) unexpected error: %v", tc.input, err)
				return
			}
			if got != tc.expected {
				t.Errorf("ParseLanguage(%q) = %v; want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestDefaultLanguageConfigs(t *testing.T) {
	configs := DefaultLanguageConfigs()

	languages := []Language{LanguageGo, LanguagePython, LanguageTypeScript, LanguageRust}
	for _, lang := range languages {
		t.Run(string(lang), func(t *testing.T) {
			cfg, ok := configs[lang]
			if !ok {
				t.Fatalf("missing config for language: %s", lang)
			}
			if cfg.DockerImage == "" {
				t.Errorf("DockerImage is empty for %s", lang)
			}
			if len(cfg.FileExtensions) == 0 {
				t.Errorf("FileExtensions is empty for %s", lang)
			}
		})
	}
}

func TestExecutorRegistry(t *testing.T) {
	registry := NewExecutorRegistry()

	// Register Go executor
	goExec := NewGoExecutor(false)
	registry.Register(goExec)

	// Test Get
	t.Run("get registered executor", func(t *testing.T) {
		exec, err := registry.Get(LanguageGo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if exec.Language() != LanguageGo {
			t.Errorf("expected Go executor, got %s", exec.Language())
		}
	})

	t.Run("get unregistered executor", func(t *testing.T) {
		_, err := registry.Get(LanguageRust)
		if err == nil {
			t.Error("expected error for unregistered executor")
		}
	})

	// Test Config
	t.Run("get config", func(t *testing.T) {
		cfg, ok := registry.Config(LanguageGo)
		if !ok {
			t.Fatal("expected config for Go")
		}
		if cfg.DockerImage == "" {
			t.Error("expected DockerImage to be set")
		}
	})

	// Test SupportedLanguages
	t.Run("supported languages", func(t *testing.T) {
		langs := registry.SupportedLanguages()
		if len(langs) != 1 {
			t.Errorf("expected 1 language, got %d", len(langs))
		}
		if langs[0] != LanguageGo {
			t.Errorf("expected Go, got %s", langs[0])
		}
	})
}

func TestGoExecutor_Language(t *testing.T) {
	exec := NewGoExecutor(false)
	if exec.Language() != LanguageGo {
		t.Errorf("expected Go, got %s", exec.Language())
	}
}

func TestPythonExecutor_Language(t *testing.T) {
	exec := NewPythonExecutor()
	if exec.Language() != LanguagePython {
		t.Errorf("expected Python, got %s", exec.Language())
	}
}

func TestTypeScriptExecutor_Language(t *testing.T) {
	exec := NewTypeScriptExecutor()
	if exec.Language() != LanguageTypeScript {
		t.Errorf("expected TypeScript, got %s", exec.Language())
	}
}

func TestRustExecutor_Language(t *testing.T) {
	exec := NewRustExecutor()
	if exec.Language() != LanguageRust {
		t.Errorf("expected Rust, got %s", exec.Language())
	}
}
