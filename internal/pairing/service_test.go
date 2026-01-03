package pairing

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/llm"
)

func TestNewService(t *testing.T) {
	registry := llm.NewRegistry()
	service := NewService(registry, "test-provider")
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_ExtractTargets(t *testing.T) {
	registry := llm.NewRegistry()
	service := NewService(registry, "test-provider")

	tests := []struct {
		name    string
		ctx     InterventionContext
		wantLen int
	}{
		{
			name:    "no current file",
			ctx:     InterventionContext{},
			wantLen: 0,
		},
		{
			name: "with current file",
			ctx: InterventionContext{
				CurrentFile: "main.go",
				CursorLine:  10,
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.extractTargets(tt.ctx)
			if len(got) != tt.wantLen {
				t.Errorf("extractTargets() returned %d targets; want %d", len(got), tt.wantLen)
			}

			if tt.wantLen > 0 {
				if got[0].File != tt.ctx.CurrentFile {
					t.Errorf("Target.File = %q; want %q", got[0].File, tt.ctx.CurrentFile)
				}
				if got[0].StartLine != tt.ctx.CursorLine {
					t.Errorf("Target.StartLine = %d; want %d", got[0].StartLine, tt.ctx.CursorLine)
				}
			}
		})
	}
}

func TestService_ExtractTargets_Details(t *testing.T) {
	registry := llm.NewRegistry()
	service := NewService(registry, "test-provider")

	ctx := InterventionContext{
		CurrentFile: "handler.go",
		CursorLine:  42,
	}

	targets := service.extractTargets(ctx)
	if len(targets) != 1 {
		t.Fatalf("extractTargets() returned %d targets; want 1", len(targets))
	}

	target := targets[0]
	if target.File != "handler.go" {
		t.Errorf("File = %q; want %q", target.File, "handler.go")
	}
	if target.StartLine != 42 {
		t.Errorf("StartLine = %d; want %d", target.StartLine, 42)
	}
	if target.EndLine != 42 {
		t.Errorf("EndLine = %d; want %d", target.EndLine, 42)
	}
}
