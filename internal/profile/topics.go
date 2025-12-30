package profile

import (
	"strings"
)

// ExtractTopic derives a topic from an exercise ID
// Exercise ID format: "pack/category/slug" or "pack/slug"
// Examples:
//   - "go-v1/basics/hello-world" -> "go/basics"
//   - "go-v1/interfaces/stringer" -> "go/interfaces"
//   - "python-v1/testing/pytest-basics" -> "python/testing"
func ExtractTopic(exerciseID string) string {
	parts := strings.Split(exerciseID, "/")
	if len(parts) < 2 {
		return "general"
	}

	// Extract language from pack (e.g., "go-v1" -> "go")
	language := extractLanguage(parts[0])

	// Use category if available, otherwise use pack
	if len(parts) >= 3 {
		return language + "/" + parts[1]
	}

	return language
}

// extractLanguage extracts the language from a pack ID
// Examples:
//   - "go-v1" -> "go"
//   - "python-fundamentals" -> "python"
//   - "typescript-v1" -> "typescript"
func extractLanguage(packID string) string {
	// Split on hyphen and take first part
	if idx := strings.Index(packID, "-"); idx > 0 {
		return packID[:idx]
	}
	return packID
}

// ExtractCategory extracts just the category from an exercise ID
// Examples:
//   - "go-v1/basics/hello-world" -> "basics"
//   - "go-v1/advanced/concurrency" -> "advanced"
func ExtractCategory(exerciseID string) string {
	parts := strings.Split(exerciseID, "/")
	if len(parts) >= 3 {
		return parts[1]
	}
	if len(parts) >= 2 {
		return parts[0]
	}
	return "unknown"
}

// ExtractPack extracts the pack ID from an exercise ID
// Examples:
//   - "go-v1/basics/hello-world" -> "go-v1"
func ExtractPack(exerciseID string) string {
	parts := strings.Split(exerciseID, "/")
	if len(parts) >= 1 {
		return parts[0]
	}
	return "unknown"
}

// ExtractSlug extracts just the exercise slug from an exercise ID
// Examples:
//   - "go-v1/basics/hello-world" -> "hello-world"
func ExtractSlug(exerciseID string) string {
	parts := strings.Split(exerciseID, "/")
	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}
	return exerciseID
}
