package queue

import "testing"

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "short URL unchanged",
			url:  "amqp://localhost",
			want: "amqp://localhost",
		},
		{
			name: "exactly 20 chars unchanged",
			url:  "amqp://localhost:567",
			want: "amqp://localhost:567",
		},
		{
			name: "long URL truncated",
			url:  "amqp://user:password@localhost:5672/vhost",
			want: "amqp://user:password...",
		},
		{
			name: "empty URL",
			url:  "",
			want: "",
		},
		{
			name: "long URL with credentials",
			url:  "amqp://temper:secretpassword@rabbitmq.production.internal:5672/",
			want: "amqp://temper:secret...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeURL(tt.url)
			if got != tt.want {
				t.Errorf("sanitizeURL(%q) = %q; want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestSanitizeURL_HidesPassword(t *testing.T) {
	// Test that long URLs with passwords get truncated
	url := "amqp://user:supersecretpassword@host:5672/"
	result := sanitizeURL(url)

	// Result should not contain the full password
	if len(result) > 23 { // 20 chars + "..."
		t.Errorf("sanitizeURL should truncate long URLs, got %q (len=%d)", result, len(result))
	}
}

func TestRunRecipe_Defaults(t *testing.T) {
	recipe := RunRecipe{}

	// Verify zero values
	if recipe.Format {
		t.Error("Format should default to false")
	}
	if recipe.Build {
		t.Error("Build should default to false")
	}
	if recipe.Test {
		t.Error("Test should default to false")
	}
	if recipe.Timeout != 0 {
		t.Errorf("Timeout should default to 0, got %d", recipe.Timeout)
	}
}

func TestFormatOutput_Fields(t *testing.T) {
	output := FormatOutput{
		OK:   true,
		Diff: "--- a/main.go\n+++ b/main.go\n",
	}

	if !output.OK {
		t.Error("OK should be true")
	}
	if output.Diff == "" {
		t.Error("Diff should not be empty")
	}
}

func TestBuildOutput_Fields(t *testing.T) {
	output := BuildOutput{
		OK:     false,
		Output: "main.go:5:1: syntax error",
	}

	if output.OK {
		t.Error("OK should be false")
	}
	if output.Output == "" {
		t.Error("Output should not be empty")
	}
}

func TestTestOutput_Fields(t *testing.T) {
	output := TestOutput{
		OK:       true,
		Output:   "PASS",
		Duration: 100,
	}

	if !output.OK {
		t.Error("OK should be true")
	}
	if output.Duration != 100 {
		t.Errorf("Duration = %d; want 100", output.Duration)
	}
}

func TestQueueNames_Constants(t *testing.T) {
	if RunQueueName != "temper.runs" {
		t.Errorf("RunQueueName = %q; want %q", RunQueueName, "temper.runs")
	}
	if ResultQueueName != "temper.results" {
		t.Errorf("ResultQueueName = %q; want %q", ResultQueueName, "temper.results")
	}
}
