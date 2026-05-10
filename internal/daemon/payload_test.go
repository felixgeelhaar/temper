package daemon

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateCodePayload_Empty(t *testing.T) {
	if err := validateCodePayload(nil); err != nil {
		t.Errorf("nil → got %v, want nil", err)
	}
	if err := validateCodePayload(map[string]string{}); err != nil {
		t.Errorf("empty → got %v, want nil", err)
	}
}

func TestValidateCodePayload_Allowed(t *testing.T) {
	code := map[string]string{
		"main.go":      strings.Repeat("a", 1024),
		"main_test.go": strings.Repeat("b", 1024),
	}
	if err := validateCodePayload(code); err != nil {
		t.Errorf("small payload should pass, got %v", err)
	}
}

func TestValidateCodePayload_TooManyFiles(t *testing.T) {
	code := make(map[string]string, MaxCodeFiles+1)
	for i := 0; i <= MaxCodeFiles; i++ {
		code[strings.Repeat("x", i+1)] = "small"
	}
	err := validateCodePayload(code)
	pe := asPayloadError(err)
	if pe == nil {
		t.Fatalf("expected PayloadError, got %v", err)
	}
	if pe.Code != "TOO_MANY_FILES" {
		t.Errorf("Code = %q, want TOO_MANY_FILES", pe.Code)
	}
}

func TestValidateCodePayload_FileTooLarge(t *testing.T) {
	code := map[string]string{
		"big.go": strings.Repeat("x", MaxCodeFileBytes+1),
	}
	err := validateCodePayload(code)
	pe := asPayloadError(err)
	if pe == nil {
		t.Fatalf("expected PayloadError, got %v", err)
	}
	if pe.Code != "FILE_TOO_LARGE" {
		t.Errorf("Code = %q, want FILE_TOO_LARGE", pe.Code)
	}
}

func TestValidateCodePayload_TotalTooLarge(t *testing.T) {
	// 5 files × 250 KiB = 1.25 MiB > 1 MiB total cap.
	code := make(map[string]string)
	for i := 0; i < 5; i++ {
		name := "f" + strings.Repeat("x", i) + ".go"
		code[name] = strings.Repeat("y", 250*1024)
	}
	err := validateCodePayload(code)
	pe := asPayloadError(err)
	if pe == nil {
		t.Fatalf("expected PayloadError, got %v", err)
	}
	if pe.Code != "PAYLOAD_TOO_LARGE" {
		t.Errorf("Code = %q, want PAYLOAD_TOO_LARGE", pe.Code)
	}
}

func TestAsPayloadError_NotPayloadError(t *testing.T) {
	if pe := asPayloadError(errors.New("some other error")); pe != nil {
		t.Errorf("got %v, want nil for non-PayloadError", pe)
	}
}
