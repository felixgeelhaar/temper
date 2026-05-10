package llm

import (
	"net"
	"net/http"
	"time"
)

// llmTransport returns a tuned http.Transport shared by both the
// blocking and streaming clients. Centralizing the transport lets
// connections be reused across both call paths.
func llmTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   5,
		MaxConnsPerHost:       10,
		ForceAttemptHTTP2:     true,
	}
}

// newLLMHTTPClient creates an HTTP client for non-streaming LLM calls.
// Carries a 120s overall timeout so long-tail latencies cannot park
// goroutines or leak FDs indefinitely.
func newLLMHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   120 * time.Second,
		Transport: llmTransport(),
	}
}

// newLLMStreamHTTPClient creates an HTTP client for streaming LLM calls.
// Has NO Client.Timeout because that would kill the connection
// mid-stream during long generations. Cancellation is delegated to the
// caller's context (each request derives a deadline if appropriate).
// The transport's ResponseHeaderTimeout still bounds time-to-first-byte.
func newLLMStreamHTTPClient() *http.Client {
	return &http.Client{
		Transport: llmTransport(),
	}
}
