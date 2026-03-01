package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestUserID_StringAndEqual(t *testing.T) {
	id := uuid.New()
	userID := NewUserID(id)

	if userID.String() != id.String() {
		t.Errorf("UserID.String() = %s; want %s", userID.String(), id.String())
	}
	if userID.IsZero() {
		t.Error("UserID.IsZero() should be false")
	}
	if !userID.Equal(NewUserID(id)) {
		t.Error("UserID.Equal() should return true for same UUID")
	}

	parsed, err := NewUserIDFromString(id.String())
	if err != nil {
		t.Fatalf("NewUserIDFromString() error = %v", err)
	}
	if !parsed.Equal(userID) {
		t.Error("NewUserIDFromString() should return matching ID")
	}
}

func TestSessionID_FromString(t *testing.T) {
	_, err := NewSessionIDFromString("not-a-uuid")
	if err == nil {
		t.Error("NewSessionIDFromString() should return error for invalid input")
	}

	id := uuid.New()
	sessionID, err := NewSessionIDFromString(id.String())
	if err != nil {
		t.Fatalf("NewSessionIDFromString() error = %v", err)
	}
	if sessionID.String() != id.String() {
		t.Errorf("SessionID.String() = %s; want %s", sessionID.String(), id.String())
	}
	if !sessionID.Equal(NewSessionID(id)) {
		t.Error("SessionID.Equal() should return true for same UUID")
	}
}

func TestArtifactID_FromString(t *testing.T) {
	id := uuid.New()
	artifactID, err := NewArtifactIDFromString(id.String())
	if err != nil {
		t.Fatalf("NewArtifactIDFromString() error = %v", err)
	}
	if artifactID.String() != id.String() {
		t.Errorf("ArtifactID.String() = %s; want %s", artifactID.String(), id.String())
	}
	if !artifactID.Equal(NewArtifactID(id)) {
		t.Error("ArtifactID.Equal() should return true for same UUID")
	}
}

func TestExerciseID_ZeroAndParts(t *testing.T) {
	var empty ExerciseID
	if !empty.IsZero() {
		t.Error("ExerciseID.IsZero() should return true for empty value")
	}

	id, err := NewExerciseID("go-v1/hello-world")
	if err != nil {
		t.Fatalf("NewExerciseID() error = %v", err)
	}
	if id.Pack() != "go-v1" || id.Slug() != "hello-world" {
		t.Errorf("ExerciseID parts = (%s, %s); want (go-v1, hello-world)", id.Pack(), id.Slug())
	}
	if !id.Equal(MustExerciseID("go-v1/hello-world")) {
		t.Error("ExerciseID.Equal() should return true for same value")
	}
}

func TestEmail_IsZeroAndEqual(t *testing.T) {
	var email Email
	if !email.IsZero() {
		t.Error("Email.IsZero() should return true for empty value")
	}

	created, err := NewEmail("User@Example.com")
	if err != nil {
		t.Fatalf("NewEmail() error = %v", err)
	}
	if created.String() != "user@example.com" {
		t.Errorf("Email.String() = %s; want user@example.com", created.String())
	}
	if !created.Equal(Email{value: "user@example.com"}) {
		t.Error("Email.Equal() should return true for same value")
	}
}
