package auth

import (
	"net/http"
)

// Provider creates an HTTP client with authentication configured.
type Provider interface {
	// HTTPClient returns an *http.Client that handles auth transparently.
	// For Kerberos, this is a spnego.Client wrapper.
	// For NoAuth, this is a standard http.Client.
	HTTPClient(transport *http.Transport) HTTPDoer
	Validate() error
}

// HTTPDoer is the interface for making HTTP requests (both http.Client and spnego.Client implement this).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// NoAuth is a no-op provider for public Zuul instances.
type NoAuth struct{}

func (n *NoAuth) HTTPClient(transport *http.Transport) HTTPDoer {
	return &http.Client{Transport: transport}
}

func (n *NoAuth) Validate() error {
	return nil
}
