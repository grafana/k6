/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package httpext

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/go-ntlmssp"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

// HTTPRequestCookie is a representation of a cookie used for request objects
type HTTPRequestCookie struct {
	Name, Value string
	Replace     bool
}

// A URL wraps net.URL, and preserves the template (if any) the URL was constructed from.
type URL struct {
	u        *url.URL
	Name     string // http://example.com/thing/${}/
	URL      string // http://example.com/thing/1234/
	CleanURL string // URL with masked user credentials, used for output
}

// NewURL returns a new URL for the provided url and name. The error is returned if the url provided
// can't be parsed
func NewURL(urlString, name string) (URL, error) {
	u, err := url.Parse(urlString)
	newURL := URL{u: u, Name: name, URL: urlString}
	newURL.CleanURL = newURL.Clean()
	if urlString == name {
		newURL.Name = newURL.CleanURL
	}
	return newURL, err
}

// Clean returns an output-safe representation of URL
func (u URL) Clean() string {
	if u.CleanURL != "" {
		return u.CleanURL
	}

	if u.u == nil || u.u.User == nil {
		return u.URL
	}

	if password, passwordOk := u.u.User.Password(); passwordOk {
		// here 3 is for the '://' and 4 is because of '://' and ':' between the credentials
		return u.URL[:len(u.u.Scheme)+3] + "****:****" + u.URL[len(u.u.Scheme)+4+len(u.u.User.Username())+len(password):]
	}

	// here 3 in both places is for the '://'
	return u.URL[:len(u.u.Scheme)+3] + "****" + u.URL[len(u.u.Scheme)+3+len(u.u.User.Username()):]
}

// GetURL returns the internal url.URL
func (u URL) GetURL() *url.URL {
	return u.u
}

// Request represent an http request
type Request struct {
	Method  string                          `json:"method"`
	URL     string                          `json:"url"`
	Headers map[string][]string             `json:"headers"`
	Body    string                          `json:"body"`
	Cookies map[string][]*HTTPRequestCookie `json:"cookies"`
}

// ParsedHTTPRequest a represantion of a request after it has been parsed from a user script
type ParsedHTTPRequest struct {
	URL          *URL
	Body         *bytes.Buffer
	Req          *http.Request
	Timeout      time.Duration
	Auth         string
	Throw        bool
	ResponseType ResponseType
	Compressions []CompressionType
	Redirects    null.Int
	ActiveJar    *cookiejar.Jar
	Cookies      map[string]*HTTPRequestCookie
	Tags         map[string]string
}

// Matches non-compliant io.Closer implementations (e.g. zstd.Decoder)
type ncloser interface {
	Close()
}

type readCloser struct {
	io.Reader
}

// Close readers with differing Close() implementations
func (r readCloser) Close() error {
	var err error
	switch v := r.Reader.(type) {
	case io.Closer:
		err = v.Close()
	case ncloser:
		v.Close()
	}
	return err
}

func stdCookiesToHTTPRequestCookies(cookies []*http.Cookie) map[string][]*HTTPRequestCookie {
	var result = make(map[string][]*HTTPRequestCookie, len(cookies))
	for _, cookie := range cookies {
		result[cookie.Name] = append(result[cookie.Name],
			&HTTPRequestCookie{Name: cookie.Name, Value: cookie.Value})
	}
	return result
}

// TODO: move as a response method? or constructor?
func updateK6Response(k6Response *Response, finishedReq *finishedRequest) {
	k6Response.ErrorCode = int(finishedReq.errorCode)
	k6Response.Error = finishedReq.errorMsg
	trail := finishedReq.trail

	if trail.ConnRemoteAddr != nil {
		remoteHost, remotePortStr, _ := net.SplitHostPort(trail.ConnRemoteAddr.String())
		remotePort, _ := strconv.Atoi(remotePortStr)
		k6Response.RemoteIP = remoteHost
		k6Response.RemotePort = remotePort
	}
	k6Response.Timings = ResponseTimings{
		Duration:       stats.D(trail.Duration),
		Blocked:        stats.D(trail.Blocked),
		Connecting:     stats.D(trail.Connecting),
		TLSHandshaking: stats.D(trail.TLSHandshaking),
		Sending:        stats.D(trail.Sending),
		Waiting:        stats.D(trail.Waiting),
		Receiving:      stats.D(trail.Receiving),
	}
}

// MakeRequest makes http request for tor the provided ParsedHTTPRequest
func MakeRequest(ctx context.Context, preq *ParsedHTTPRequest) (*Response, error) {
	state := lib.GetState(ctx)

	respReq := &Request{
		Method:  preq.Req.Method,
		URL:     preq.Req.URL.String(),
		Cookies: stdCookiesToHTTPRequestCookies(preq.Req.Cookies()),
		Headers: preq.Req.Header,
	}

	if preq.Body != nil {
		// TODO: maybe hide this behind of flag in order for this to not happen for big post/puts?
		// should we set this after the compression? what will be the point ?
		respReq.Body = preq.Body.String()

		if len(preq.Compressions) > 0 {
			compressedBody, contentEncoding, err := compressBody(preq.Compressions, ioutil.NopCloser(preq.Body))
			if err != nil {
				return nil, err
			}
			preq.Body = compressedBody

			currentContentEncoding := preq.Req.Header.Get("Content-Encoding")
			if currentContentEncoding == "" {
				preq.Req.Header.Set("Content-Encoding", contentEncoding)
			} else if currentContentEncoding != contentEncoding {
				state.Logger.Warningf(
					"There's a mismatch between the desired `compression` the manually set `Content-Encoding` header "+
						"in the %s request for '%s', the custom header has precedence and won't be overwritten. "+
						"This may result in invalid data being sent to the server.", preq.Req.Method, preq.Req.URL,
				)
			}
		}

		preq.Req.ContentLength = int64(preq.Body.Len()) // This will make Go set the content-length header
		preq.Req.GetBody = func() (io.ReadCloser, error) {
			//  using `Bytes()` should reuse the same buffer and as such help with the memory usage. We
			//  should not be writing to it any way so there shouldn't be way to corrupt it (?)
			return ioutil.NopCloser(bytes.NewBuffer(preq.Body.Bytes())), nil
		}
		// as per the documentation using GetBody still requires setting the Body.
		preq.Req.Body, _ = preq.Req.GetBody()
	}

	if contentLengthHeader := preq.Req.Header.Get("Content-Length"); contentLengthHeader != "" {
		// The content-length header was set by the user, delete it (since Go
		// will set it automatically) and warn if there were differences
		preq.Req.Header.Del("Content-Length")
		length, err := strconv.Atoi(contentLengthHeader)
		if err != nil || preq.Req.ContentLength != int64(length) {
			state.Logger.Warnf(
				"The specified Content-Length header %q in the %s request for %s "+
					"doesn't match the actual request body length of %d, so it will be ignored!",
				contentLengthHeader, preq.Req.Method, preq.Req.URL, preq.Req.ContentLength,
			)
		}
	}

	tags := state.CloneTags()
	// Override any global tags with request-specific ones.
	for k, v := range preq.Tags {
		tags[k] = v
	}

	// Only set the name system tag if the user didn't explicitly set it beforehand,
	// and the Name was generated from a tagged template string (via http.url).
	if _, ok := tags["name"]; !ok && state.Options.SystemTags.Has(stats.TagName) &&
		preq.URL.Name != "" && preq.URL.Name != preq.URL.Clean() {
		tags["name"] = preq.URL.Name
	}

	// Check rate limit *after* we've prepared a request; no need to wait with that part.
	if rpsLimit := state.RPSLimit; rpsLimit != nil {
		if err := rpsLimit.Wait(ctx); err != nil {
			return nil, err
		}
	}

	tracerTransport := newTransport(ctx, state, tags)
	var transport http.RoundTripper = tracerTransport

	// Combine tags with common log fields
	combinedLogFields := map[string]interface{}{"source": "http-debug", "vu": state.Vu, "iter": state.Iteration}
	for k, v := range tags {
		if _, present := combinedLogFields[k]; !present {
			combinedLogFields[k] = v
		}
	}

	if state.Options.HTTPDebug.String != "" {
		transport = httpDebugTransport{
			originalTransport: transport,
			httpDebugOption:   state.Options.HTTPDebug.String,
			logger:            state.Logger.WithFields(combinedLogFields),
		}
	}

	if preq.Auth == "digest" {
		transport = digestTransport{originalTransport: transport}
	} else if preq.Auth == "ntlm" {
		transport = ntlmssp.Negotiator{RoundTripper: transport}
	}

	resp := &Response{ctx: ctx, URL: preq.URL.URL, Request: *respReq}
	client := http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			resp.URL = req.URL.String()

			// Update active jar with cookies found in "Set-Cookie" header(s) of redirect response
			if preq.ActiveJar != nil {
				if respCookies := req.Response.Cookies(); len(respCookies) > 0 {
					preq.ActiveJar.SetCookies(via[len(via)-1].URL, respCookies)
				}
				req.Header.Del("Cookie")
				SetRequestCookies(req, preq.ActiveJar, preq.Cookies)
			}

			if l := len(via); int64(l) > preq.Redirects.Int64 {
				if !preq.Redirects.Valid {
					url := req.URL
					if l > 0 {
						url = via[0].URL
					}
					state.Logger.WithFields(logrus.Fields{"url": url.String()}).Warnf(
						"Stopped after %d redirects and returned the redirection; pass { redirects: n }"+
							" in request params or set global maxRedirects to silence this", l)
				}
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	reqCtx, cancelFunc := context.WithTimeout(ctx, preq.Timeout)
	defer cancelFunc()
	mreq := preq.Req.WithContext(reqCtx)
	res, resErr := client.Do(mreq)

	// TODO(imiric): It would be safer to check for a writeable
	// response body here instead of status code, but those are
	// wrapped in a read-only body when using client timeouts and are
	// unusable until https://github.com/golang/go/issues/31391 is fixed.
	if res != nil && res.StatusCode == http.StatusSwitchingProtocols {
		_ = res.Body.Close()
		return nil, fmt.Errorf("unsupported response status: %s", res.Status)
	}

	resp.Body, resErr = readResponseBody(state, preq.ResponseType, res, resErr)
	finishedReq := tracerTransport.processLastSavedRequest(wrapDecompressionError(resErr))
	if finishedReq != nil {
		updateK6Response(resp, finishedReq)
	}

	if resErr == nil {
		if preq.ActiveJar != nil {
			if rc := res.Cookies(); len(rc) > 0 {
				preq.ActiveJar.SetCookies(res.Request.URL, rc)
			}
		}

		resp.URL = res.Request.URL.String()
		resp.Status = res.StatusCode
		resp.StatusText = res.Status
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
		if preq.Throw { // if we are going to throw, we shouldn't log it
			return nil, resErr
		}

		// Do *not* log errors about the context being cancelled.
		select {
		case <-ctx.Done():
		default:
			state.Logger.WithField("error", resErr).Warn("Request Failed")
		}
	}

	return resp, nil
}

// SetRequestCookies sets the cookies of the requests getting those cookies both from the jar and
// from the reqCookies map. The Replace field of the HTTPRequestCookie will be taken into account
func SetRequestCookies(req *http.Request, jar *cookiejar.Jar, reqCookies map[string]*HTTPRequestCookie) {
	var replacedCookies = make(map[string]struct{})
	for key, reqCookie := range reqCookies {
		req.AddCookie(&http.Cookie{Name: key, Value: reqCookie.Value})
		if reqCookie.Replace {
			replacedCookies[key] = struct{}{}
		}
	}
	for _, c := range jar.Cookies(req.URL) {
		if _, ok := replacedCookies[c.Name]; !ok {
			req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
		}
	}
}
