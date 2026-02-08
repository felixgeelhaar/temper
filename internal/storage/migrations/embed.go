package migrations

import "embed"

// FS embeds all SQL migration files for the SQLite storage layer.
//
//go:embed *.sql
var FS embed.FS
