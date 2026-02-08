package sqlite

import (
	"github.com/felixgeelhaar/temper/internal/profile"
	"github.com/felixgeelhaar/temper/internal/session"
)

// Ensure SQLite stores implement the storage interfaces.
var (
	_ session.SessionStore = (*SessionStore)(nil)
	_ profile.ProfileStore = (*ProfileStore)(nil)
)
