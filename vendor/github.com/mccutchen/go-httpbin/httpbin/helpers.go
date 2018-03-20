package httpbin

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// requestHeaders takes in incoming request and returns an http.Header map
// suitable for inclusion in our response data structures.
//
// This is necessary to ensure that the incoming Host header is included,
// because golang only exposes that header on the http.Request struct itself.
func getRequestHeaders(r *http.Request) http.Header {
	h := r.Header
	h.Set("Host", r.Host)
	return h
}

func getOrigin(r *http.Request) string {
	origin := r.Header.Get("X-Forwarded-For")
	if origin == "" {
		origin = r.RemoteAddr
	}
	return origin
}

func getURL(r *http.Request) *url.URL {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = r.Header.Get("X-Forwarded-Protocol")
	}
	if scheme == "" && r.Header.Get("X-Forwarded-Ssl") == "on" {
		scheme = "https"
	}
	if scheme == "" {
		scheme = "http"
	}

	host := r.URL.Host
	if host == "" {
		host = r.Host
	}

	return &url.URL{
		Scheme:     scheme,
		Opaque:     r.URL.Opaque,
		User:       r.URL.User,
		Host:       host,
		Path:       r.URL.Path,
		RawPath:    r.URL.RawPath,
		ForceQuery: r.URL.ForceQuery,
		RawQuery:   r.URL.RawQuery,
		Fragment:   r.URL.Fragment,
	}
}

func writeResponse(w http.ResponseWriter, status int, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.WriteHeader(status)
	w.Write(body)
}

func writeJSON(w http.ResponseWriter, body []byte, status int) {
	writeResponse(w, status, jsonContentType, body)
}

func writeHTML(w http.ResponseWriter, body []byte, status int) {
	writeResponse(w, status, htmlContentType, body)
}

// parseBody handles parsing a request body into our standard API response,
// taking care to only consume the request body once based on the Content-Type
// of the request. The given bodyResponse will be modified.
//
// Note: this function expects callers to limit the the maximum size of the
// request body. See, e.g., the limitRequestSize middleware.
func parseBody(w http.ResponseWriter, r *http.Request, resp *bodyResponse) error {
	if r.Body == nil {
		return nil
	}

	// Always set resp.Data to the incoming request body, in case we don't know
	// how to handle the content type
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		r.Body.Close()
		return err
	}
	resp.Data = string(body)

	// After reading the body to populate resp.Data, we need to re-wrap it in
	// an io.Reader for further processing below
	r.Body.Close()
	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "application/x-www-form-urlencoded"):
		if err := r.ParseForm(); err != nil {
			return err
		}
		resp.Form = r.PostForm
	case strings.HasPrefix(ct, "multipart/form-data"):
		// The memory limit here only restricts how many parts will be kept in
		// memory before overflowing to disk:
		// http://localhost:8080/pkg/net/http/#Request.ParseMultipartForm
		if err := r.ParseMultipartForm(1024); err != nil {
			return err
		}
		resp.Form = r.PostForm
	case strings.HasPrefix(ct, "application/json"):
		err := json.NewDecoder(r.Body).Decode(&resp.JSON)
		if err != nil && err != io.EOF {
			return err
		}
	}

	return nil
}

// parseDuration takes a user's input as a string and attempts to convert it
// into a time.Duration. If not given as a go-style duration string, the input
// is assumed to be seconds as a float.
func parseDuration(input string) (time.Duration, error) {
	d, err := time.ParseDuration(input)
	if err != nil {
		n, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return 0, err
		}
		d = time.Duration(n*1000) * time.Millisecond
	}
	return d, nil
}

// parseBoundedDuration parses a time.Duration from user input and ensures that
// it is within a given maximum and minimum time
func parseBoundedDuration(input string, min, max time.Duration) (time.Duration, error) {
	d, err := parseDuration(input)
	if err != nil {
		return 0, err
	}

	if d > max {
		err = fmt.Errorf("duration %s longer than %s", d, max)
	} else if d < min {
		err = fmt.Errorf("duration %s shorter than %s", d, min)
	}
	return d, err
}

// syntheticByteStream implements the ReadSeeker interface to allow reading
// arbitrary subsets of bytes up to a maximum size given a function for
// generating the byte at a given offset.
type syntheticByteStream struct {
	mu sync.Mutex

	size    int64
	offset  int64
	factory func(int64) byte
}

// newSyntheticByteStream returns a new stream of bytes of a specific size,
// given a factory function for generating the byte at a given offset.
func newSyntheticByteStream(size int64, factory func(int64) byte) io.ReadSeeker {
	return &syntheticByteStream{
		size:    size,
		factory: factory,
	}
}

// Read implements the Reader interface for syntheticByteStream
func (s *syntheticByteStream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := s.offset
	end := start + int64(len(p))
	var err error
	if end >= s.size {
		err = io.EOF
		end = s.size
	}

	for idx := start; idx < end; idx++ {
		p[idx-start] = s.factory(idx)
	}

	s.offset = end

	return int(end - start), err
}

// Seek implements the Seeker interface for syntheticByteStream
func (s *syntheticByteStream) Seek(offset int64, whence int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch whence {
	case io.SeekStart:
		s.offset = offset
	case io.SeekCurrent:
		s.offset += offset
	case io.SeekEnd:
		s.offset = s.size - offset
	default:
		return 0, errors.New("Seek: invalid whence")
	}

	if s.offset < 0 {
		return 0, errors.New("Seek: invalid offset")
	}

	return s.offset, nil
}

func sha1hash(input string) string {
	h := sha1.New()
	return fmt.Sprintf("%x", h.Sum([]byte(input)))
}
