package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxConcurrentSandboxes limits the number of active sandboxes.
	MaxConcurrentSandboxes = 10
	// DefaultIdleTTL is the default time before an idle sandbox expires.
	DefaultIdleTTL = 30 * time.Minute
)

// Manager orchestrates sandbox lifecycle: create, attach, execute, pause, resume, destroy.
type Manager struct {
	store   Store
	backend *DockerBackend
	mu      sync.Mutex
}

// NewManager creates a new sandbox manager.
func NewManager(store Store, backend *DockerBackend) *Manager {
	return &Manager{
		store:   store,
		backend: backend,
	}
}

// Create creates a new persistent sandbox for a session.
func (m *Manager) Create(ctx context.Context, sessionID string, cfg Config) (*Sandbox, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if session already has an active sandbox
	existing, err := m.store.GetBySession(sessionID)
	if err == nil && existing.IsActive() {
		return nil, ErrSessionHasSandbox
	}

	// Check concurrent sandbox limit
	active, err := m.store.ListActive()
	if err != nil {
		return nil, fmt.Errorf("list active sandboxes: %w", err)
	}
	if len(active) >= MaxConcurrentSandboxes {
		return nil, ErrMaxSandboxes
	}

	// Apply defaults
	if cfg.Image == "" {
		cfg = DefaultConfig()
	}
	if cfg.IdleTTL == 0 {
		cfg.IdleTTL = DefaultIdleTTL
	}

	now := time.Now()
	sb := &Sandbox{
		ID:         uuid.New().String(),
		SessionID:  sessionID,
		Language:   cfg.Language,
		Image:      cfg.Image,
		Status:     StatusCreating,
		MemoryMB:   cfg.MemoryMB,
		CPULimit:   cfg.CPULimit,
		NetworkOff: cfg.NetworkOff,
		ExpiresAt:  now.Add(cfg.IdleTTL),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Persist creating state
	if err := m.store.Save(sb); err != nil {
		return nil, fmt.Errorf("save sandbox: %w", err)
	}

	// Create Docker container
	containerID, err := m.backend.CreateContainer(ctx, cfg)
	if err != nil {
		sb.Status = StatusDestroyed
		m.store.Save(sb)
		return nil, fmt.Errorf("create container: %w", err)
	}

	sb.ContainerID = containerID
	sb.Status = StatusReady
	sb.UpdatedAt = time.Now()

	if err := m.store.Save(sb); err != nil {
		// Roll back container
		m.backend.DestroyContainer(ctx, containerID)
		return nil, fmt.Errorf("save sandbox: %w", err)
	}

	slog.Info("sandbox created",
		"sandbox_id", sb.ID,
		"session_id", sessionID,
		"container_id", containerID[:12],
		"language", cfg.Language,
	)

	return sb, nil
}

// Get retrieves a sandbox by ID.
func (m *Manager) Get(ctx context.Context, id string) (*Sandbox, error) {
	return m.store.Get(id)
}

// GetBySession retrieves the sandbox for a session.
func (m *Manager) GetBySession(ctx context.Context, sessionID string) (*Sandbox, error) {
	return m.store.GetBySession(sessionID)
}

// AttachCode copies files into a sandbox's workspace.
func (m *Manager) AttachCode(ctx context.Context, sandboxID string, code map[string]string) error {
	sb, err := m.store.Get(sandboxID)
	if err != nil {
		return ErrSandboxNotFound
	}

	if !sb.IsActive() {
		return ErrSandboxNotReady
	}
	if sb.IsExpired() {
		return ErrSandboxExpired
	}

	return m.backend.CopyFiles(ctx, sb.ContainerID, code)
}

// Execute runs a command inside the sandbox.
func (m *Manager) Execute(ctx context.Context, sandboxID string, cmd []string, timeout time.Duration) (*ExecResult, error) {
	sb, err := m.store.Get(sandboxID)
	if err != nil {
		return nil, ErrSandboxNotFound
	}

	if !sb.IsActive() {
		return nil, ErrSandboxNotReady
	}
	if sb.IsExpired() {
		return nil, ErrSandboxExpired
	}

	// Update status to running
	sb.Status = StatusRunning
	sb.UpdatedAt = time.Now()
	m.store.Save(sb)

	result, err := m.backend.Exec(ctx, sb.ContainerID, cmd, timeout)

	// Update status back to ready and extend expiry
	now := time.Now()
	sb.Status = StatusReady
	sb.LastExecAt = &now
	sb.ExpiresAt = now.Add(DefaultIdleTTL)
	sb.UpdatedAt = now
	m.store.Save(sb)

	return result, err
}

// Pause pauses a sandbox container (saves resources while idle).
func (m *Manager) Pause(ctx context.Context, sandboxID string) error {
	sb, err := m.store.Get(sandboxID)
	if err != nil {
		return ErrSandboxNotFound
	}

	if sb.Status != StatusReady {
		return ErrSandboxNotReady
	}

	if err := m.backend.PauseContainer(ctx, sb.ContainerID); err != nil {
		return fmt.Errorf("pause container: %w", err)
	}

	sb.Status = StatusPaused
	sb.UpdatedAt = time.Now()
	return m.store.Save(sb)
}

// Resume resumes a paused sandbox container.
func (m *Manager) Resume(ctx context.Context, sandboxID string) error {
	sb, err := m.store.Get(sandboxID)
	if err != nil {
		return ErrSandboxNotFound
	}

	if sb.Status != StatusPaused {
		return fmt.Errorf("sandbox is not paused (status: %s)", sb.Status)
	}

	if err := m.backend.ResumeContainer(ctx, sb.ContainerID); err != nil {
		return fmt.Errorf("resume container: %w", err)
	}

	sb.Status = StatusReady
	sb.ExpiresAt = time.Now().Add(DefaultIdleTTL) // Reset expiry on resume
	sb.UpdatedAt = time.Now()
	return m.store.Save(sb)
}

// Destroy stops and removes a sandbox container.
func (m *Manager) Destroy(ctx context.Context, sandboxID string) error {
	sb, err := m.store.Get(sandboxID)
	if err != nil {
		return ErrSandboxNotFound
	}

	if sb.ContainerID != "" {
		if err := m.backend.DestroyContainer(ctx, sb.ContainerID); err != nil {
			slog.Warn("failed to destroy container", "container_id", sb.ContainerID, "error", err)
		}
	}

	sb.Status = StatusDestroyed
	sb.UpdatedAt = time.Now()
	return m.store.Save(sb)
}

// Cleanup removes all expired sandboxes.
func (m *Manager) Cleanup(ctx context.Context) (int, error) {
	expired, err := m.store.ListExpired()
	if err != nil {
		return 0, fmt.Errorf("list expired: %w", err)
	}

	cleaned := 0
	for _, sb := range expired {
		if sb.ContainerID != "" {
			if err := m.backend.DestroyContainer(ctx, sb.ContainerID); err != nil {
				slog.Warn("cleanup: failed to destroy container",
					"sandbox_id", sb.ID,
					"container_id", sb.ContainerID,
					"error", err,
				)
			}
		}

		sb.Status = StatusDestroyed
		sb.UpdatedAt = time.Now()
		if err := m.store.Save(sb); err != nil {
			slog.Warn("cleanup: failed to update sandbox", "sandbox_id", sb.ID, "error", err)
			continue
		}
		cleaned++
	}

	if cleaned > 0 {
		slog.Info("sandbox cleanup complete", "cleaned", cleaned)
	}

	return cleaned, nil
}

// StartCleanupLoop starts a background goroutine that periodically cleans up expired sandboxes.
func (m *Manager) StartCleanupLoop(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := m.Cleanup(ctx); err != nil {
					slog.Warn("sandbox cleanup error", "error", err)
				}
			}
		}
	}()
}

// Close cleans up all active sandboxes and closes the Docker client.
func (m *Manager) Close(ctx context.Context) error {
	active, err := m.store.ListActive()
	if err != nil {
		slog.Warn("failed to list active sandboxes during shutdown", "error", err)
	} else {
		for _, sb := range active {
			if sb.ContainerID != "" {
				_ = m.backend.DestroyContainer(ctx, sb.ContainerID)
			}
			sb.Status = StatusDestroyed
			sb.UpdatedAt = time.Now()
			_ = m.store.Save(sb)
		}
	}

	return m.backend.Close()
}
