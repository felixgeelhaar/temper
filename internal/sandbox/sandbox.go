package sandbox

import (
	"errors"
	"time"
)

// Status represents the lifecycle state of a sandbox.
type Status string

const (
	StatusCreating  Status = "creating"
	StatusReady     Status = "ready"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusDestroyed Status = "destroyed"
)

// Sandbox represents a persistent Docker container bound to a session.
type Sandbox struct {
	ID          string     `json:"id"`
	SessionID   string     `json:"session_id"`
	ContainerID string     `json:"container_id"`
	Language    string     `json:"language"`
	Image       string     `json:"image"`
	Status      Status     `json:"status"`
	MemoryMB    int        `json:"memory_mb"`
	CPULimit    float64    `json:"cpu_limit"`
	NetworkOff  bool       `json:"network_off"`
	LastExecAt  *time.Time `json:"last_exec_at,omitempty"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// IsExpired returns true if the sandbox has expired.
func (s *Sandbox) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsActive returns true if the sandbox can accept work.
func (s *Sandbox) IsActive() bool {
	return s.Status == StatusReady || s.Status == StatusRunning
}

// ExecResult holds the output from a sandbox execution.
type ExecResult struct {
	ExitCode int           `json:"exit_code"`
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	Duration time.Duration `json:"duration"`
}

// Config holds sandbox creation parameters.
type Config struct {
	Language   string        `json:"language"`
	Image      string        `json:"image"`
	MemoryMB   int           `json:"memory_mb"`
	CPULimit   float64       `json:"cpu_limit"`
	NetworkOff bool          `json:"network_off"`
	IdleTTL    time.Duration `json:"idle_ttl"` // How long to keep idle sandbox alive
}

// DefaultConfig returns sensible defaults for a Go sandbox.
func DefaultConfig() Config {
	return Config{
		Language:   "go",
		Image:      "golang:1.23-alpine",
		MemoryMB:   256,
		CPULimit:   0.5,
		NetworkOff: true,
		IdleTTL:    30 * time.Minute,
	}
}

var (
	ErrSandboxNotFound   = errors.New("sandbox not found")
	ErrSandboxExpired    = errors.New("sandbox has expired")
	ErrSandboxNotReady   = errors.New("sandbox is not ready")
	ErrMaxSandboxes      = errors.New("maximum concurrent sandboxes reached")
	ErrSessionHasSandbox = errors.New("session already has a sandbox")
)
