package daemon

import (
	"errors"
	"fmt"
)

// Payload caps for endpoints that accept user-supplied code maps. These
// bound memory pressure on the daemon and runner against malicious or
// buggy clients sending unbounded payloads.
const (
	MaxCodeFiles      = 50
	MaxCodeFileBytes  = 256 * 1024  // 256 KiB per file
	MaxCodeTotalBytes = 1024 * 1024 // 1 MiB total
	// HTTP body limit slightly larger than MaxCodeTotalBytes to cover JSON
	// overhead (quoting, escapes, structure). Body that exceeds this is
	// rejected before json decoding even begins.
	MaxRunBodyBytes = MaxCodeTotalBytes + 256*1024
)

// PayloadError describes a payload validation failure with a stable
// machine-readable code and a human-readable detail.
type PayloadError struct {
	Code    string // PAYLOAD_TOO_LARGE | TOO_MANY_FILES | FILE_TOO_LARGE
	Message string
}

func (e *PayloadError) Error() string {
	return e.Message
}

// validateCodePayload returns a *PayloadError when the code map exceeds any
// of the documented limits. Empty maps are valid.
func validateCodePayload(code map[string]string) error {
	if len(code) > MaxCodeFiles {
		return &PayloadError{
			Code:    "TOO_MANY_FILES",
			Message: fmt.Sprintf("payload contains %d files, limit is %d", len(code), MaxCodeFiles),
		}
	}
	total := 0
	for name, content := range code {
		if len(content) > MaxCodeFileBytes {
			return &PayloadError{
				Code: "FILE_TOO_LARGE",
				Message: fmt.Sprintf("file %q is %d bytes, per-file limit is %d",
					name, len(content), MaxCodeFileBytes),
			}
		}
		total += len(content)
		if total > MaxCodeTotalBytes {
			return &PayloadError{
				Code: "PAYLOAD_TOO_LARGE",
				Message: fmt.Sprintf("total payload %d bytes exceeds limit %d",
					total, MaxCodeTotalBytes),
			}
		}
	}
	return nil
}

// asPayloadError extracts a *PayloadError from err if present.
func asPayloadError(err error) *PayloadError {
	var pe *PayloadError
	if errors.As(err, &pe) {
		return pe
	}
	return nil
}
