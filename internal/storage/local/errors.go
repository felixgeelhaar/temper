package local

import "errors"

var (
	// ErrNotFound is returned when a record is not found
	ErrNotFound = errors.New("not found")
)
