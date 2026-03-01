package sandbox

import (
	"testing"
	"time"
)

func TestSandbox_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "expired sandbox",
			expiresAt: time.Now().Add(-1 * time.Hour),
			want:      true,
		},
		{
			name:      "not expired sandbox",
			expiresAt: time.Now().Add(1 * time.Hour),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sandbox{
				ExpiresAt: tt.expiresAt,
			}
			if got := s.IsExpired(); got != tt.want {
				t.Errorf("Sandbox.IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSandbox_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "ready is active",
			status: StatusReady,
			want:   true,
		},
		{
			name:   "running is active",
			status: StatusRunning,
			want:   true,
		},
		{
			name:   "creating is not active",
			status: StatusCreating,
			want:   false,
		},
		{
			name:   "paused is not active",
			status: StatusPaused,
			want:   false,
		},
		{
			name:   "destroyed is not active",
			status: StatusDestroyed,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sandbox{
				Status: tt.status,
			}
			if got := s.IsActive(); got != tt.want {
				t.Errorf("Sandbox.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}
