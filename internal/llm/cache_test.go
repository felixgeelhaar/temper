package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClaudeBuildRequest_LegacySystemString(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{APIKey: "k"})
	req := &Request{
		System: "You are a tutor.",
		Messages: []Message{
			{Role: RoleUser, Content: "hi"},
		},
	}

	cr := p.buildRequest(req, false)
	got, ok := cr.System.(string)
	if !ok {
		t.Fatalf("legacy System should serialize as string, got %T", cr.System)
	}
	if got != "You are a tutor." {
		t.Errorf("System = %q", got)
	}
}

func TestClaudeBuildRequest_SystemBlocks_EmitCacheControl(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{APIKey: "k"})
	req := &Request{
		SystemBlocks: []SystemContentBlock{
			{Text: "stable instructions", CacheControl: true},
			{Text: "dynamic addendum", CacheControl: false},
		},
		Messages: []Message{
			{Role: RoleUser, Content: "hi"},
		},
	}

	cr := p.buildRequest(req, false)
	blocks, ok := cr.System.([]claudeSystemBlock)
	if !ok {
		t.Fatalf("SystemBlocks should serialize as []claudeSystemBlock, got %T", cr.System)
	}
	if len(blocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2", len(blocks))
	}
	if blocks[0].CacheControl == nil || blocks[0].CacheControl.Type != "ephemeral" {
		t.Errorf("first block must have CacheControl.Type=ephemeral, got %+v", blocks[0].CacheControl)
	}
	if blocks[1].CacheControl != nil {
		t.Errorf("second block must not have CacheControl, got %+v", blocks[1].CacheControl)
	}

	// Marshal end-to-end and verify the wire shape Anthropic expects.
	body, err := json.Marshal(cr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wire := string(body)
	if !strings.Contains(wire, `"cache_control":{"type":"ephemeral"}`) {
		t.Errorf("wire format missing cache_control: %s", wire)
	}
	if !strings.Contains(wire, `"type":"text"`) {
		t.Errorf("wire format missing block type: %s", wire)
	}
}

func TestClaudeBuildRequest_SystemBlocksTakePrecedenceOverLegacy(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{APIKey: "k"})
	req := &Request{
		System: "ignored",
		SystemBlocks: []SystemContentBlock{
			{Text: "from blocks", CacheControl: true},
		},
	}

	cr := p.buildRequest(req, false)
	if _, ok := cr.System.(string); ok {
		t.Errorf("SystemBlocks present: System should be []claudeSystemBlock, got string")
	}
}

func TestFlattenSystemBlocks_FallbackToLegacy(t *testing.T) {
	got := flattenSystemBlocks(&Request{System: "legacy"})
	if got != "legacy" {
		t.Errorf("got %q, want legacy", got)
	}
}

func TestFlattenSystemBlocks_JoinsBlocks(t *testing.T) {
	got := flattenSystemBlocks(&Request{
		SystemBlocks: []SystemContentBlock{
			{Text: "first"},
			{Text: "second", CacheControl: true},
			{Text: ""}, // empty block dropped
			{Text: "third"},
		},
	})
	want := "first\n\nsecond\n\nthird"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNewClaudeProvider_DefaultModelIsCurrent(t *testing.T) {
	p := NewClaudeProvider(ClaudeConfig{APIKey: "k"})
	if p.model != "claude-sonnet-4-6" {
		t.Errorf("default model = %q, want claude-sonnet-4-6", p.model)
	}
}
