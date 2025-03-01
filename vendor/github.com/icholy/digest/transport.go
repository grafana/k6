package digest

import (
	"bytes"
	"io"
	"net/http"
	"sync"
)

// cchal is a cached challenge and the number of times it's been used.
type cchal struct {
	c *Challenge
	n int
}

// Transport implements http.RoundTripper
type Transport struct {
	Username string
	Password string

	// Digest computes the digest credentials.
	// If nil, the Digest function is used.
	Digest func(*http.Request, *Challenge, Options) (*Credentials, error)

	// FindChallenge extracts the challenge from the request headers.
	// If nil, the FindChallenge function is used.
	FindChallenge func(http.Header) (*Challenge, error)

	// Transport specifies the mechanism by which individual
	// HTTP requests are made.
	// If nil, DefaultTransport is used.
	Transport http.RoundTripper

	// Jar specifies the cookie jar.
	//
	// The Jar is used to insert relevant cookies into every
	// outbound Request and is updated with the cookie values
	// of every inbound Response. The Jar is consulted for every
	// redirect that the Client follows.
	//
	// If Jar is nil, cookies are only sent if they are explicitly
	// set on the Request.
	Jar http.CookieJar

	// NoReuse prevents the transport from reusing challenges.
	NoReuse bool

	// cache of challenges indexed by host
	cache   map[string]*cchal
	cacheMu sync.Mutex
}

// save parses the digest challenge from the response
// and adds it to the cache
func (t *Transport) save(res *http.Response) error {
	// save cookies
	if t.Jar != nil {
		t.Jar.SetCookies(res.Request.URL, res.Cookies())
	}
	// find and save digest challenge
	find := t.FindChallenge
	if find == nil {
		find = FindChallenge
	}
	chal, err := find(res.Header)
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	if t.cache == nil {
		t.cache = map[string]*cchal{}
	}
	// TODO: if the challenge contains a domain, we should be using that
	//       to match against outgoing requests. We're currently ignoring
	//       it and just matching the hostname. That being said, none of
	//       the major browsers respect the domain either.
	host := res.Request.URL.Hostname()
	if err != nil {
		// if save is being invoked, the existing cached challenge didn't work
		delete(t.cache, host)
		return err
	}
	t.cache[host] = &cchal{c: chal}
	return nil
}

// digest creates credentials from the cached challenge
func (t *Transport) digest(req *http.Request, chal *Challenge, count int) (*Credentials, error) {
	opt := Options{
		Method:   req.Method,
		URI:      req.URL.RequestURI(),
		GetBody:  req.GetBody,
		Count:    count,
		Username: t.Username,
		Password: t.Password,
	}
	if t.Digest != nil {
		return t.Digest(req, chal, opt)
	}
	return Digest(chal, opt)
}

// challenge returns a cached challenge and count for the provided request
func (t *Transport) challenge(req *http.Request) (*Challenge, int, bool) {
	t.cacheMu.Lock()
	defer t.cacheMu.Unlock()
	host := req.URL.Hostname()
	cc, ok := t.cache[host]
	if !ok {
		return nil, 0, false
	}
	if t.NoReuse {
		delete(t.cache, host)
	}
	cc.n++
	return cc.c, cc.n, true
}

// prepare attempts to find a cached challenge that matches the
// requested domain, and use it to set the Authorization header
func (t *Transport) prepare(req *http.Request) error {
	// add cookies
	if t.Jar != nil {
		for _, cookie := range t.Jar.Cookies(req.URL) {
			req.AddCookie(cookie)
		}
	}
	// add auth
	chal, count, ok := t.challenge(req)
	if !ok {
		return nil
	}
	cred, err := t.digest(req, chal, count)
	if err != nil {
		return err
	}
	if cred != nil {
		req.Header.Set("Authorization", cred.String())
	}
	return nil
}

// RoundTrip will try to authorize the request using a cached challenge.
// If that doesn't work and we receive a 401, we'll try again using that challenge.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// use the configured transport if there is one
	tr := t.Transport
	if tr == nil {
		tr = http.DefaultTransport
	}
	// don't modify the original request
	clone, err := cloner(req)
	if err != nil {
		return nil, err
	}
	// make a copy of the request
	first, err := clone()
	if err != nil {
		return nil, err
	}
	// prepare the first request using a cached challenge
	if err := t.prepare(first); err != nil {
		return nil, err
	}
	// the first request will either succeed or return a 401
	res, err := tr.RoundTrip(first)
	if err != nil || res.StatusCode != http.StatusUnauthorized {
		return res, err
	}
	// drain and close the first message body
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	// save the challenge for future use
	if err := t.save(res); err != nil {
		if err == ErrNoChallenge {
			return res, nil
		}
		return nil, err
	}
	// make a second copy of the request
	second, err := clone()
	if err != nil {
		return nil, err
	}
	// prepare the second request based on the new challenge
	if err := t.prepare(second); err != nil {
		return nil, err
	}
	return tr.RoundTrip(second)
}

// CloseIdleConnections delegates the call to the underlying transport.
func (t *Transport) CloseIdleConnections() {
	tr := t.Transport
	if tr == nil {
		tr = http.DefaultTransport
	}
	type closeIdler interface {
		CloseIdleConnections()
	}
	if tr, ok := tr.(closeIdler); ok {
		tr.CloseIdleConnections()
	}
}

// cloner returns a function which makes clones of the provided request
func cloner(req *http.Request) (func() (*http.Request, error), error) {
	getbody := req.GetBody
	// if there's no GetBody function set we have to copy the body
	// into memory to use for future clones
	if getbody == nil {
		if req.Body == nil || req.Body == http.NoBody {
			getbody = func() (io.ReadCloser, error) {
				return http.NoBody, nil
			}
		} else {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if err := req.Body.Close(); err != nil {
				return nil, err
			}
			getbody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(body)), nil
			}
		}
	}
	return func() (*http.Request, error) {
		clone := req.Clone(req.Context())
		body, err := getbody()
		if err != nil {
			return nil, err
		}
		clone.Body = body
		clone.GetBody = getbody
		return clone, nil
	}, nil
}
