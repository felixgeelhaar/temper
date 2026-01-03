package llm

import (
	"net"
	"net/http"
	"time"
)

// newLLMHTTPClient creates an HTTP client optimized for LLM API calls
// with proper timeouts for long-running streaming responses
func newLLMHTTPClient() *http.Client {
	transport := &http.Transport{
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

	return &http.Client{
		Timeout:   120 * time.Second, // Long timeout for LLM responses
		Transport: transport,
	}
}
