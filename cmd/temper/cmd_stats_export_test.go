package main

import (
	"strings"
	"testing"
)

func TestAnonymize_DeterministicPerSalt(t *testing.T) {
	a1 := anonymize("session-abc", "cohort-1")
	a2 := anonymize("session-abc", "cohort-1")
	if a1 != a2 {
		t.Errorf("same (id, salt) should be deterministic: %s vs %s", a1, a2)
	}

	other := anonymize("session-abc", "cohort-2")
	if a1 == other {
		t.Errorf("different salt should produce different hash; both = %s", a1)
	}
}

func TestAnonymize_LengthAndShape(t *testing.T) {
	got := anonymize("uuid-1234-5678", "salt")
	if len(got) != 12 {
		t.Errorf("len(hash) = %d, want 12", len(got))
	}
	for _, ch := range got {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			t.Errorf("hash must be lowercase hex, got %q", got)
			break
		}
	}
}

func TestAnonymize_EmptyInput(t *testing.T) {
	if got := anonymize("", "salt"); got != "" {
		t.Errorf("empty id should return empty, got %q", got)
	}
}

func TestTopicFromExerciseID(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"go-v1/basics/hello-world", "go/basics"},
		{"go-v1/concurrency/channels", "go/concurrency"},
		{"python-v1/testing/pytest", "python/testing"},
		{"typescript-v1/advanced/promises", "typescript/advanced"},
		{"rust-v1/advanced", "rust"},
		{"single-segment", "general"},
		{"", "general"},
	}
	for _, c := range cases {
		if got := topicFromExerciseID(c.in); got != c.want {
			t.Errorf("topicFromExerciseID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTopicFromExerciseID_NoLanguageSuffix(t *testing.T) {
	if got := topicFromExerciseID("foo/bar/baz"); !strings.HasPrefix(got, "foo") {
		t.Errorf("should default pack name when no -version suffix, got %q", got)
	}
}
