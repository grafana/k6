package netext

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	ntlmssp "github.com/Azure/go-ntlmssp"
	digest "github.com/Soontao/goHttpDigestClient"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"
)

// HTTPRequestCookie is a representation of a cookie used for request objects
type HTTPRequestCookie struct {
	Name, Value string
	Replace     bool
}

// A URL wraps net.URL, and preserves the template (if any) the URL was constructed from.
type URL struct {
	u    *url.URL
	Name string // http://example.com/thing/${}/
	URL  string // http://example.com/thing/1234/
}

// NewURL returns a new URL for the provided url and name. The error is returned if the url provided
// can't be parsed
func NewURL(urlString, name string) (URL, error) {
	u, err := url.Parse(urlString)
	return URL{u: u, Name: name, URL: urlString}, err
}

// GetURL returns the internal url.URL
func (u URL) GetURL() *url.URL {
	return u.u
}

// ParsedHTTPRequest a represantion of a request after it has been parsed from a user script
type ParsedHTTPRequest struct {
	URL           *URL
	Body          *bytes.Buffer
	Req           *http.Request
	Timeout       time.Duration
	Auth          string
	Throw         bool
	ResponseType  ResponseType
	Redirects     null.Int
	ActiveJar     *cookiejar.Jar
	Cookies       map[string]*HTTPRequestCookie
	MergedCookies map[string][]*HTTPRequestCookie
	Tags          map[string]string
}

// Do makes http request for tor the provided ParsedHTTPRequest
func Do(ctx context.Context, preq *ParsedHTTPRequest) (*Response, error) {
	state := lib.GetState(ctx)

	respReq := &Request{
		Method:  preq.Req.Method,
		URL:     preq.Req.URL.String(),
		Cookies: preq.MergedCookies,
		Headers: preq.Req.Header,
	}
	if preq.Body != nil {
		respReq.Body = preq.Body.String()
	}

	tags := state.Options.RunTags.CloneTags()
	for k, v := range preq.Tags {
		tags[k] = v
	}

	if state.Options.SystemTags["method"] {
		tags["method"] = preq.Req.Method
	}
	if state.Options.SystemTags["url"] {
		tags["url"] = preq.URL.URL
	}

	// Only set the name system tag if the user didn't explicitly set it beforehand
	if _, ok := tags["name"]; !ok && state.Options.SystemTags["name"] {
		tags["name"] = preq.URL.Name
	}
	if state.Options.SystemTags["group"] {
		tags["group"] = state.Group.Path
	}
	if state.Options.SystemTags["vu"] {
		tags["vu"] = strconv.FormatInt(state.Vu, 10)
	}
	if state.Options.SystemTags["iter"] {
		tags["iter"] = strconv.FormatInt(state.Iteration, 10)
	}

	// Check rate limit *after* we've prepared a request; no need to wait with that part.
	if rpsLimit := state.RPSLimit; rpsLimit != nil {
		if err := rpsLimit.Wait(ctx); err != nil {
			return nil, err
		}
	}

	tracerTransport := NewTransport(state.Transport, state.Samples, &state.Options, tags)
	var transport http.RoundTripper = tracerTransport
	if preq.Auth == "ntlm" {
		transport = ntlmssp.Negotiator{
			RoundTripper: tracerTransport,
		}
	}

	resp := &Response{ctx: ctx, URL: preq.URL.URL, Request: *respReq}
	client := http.Client{
		Transport: transport,
		Timeout:   preq.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			debugResponse(state, req.Response, "RedirectResponse")

			// Update active jar with cookies found in "Set-Cookie" header(s) of redirect response
			if preq.ActiveJar != nil {
				if respCookies := req.Response.Cookies(); len(respCookies) > 0 {
					preq.ActiveJar.SetCookies(req.URL, respCookies)
				}
				req.Header.Del("Cookie")
				mergedCookies := mergeCookies(req, preq.ActiveJar, preq.Cookies)

				setRequestCookies(req, mergedCookies)
			}

			if l := len(via); int64(l) > preq.Redirects.Int64 {
				if !preq.Redirects.Valid {
					url := req.URL
					if l > 0 {
						url = via[0].URL
					}
					state.Logger.WithFields(log.Fields{"url": url.String()}).Warnf("Stopped after %d redirects and returned the redirection; pass { redirects: n } in request params or set global maxRedirects to silence this", l)
				}
				return http.ErrUseLastResponse
			}
			debugRequest(state, req, "RedirectRequest")
			return nil
		},
	}

	// if digest authentication option is passed, make an initial request to get the authentication params to compute the authorization header
	if preq.Auth == "digest" {
		username := preq.URL.u.User.Username()
		password, _ := preq.URL.u.User.Password()

		// removing user from URL to avoid sending the authorization header fo basic auth
		preq.Req.URL.User = nil

		debugRequest(state, preq.Req, "DigestRequest")
		res, err := client.Do(preq.Req.WithContext(ctx))
		debugRequest(state, preq.Req, "DigestResponse")
		if err != nil {
			// Do *not* log errors about the contex being cancelled.
			select {
			case <-ctx.Done():
			default:
				state.Logger.WithField("error", res).Warn("Digest request failed")
			}

			if preq.Throw {
				return nil, err
			}

			resp.setError(err)
			return resp, nil
		}

		if res.StatusCode == http.StatusUnauthorized {
			body := ""
			if b, err := ioutil.ReadAll(res.Body); err == nil {
				body = string(b)
			}

			challenge := digest.GetChallengeFromHeader(&res.Header)
			challenge.ComputeResponse(preq.Req.Method, preq.Req.URL.RequestURI(), body, username, password)
			authorization := challenge.ToAuthorizationStr()
			preq.Req.Header.Set(digest.KEY_AUTHORIZATION, authorization)
		}
	}

	debugRequest(state, preq.Req, "Request")
	res, resErr := client.Do(preq.Req.WithContext(ctx))
	debugResponse(state, res, "Response")
	if resErr == nil && res != nil {
		switch res.Header.Get("Content-Encoding") {
		case "deflate":
			res.Body, resErr = zlib.NewReader(res.Body)
		case "gzip":
			res.Body, resErr = gzip.NewReader(res.Body)
		}
	}
	if resErr == nil && res != nil {
		if preq.ResponseType == ResponseTypeNone {
			_, err := io.Copy(ioutil.Discard, res.Body)
			if err != nil && err != io.EOF {
				resErr = err
			}
			resp.Body = nil
		} else {
			// Binary or string
			buf := state.BPool.Get()
			buf.Reset()
			defer state.BPool.Put(buf)
			_, err := io.Copy(buf, res.Body)
			if err != nil && err != io.EOF {
				resErr = err
			}

			switch preq.ResponseType {
			case ResponseTypeText:
				resp.Body = buf.String()
			case ResponseTypeBinary:
				resp.Body = buf.Bytes()
			default:
				resErr = fmt.Errorf("unknown responseType %s", preq.ResponseType)
			}
		}
		_ = res.Body.Close()
	}

	trail := tracerTransport.GetTrail()

	if trail.ConnRemoteAddr != nil {
		remoteHost, remotePortStr, _ := net.SplitHostPort(trail.ConnRemoteAddr.String())
		remotePort, _ := strconv.Atoi(remotePortStr)
		resp.RemoteIP = remoteHost
		resp.RemotePort = remotePort
	}
	resp.Timings = ResponseTimings{
		Duration:       stats.D(trail.Duration),
		Blocked:        stats.D(trail.Blocked),
		Connecting:     stats.D(trail.Connecting),
		TLSHandshaking: stats.D(trail.TLSHandshaking),
		Sending:        stats.D(trail.Sending),
		Waiting:        stats.D(trail.Waiting),
		Receiving:      stats.D(trail.Receiving),
	}

	if resErr != nil {
		resp.setError(resErr)
	} else {
		if preq.ActiveJar != nil {
			if rc := res.Cookies(); len(rc) > 0 {
				preq.ActiveJar.SetCookies(res.Request.URL, rc)
			}
		}

		resp.URL = res.Request.URL.String()
		resp.setStatusCode(res.StatusCode)
		resp.Proto = res.Proto

		if res.TLS != nil {
			resp.setTLSInfo(res.TLS)
		}

		resp.Headers = make(map[string]string, len(res.Header))
		for k, vs := range res.Header {
			resp.Headers[k] = strings.Join(vs, ", ")
		}

		resCookies := res.Cookies()
		resp.Cookies = make(map[string][]*HTTPCookie, len(resCookies))
		for _, c := range resCookies {
			resp.Cookies[c.Name] = append(resp.Cookies[c.Name], &HTTPCookie{
				Name:     c.Name,
				Value:    c.Value,
				Domain:   c.Domain,
				Path:     c.Path,
				HTTPOnly: c.HttpOnly,
				Secure:   c.Secure,
				MaxAge:   c.MaxAge,
				Expires:  c.Expires.UnixNano() / 1000000,
			})
		}
	}

	if resErr != nil {
		// Do *not* log errors about the contex being cancelled.
		select {
		case <-ctx.Done():
		default:
			state.Logger.WithField("error", resErr).Warn("Request Failed")
		}

		if preq.Throw {
			return nil, resErr
		}
	}

	return resp, nil
}

// This should be used instead of setting Error as it will correctly set ErrorCode as well
func (res *Response) setError(err error) {
	var errorCode, errorMsg = errorCodeForError(err)
	res.ErrorCode = int(errorCode)
	if errorMsg == "" {
		errorMsg = err.Error()
	}
	res.Error = errorMsg
}

// This should be used instead of setting Error as it will correctly set ErrorCode as well
func (res *Response) setStatusCode(statusCode int) {
	res.Status = statusCode
	if statusCode >= 400 && statusCode < 600 {
		res.ErrorCode = 1000 + statusCode
		// TODO: maybe set the res.Error to some custom message
	}
}

func (res *Response) setTLSInfo(tlsState *tls.ConnectionState) {
	tlsInfo, oscp := ParseTLSConnState(tlsState)
	res.TLSVersion = tlsInfo.Version
	res.TLSCipherSuite = tlsInfo.CipherSuite
	res.OCSP = oscp
}

func mergeCookies(req *http.Request, jar *cookiejar.Jar, reqCookies map[string]*HTTPRequestCookie) map[string][]*HTTPRequestCookie {
	allCookies := make(map[string][]*HTTPRequestCookie)
	for _, c := range jar.Cookies(req.URL) {
		allCookies[c.Name] = append(allCookies[c.Name], &HTTPRequestCookie{Name: c.Name, Value: c.Value})
	}
	for key, reqCookie := range reqCookies {
		if jc := allCookies[key]; jc != nil && reqCookie.Replace {
			allCookies[key] = []*HTTPRequestCookie{{Name: key, Value: reqCookie.Value}}
		} else {
			allCookies[key] = append(allCookies[key], &HTTPRequestCookie{Name: key, Value: reqCookie.Value})
		}
	}
	return allCookies
}

func setRequestCookies(req *http.Request, reqCookies map[string][]*HTTPRequestCookie) {
	for _, cookies := range reqCookies {
		for _, c := range cookies {
			req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
		}
	}
}

func debugRequest(state *lib.State, req *http.Request, description string) {
	if state.Options.HttpDebug.String != "" {
		dump, err := httputil.DumpRequestOut(req, state.Options.HttpDebug.String == "full")
		if err != nil {
			log.Fatal(err)
		}
		logDump(description, dump)
	}
}

// ResponseType is used in the request to specify how the response body should be treated
// The conversion and validation methods are auto-generated with https://github.com/alvaroloes/enumer:
//go:generate enumer -type=ResponseType -transform=snake -json -text -trimprefix ResponseType -output response_type_gen.go
type ResponseType uint

const (
	// ResponseTypeText causes k6 to return the response body as a string. It works
	// well for web pages and JSON documents, but it can cause issues with
	// binary files since their data could be lost when they're converted in the
	// UTF-16 strings JavaScript uses.
	// This is the default value for backwards-compatibility, unless the global
	// discardResponseBodies option is enabled.
	ResponseTypeText ResponseType = iota
	// ResponseTypeBinary causes k6 to return the response body as a []byte, suitable
	// for working with binary files without lost data and needless string conversions.
	ResponseTypeBinary
	// ResponseTypeNone causes k6 to fully read the response body while immediately
	// discarding the actual data - k6 would set the body of the returned HTTPResponse
	// to null. This saves CPU and memory and is suitable for HTTP requests that we just
	// want to  measure, but we don't care about their responses' contents. This is the
	// default value for all requests if the global discardResponseBodies is enablled.
	ResponseTypeNone
)

// Response is a representation of an HTTP response
type Response struct {
	ctx context.Context

	RemoteIP       string                   `json:"remote_ip"`
	RemotePort     int                      `json:"remote_port"`
	URL            string                   `json:"url"`
	Status         int                      `json:"status"`
	Proto          string                   `json:"proto"`
	Headers        map[string]string        `json:"headers"`
	Cookies        map[string][]*HTTPCookie `json:"cookies"`
	Body           interface{}              `json:"body"`
	Timings        ResponseTimings          `json:"timings"`
	TLSVersion     string                   `json:"tls_version"`
	TLSCipherSuite string                   `json:"tls_cipher_suite"`
	OCSP           OCSP                     `json:"ocsp"`
	Error          string                   `json:"error"`
	ErrorCode      int                      `json:"error_code"`
	Request        Request                  `json:"request"`
}

// GetCtx return the response context
func (res *Response) GetCtx() context.Context {
	return res.ctx
}

// Request represent an http request
type Request struct {
	Method  string                          `json:"method"`
	URL     string                          `json:"url"`
	Headers map[string][]string             `json:"headers"`
	Body    string                          `json:"body"`
	Cookies map[string][]*HTTPRequestCookie `json:"cookies"`
}

// ResponseTimings is a struct to put all timings for a given HTTP response/request
type ResponseTimings struct {
	Duration       float64 `json:"duration"`
	Blocked        float64 `json:"blocked"`
	LookingUp      float64 `json:"looking_up"`
	Connecting     float64 `json:"connecting"`
	TLSHandshaking float64 `json:"tls_handshaking"`
	Sending        float64 `json:"sending"`
	Waiting        float64 `json:"waiting"`
	Receiving      float64 `json:"receiving"`
}

// HTTPCookie is a representation of an http cookies used in the Response object
type HTTPCookie struct {
	Name, Value, Domain, Path string
	HTTPOnly, Secure          bool
	MaxAge                    int
	Expires                   int64
}

func debugResponse(state *lib.State, res *http.Response, description string) {
	if state.Options.HttpDebug.String != "" && res != nil {
		dump, err := httputil.DumpResponse(res, state.Options.HttpDebug.String == "full")
		if err != nil {
			log.Fatal(err)
		}
		logDump(description, dump)
	}
}

func logDump(description string, dump []byte) {
	fmt.Printf("%s:\n%s\n", description, dump)
}
