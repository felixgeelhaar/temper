package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/felixgeelhaar/temper/internal/config"
)

// daemonAuthToken caches the loaded token to avoid re-reading secrets.yaml
// on every request inside a single CLI invocation.
var daemonAuthToken string

func daemonToken() string {
	if daemonAuthToken != "" {
		return daemonAuthToken
	}
	cfg, err := config.LoadLocalConfig()
	if err != nil {
		return ""
	}
	daemonAuthToken = cfg.Daemon.AuthToken
	return daemonAuthToken
}

// daemonGet issues an authenticated GET request to the daemon.
func daemonGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if t := daemonToken(); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	return http.DefaultClient.Do(req)
}

// daemonPost issues an authenticated POST request to the daemon with the
// given content type and body.
func daemonPost(url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if t := daemonToken(); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	return http.DefaultClient.Do(req)
}

// authError returns true if the response indicates the bearer token is
// missing or wrong, with a CLI-friendly hint.
func authError(resp *http.Response) error {
	if resp.StatusCode != http.StatusUnauthorized {
		return nil
	}
	return fmt.Errorf("daemon rejected the request as unauthorized. Run `temper init` to generate a token")
}
