// Package provtest provides an HTTP test server that simulates the
// provisioning API. Routes are registered by individual specs as they
// add API methods; this skeleton starts with an empty mux.
package provtest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Server is a test HTTP server for the provisioning API.
type Server struct {
	*httptest.Server
	Mux *http.ServeMux
}

// NewServer creates and starts a test server with an empty route table.
// The server is automatically closed when the test finishes.
func NewServer(t *testing.T) *Server {
	t.Helper()

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &Server{
		Server: srv,
		Mux:    mux,
	}
}
