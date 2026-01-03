package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestUserID(t *testing.T) {
	t.Run("NewUserID", func(t *testing.T) {
		id := uuid.New()
		userID := NewUserID(id)
		if userID.UUID() != id {
			t.Errorf("UUID() = %v, want %v", userID.UUID(), id)
		}
	})

	t.Run("NewUserIDFromString valid", func(t *testing.T) {
		id := uuid.New()
		userID, err := NewUserIDFromString(id.String())
		if err != nil {
			t.Fatalf("NewUserIDFromString() error = %v", err)
		}
		if userID.UUID() != id {
			t.Errorf("UUID() = %v, want %v", userID.UUID(), id)
		}
	})

	t.Run("NewUserIDFromString invalid", func(t *testing.T) {
		_, err := NewUserIDFromString("invalid")
		if err == nil {
			t.Error("NewUserIDFromString() should error on invalid UUID")
		}
	})

	t.Run("GenerateUserID", func(t *testing.T) {
		id := GenerateUserID()
		if id.IsZero() {
			t.Error("GenerateUserID() should not return zero value")
		}
	})

	t.Run("IsZero", func(t *testing.T) {
		zeroID := NewUserID(uuid.Nil)
		if !zeroID.IsZero() {
			t.Error("IsZero() should return true for nil UUID")
		}

		nonZeroID := GenerateUserID()
		if nonZeroID.IsZero() {
			t.Error("IsZero() should return false for valid UUID")
		}
	})

	t.Run("Equal", func(t *testing.T) {
		id1 := GenerateUserID()
		id2 := NewUserID(id1.UUID())
		id3 := GenerateUserID()

		if !id1.Equal(id2) {
			t.Error("Equal() should return true for same UUID")
		}
		if id1.Equal(id3) {
			t.Error("Equal() should return false for different UUIDs")
		}
	})
}

func TestSessionID(t *testing.T) {
	t.Run("NewSessionID", func(t *testing.T) {
		id := uuid.New()
		sessionID := NewSessionID(id)
		if sessionID.UUID() != id {
			t.Errorf("UUID() = %v, want %v", sessionID.UUID(), id)
		}
	})

	t.Run("GenerateSessionID", func(t *testing.T) {
		id := GenerateSessionID()
		if id.IsZero() {
			t.Error("GenerateSessionID() should not return zero value")
		}
	})

	t.Run("String", func(t *testing.T) {
		id := uuid.New()
		sessionID := NewSessionID(id)
		if sessionID.String() != id.String() {
			t.Errorf("String() = %v, want %v", sessionID.String(), id.String())
		}
	})
}

func TestArtifactID(t *testing.T) {
	t.Run("NewArtifactID", func(t *testing.T) {
		id := uuid.New()
		artifactID := NewArtifactID(id)
		if artifactID.UUID() != id {
			t.Errorf("UUID() = %v, want %v", artifactID.UUID(), id)
		}
	})

	t.Run("GenerateArtifactID", func(t *testing.T) {
		id := GenerateArtifactID()
		if id.IsZero() {
			t.Error("GenerateArtifactID() should not return zero value")
		}
	})
}

func TestExerciseID(t *testing.T) {
	t.Run("valid formats", func(t *testing.T) {
		tests := []string{
			"go-v1/hello-world",
			"python-v2/variables",
			"typescript-v1/types_basics",
			"rust-v1/ownership-1",
		}
		for _, s := range tests {
			t.Run(s, func(t *testing.T) {
				id, err := NewExerciseID(s)
				if err != nil {
					t.Fatalf("NewExerciseID(%q) error = %v", s, err)
				}
				if id.String() != s {
					t.Errorf("String() = %q, want %q", id.String(), s)
				}
			})
		}
	})

	t.Run("invalid formats", func(t *testing.T) {
		tests := []struct {
			name  string
			input string
		}{
			{"empty", ""},
			{"no slash", "go-v1-hello"},
			{"double slash", "go-v1//hello"},
			{"spaces", "go v1/hello"},
			{"special chars", "go-v1/hello@world"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := NewExerciseID(tt.input)
				if err == nil {
					t.Errorf("NewExerciseID(%q) should error", tt.input)
				}
			})
		}
	})

	t.Run("Pack and Slug", func(t *testing.T) {
		id, _ := NewExerciseID("go-v1/hello-world")
		if id.Pack() != "go-v1" {
			t.Errorf("Pack() = %q, want go-v1", id.Pack())
		}
		if id.Slug() != "hello-world" {
			t.Errorf("Slug() = %q, want hello-world", id.Slug())
		}
	})

	t.Run("MustExerciseID panics on invalid", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustExerciseID should panic on invalid input")
			}
		}()
		MustExerciseID("invalid")
	})

	t.Run("Equal", func(t *testing.T) {
		id1, _ := NewExerciseID("go-v1/hello")
		id2, _ := NewExerciseID("go-v1/hello")
		id3, _ := NewExerciseID("go-v1/world")

		if !id1.Equal(id2) {
			t.Error("Equal() should return true for same exercise ID")
		}
		if id1.Equal(id3) {
			t.Error("Equal() should return false for different exercise IDs")
		}
	})
}

func TestEmail(t *testing.T) {
	t.Run("valid emails", func(t *testing.T) {
		tests := []struct {
			input string
			want  string
		}{
			{"user@example.com", "user@example.com"},
			{"USER@EXAMPLE.COM", "user@example.com"},        // normalized to lowercase
			{"  user@example.com  ", "user@example.com"},    // trimmed
			{"user.name+tag@example.co.uk", "user.name+tag@example.co.uk"},
		}
		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				email, err := NewEmail(tt.input)
				if err != nil {
					t.Fatalf("NewEmail(%q) error = %v", tt.input, err)
				}
				if email.String() != tt.want {
					t.Errorf("String() = %q, want %q", email.String(), tt.want)
				}
			})
		}
	})

	t.Run("invalid emails", func(t *testing.T) {
		tests := []string{
			"",
			"invalid",
			"@example.com",
			"user@",
			"user@.com",
			"user@example",
		}
		for _, s := range tests {
			t.Run(s, func(t *testing.T) {
				_, err := NewEmail(s)
				if err == nil {
					t.Errorf("NewEmail(%q) should error", s)
				}
			})
		}
	})

	t.Run("Domain and LocalPart", func(t *testing.T) {
		email, _ := NewEmail("user@example.com")
		if email.Domain() != "example.com" {
			t.Errorf("Domain() = %q, want example.com", email.Domain())
		}
		if email.LocalPart() != "user" {
			t.Errorf("LocalPart() = %q, want user", email.LocalPart())
		}
	})

	t.Run("Equal", func(t *testing.T) {
		e1, _ := NewEmail("user@example.com")
		e2, _ := NewEmail("USER@EXAMPLE.COM")
		e3, _ := NewEmail("other@example.com")

		if !e1.Equal(e2) {
			t.Error("Equal() should return true for same email (case insensitive)")
		}
		if e1.Equal(e3) {
			t.Error("Equal() should return false for different emails")
		}
	})
}

// Note: SkillLevel tests are in profile_test.go as that's where SkillLevel is defined
