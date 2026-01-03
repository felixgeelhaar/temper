package profile

import "testing"

func TestExtractTopic(t *testing.T) {
	tests := []struct {
		exerciseID string
		want       string
	}{
		{"go-v1/basics/hello-world", "go/basics"},
		{"go-v1/interfaces/stringer", "go/interfaces"},
		{"python-v1/testing/pytest-basics", "python/testing"},
		{"typescript-v1/advanced/generics", "typescript/advanced"},
		{"go-v1/hello", "go"},
		{"single", "general"},
		{"", "general"},
	}

	for _, tt := range tests {
		t.Run(tt.exerciseID, func(t *testing.T) {
			got := ExtractTopic(tt.exerciseID)
			if got != tt.want {
				t.Errorf("ExtractTopic(%q) = %q; want %q", tt.exerciseID, got, tt.want)
			}
		})
	}
}

func TestExtractLanguage(t *testing.T) {
	tests := []struct {
		packID string
		want   string
	}{
		{"go-v1", "go"},
		{"python-fundamentals", "python"},
		{"typescript-v1", "typescript"},
		{"rust", "rust"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.packID, func(t *testing.T) {
			got := extractLanguage(tt.packID)
			if got != tt.want {
				t.Errorf("extractLanguage(%q) = %q; want %q", tt.packID, got, tt.want)
			}
		})
	}
}

func TestExtractCategory(t *testing.T) {
	tests := []struct {
		exerciseID string
		want       string
	}{
		{"go-v1/basics/hello-world", "basics"},
		{"go-v1/advanced/concurrency", "advanced"},
		{"pack/slug", "pack"},
		{"single", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.exerciseID, func(t *testing.T) {
			got := ExtractCategory(tt.exerciseID)
			if got != tt.want {
				t.Errorf("ExtractCategory(%q) = %q; want %q", tt.exerciseID, got, tt.want)
			}
		})
	}
}

func TestExtractPack(t *testing.T) {
	tests := []struct {
		exerciseID string
		want       string
	}{
		{"go-v1/basics/hello-world", "go-v1"},
		{"python-v1/testing/pytest", "python-v1"},
		{"single", "single"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.exerciseID, func(t *testing.T) {
			got := ExtractPack(tt.exerciseID)
			if got != tt.want {
				t.Errorf("ExtractPack(%q) = %q; want %q", tt.exerciseID, got, tt.want)
			}
		})
	}
}

func TestExtractSlug(t *testing.T) {
	tests := []struct {
		exerciseID string
		want       string
	}{
		{"go-v1/basics/hello-world", "hello-world"},
		{"pack/slug", "slug"},
		{"single", "single"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.exerciseID, func(t *testing.T) {
			got := ExtractSlug(tt.exerciseID)
			if got != tt.want {
				t.Errorf("ExtractSlug(%q) = %q; want %q", tt.exerciseID, got, tt.want)
			}
		})
	}
}
