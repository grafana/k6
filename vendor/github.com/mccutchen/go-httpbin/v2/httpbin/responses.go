package httpbin

import (
	"net/http"
	"net/url"
)

const (
	binaryContentType = "application/octet-stream"
	htmlContentType   = "text/html; charset=utf-8"
	jsonContentType   = "application/json; charset=utf-8"
	sseContentType    = "text/event-stream; charset=utf-8"
	textContentType   = "text/plain; charset=utf-8"
)

type envResponse struct {
	Env map[string]string `json:"env"`
}

type headersResponse struct {
	Headers http.Header `json:"headers"`
}

type ipResponse struct {
	Origin string `json:"origin"`
}

type userAgentResponse struct {
	UserAgent string `json:"user-agent"`
}

// A generic response for any incoming request that should not contain a body
// (GET, HEAD, OPTIONS, etc).
type noBodyResponse struct {
	Args    url.Values  `json:"args"`
	Headers http.Header `json:"headers"`
	Method  string      `json:"method"`
	Origin  string      `json:"origin"`
	URL     string      `json:"url"`

	Deflated bool `json:"deflated,omitempty"`
	Gzipped  bool `json:"gzipped,omitempty"`
}

// A response for incoming request where body data is discarded, like `/upload`
// (POST, PUT, PATCH).
type discardedBodyResponse struct {
	noBodyResponse
	BytesReceived int64 `json:"bytes_received"`
}

// A generic response for any incoming request that might contain a body (POST,
// PUT, PATCH, etc).
type bodyResponse struct {
	Args    url.Values  `json:"args"`
	Headers http.Header `json:"headers"`
	Method  string      `json:"method"`
	Origin  string      `json:"origin"`
	URL     string      `json:"url"`

	Data  string     `json:"data"`
	Files url.Values `json:"files"`
	Form  url.Values `json:"form"`
	JSON  any        `json:"json"`
}

type cookiesResponse struct {
	Cookies map[string]string `json:"cookies"`
}

type authResponse struct {
	Authenticated bool   `json:"authenticated"`
	User          string `json:"user"`

	// kept for backwards-compatibility with go-httpbin versions <= 2.20
	Authorized bool `json:"authorized"`
}

// An actual stream response body will be made up of one or more of these
// structs, encoded as JSON and separated by newlines
type streamResponse struct {
	ID      int         `json:"id"`
	Args    url.Values  `json:"args"`
	Headers http.Header `json:"headers"`
	Origin  string      `json:"origin"`
	URL     string      `json:"url"`
}

type uuidResponse struct {
	UUID string `json:"uuid"`
}

type bearerResponse struct {
	Authenticated bool   `json:"authenticated"`
	Token         string `json:"token"`
}

type hostnameResponse struct {
	Hostname string `json:"hostname"`
}

type errorRespnose struct {
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
	Detail     string `json:"detail,omitempty"`
}

type serverSentEvent struct {
	ID        int   `json:"id"`
	Timestamp int64 `json:"timestamp"`
}
