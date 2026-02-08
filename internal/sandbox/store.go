package sandbox

// Store defines the persistence interface for sandboxes.
type Store interface {
	Save(sandbox *Sandbox) error
	Get(id string) (*Sandbox, error)
	GetBySession(sessionID string) (*Sandbox, error)
	Delete(id string) error
	ListActive() ([]*Sandbox, error)
	ListExpired() ([]*Sandbox, error)
}
