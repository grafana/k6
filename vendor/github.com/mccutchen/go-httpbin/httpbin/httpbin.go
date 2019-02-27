package httpbin

import (
	"net/http"
	"net/url"
	"time"
)

// Default configuration values
const (
	DefaultMaxBodySize int64 = 1024 * 1024
	DefaultMaxDuration       = 10 * time.Second
)

const jsonContentType = "application/json; encoding=utf-8"
const htmlContentType = "text/html; charset=utf-8"

type headersResponse struct {
	Headers http.Header `json:"headers"`
}

type ipResponse struct {
	Origin string `json:"origin"`
}

type userAgentResponse struct {
	UserAgent string `json:"user-agent"`
}

type getResponse struct {
	Args    url.Values  `json:"args"`
	Headers http.Header `json:"headers"`
	Origin  string      `json:"origin"`
	URL     string      `json:"url"`
}

// A generic response for any incoming request that might contain a body
type bodyResponse struct {
	Args    url.Values  `json:"args"`
	Headers http.Header `json:"headers"`
	Origin  string      `json:"origin"`
	URL     string      `json:"url"`

	Data  string              `json:"data"`
	Files map[string][]string `json:"files"`
	Form  map[string][]string `json:"form"`
	JSON  interface{}         `json:"json"`
}

type cookiesResponse map[string]string

type authResponse struct {
	Authorized bool   `json:"authorized"`
	User       string `json:"user"`
}

type gzipResponse struct {
	Headers http.Header `json:"headers"`
	Origin  string      `json:"origin"`
	Gzipped bool        `json:"gzipped"`
}

type deflateResponse struct {
	Headers  http.Header `json:"headers"`
	Origin   string      `json:"origin"`
	Deflated bool        `json:"deflated"`
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

// HTTPBin contains the business logic
type HTTPBin struct {
	// Max size of an incoming request generated response body, in bytes
	MaxBodySize int64

	// Max duration of a request, for those requests that allow user control
	// over timing (e.g. /delay)
	MaxDuration time.Duration

	// Observer called with the result of each handled request
	Observer Observer
}

// Handler returns an http.Handler that exposes all HTTPBin endpoints
func (h *HTTPBin) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", methods(h.Index, "GET"))
	mux.HandleFunc("/forms/post", methods(h.FormsPost, "GET"))
	mux.HandleFunc("/encoding/utf8", methods(h.UTF8, "GET"))

	mux.HandleFunc("/get", methods(h.Get, "GET"))
	mux.HandleFunc("/post", methods(h.RequestWithBody, "POST"))
	mux.HandleFunc("/put", methods(h.RequestWithBody, "PUT"))
	mux.HandleFunc("/patch", methods(h.RequestWithBody, "PATCH"))
	mux.HandleFunc("/delete", methods(h.RequestWithBody, "DELETE"))

	mux.HandleFunc("/ip", h.IP)
	mux.HandleFunc("/user-agent", h.UserAgent)
	mux.HandleFunc("/headers", h.Headers)
	mux.HandleFunc("/response-headers", h.ResponseHeaders)

	mux.HandleFunc("/status/", h.Status)

	mux.HandleFunc("/redirect/", h.Redirect)
	mux.HandleFunc("/relative-redirect/", h.RelativeRedirect)
	mux.HandleFunc("/absolute-redirect/", h.AbsoluteRedirect)
	mux.HandleFunc("/redirect-to", h.RedirectTo)

	mux.HandleFunc("/cookies", h.Cookies)
	mux.HandleFunc("/cookies/set", h.SetCookies)
	mux.HandleFunc("/cookies/delete", h.DeleteCookies)

	mux.HandleFunc("/basic-auth/", h.BasicAuth)
	mux.HandleFunc("/hidden-basic-auth/", h.HiddenBasicAuth)
	mux.HandleFunc("/digest-auth/", h.DigestAuth)

	mux.HandleFunc("/deflate", h.Deflate)
	mux.HandleFunc("/gzip", h.Gzip)

	mux.HandleFunc("/stream/", h.Stream)
	mux.HandleFunc("/delay/", h.Delay)
	mux.HandleFunc("/drip", h.Drip)

	mux.HandleFunc("/range/", h.Range)
	mux.HandleFunc("/bytes/", h.Bytes)
	mux.HandleFunc("/stream-bytes/", h.StreamBytes)

	mux.HandleFunc("/html", h.HTML)
	mux.HandleFunc("/robots.txt", h.Robots)
	mux.HandleFunc("/deny", h.Deny)

	mux.HandleFunc("/cache", h.Cache)
	mux.HandleFunc("/cache/", h.CacheControl)
	mux.HandleFunc("/etag/", h.ETag)

	mux.HandleFunc("/links/", h.Links)

	mux.HandleFunc("/image", h.ImageAccept)
	mux.HandleFunc("/image/", h.Image)
	mux.HandleFunc("/xml", h.XML)

	// existing httpbin endpoints that we do not support
	mux.HandleFunc("/brotli", notImplementedHandler)

	// Make sure our ServeMux doesn't "helpfully" redirect these invalid
	// endpoints by adding a trailing slash. See the ServeMux docs for more
	// info: https://golang.org/pkg/net/http/#ServeMux
	mux.HandleFunc("/absolute-redirect", http.NotFound)
	mux.HandleFunc("/basic-auth", http.NotFound)
	mux.HandleFunc("/delay", http.NotFound)
	mux.HandleFunc("/digest-auth", http.NotFound)
	mux.HandleFunc("/hidden-basic-auth", http.NotFound)
	mux.HandleFunc("/redirect", http.NotFound)
	mux.HandleFunc("/relative-redirect", http.NotFound)
	mux.HandleFunc("/status", http.NotFound)
	mux.HandleFunc("/stream", http.NotFound)
	mux.HandleFunc("/bytes", http.NotFound)
	mux.HandleFunc("/stream-bytes", http.NotFound)
	mux.HandleFunc("/links", http.NotFound)

	// Apply global middleware
	var handler http.Handler
	handler = mux
	handler = limitRequestSize(h.MaxBodySize, handler)
	handler = preflight(handler)
	handler = autohead(handler)
	if h.Observer != nil {
		handler = observe(h.Observer, handler)
	}

	return handler
}

// New creates a new HTTPBin instance
func New(opts ...OptionFunc) *HTTPBin {
	h := &HTTPBin{
		MaxBodySize: DefaultMaxBodySize,
		MaxDuration: DefaultMaxDuration,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// OptionFunc uses the "functional options" pattern to customize an HTTPBin
// instance
type OptionFunc func(*HTTPBin)

// WithMaxBodySize sets the maximum amount of memory
func WithMaxBodySize(m int64) OptionFunc {
	return func(h *HTTPBin) {
		h.MaxBodySize = m
	}
}

// WithMaxDuration sets the maximum amount of time httpbin may take to respond
func WithMaxDuration(d time.Duration) OptionFunc {
	return func(h *HTTPBin) {
		h.MaxDuration = d
	}
}

// WithObserver sets the request observer callback
func WithObserver(o Observer) OptionFunc {
	return func(h *HTTPBin) {
		h.Observer = o
	}
}
