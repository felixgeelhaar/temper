package daemon

import (
	"bytes"
	"net/http"
)

// capturingResponseWriter buffers response bytes + status alongside the
// real ResponseWriter so a successful response can be replayed under
// the same Idempotency-Key.
type capturingResponseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	wroteCode  bool
}

func newCapturingResponseWriter(w http.ResponseWriter) *capturingResponseWriter {
	return &capturingResponseWriter{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK,
	}
}

func (c *capturingResponseWriter) WriteHeader(code int) {
	if c.wroteCode {
		return
	}
	c.statusCode = code
	c.wroteCode = true
	c.ResponseWriter.WriteHeader(code)
}

func (c *capturingResponseWriter) Write(b []byte) (int, error) {
	if !c.wroteCode {
		c.WriteHeader(http.StatusOK)
	}
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}
