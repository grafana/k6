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
	"compress/gzip"
	"compress/zlib"
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

	ntlmssp "github.com/Azure/go-ntlmssp"
	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/sirupsen/logrus"
	null "gopkg.in/guregu/null.v3"

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

// CompressionType is used to specify what compression is to be used to compress the body of a
// request
// The conversion and validation methods are auto-generated with https://github.com/alvaroloes/enumer:
//nolint: lll
//go:generate enumer -type=CompressionType -transform=snake -trimprefix CompressionType -output compression_type_gen.go
type CompressionType uint

const (
	// CompressionTypeGzip compresses through gzip
	CompressionTypeGzip CompressionType = iota
	// CompressionTypeDeflate compresses through flate
	CompressionTypeDeflate
	// CompressionTypeZstd compresses through zstd
	CompressionTypeZstd
	// CompressionTypeBr compresses through brotli
	CompressionTypeBr
	// TODO: add compress(lzw), maybe bzip2 and others listed at
	// https://en.wikipedia.org/wiki/HTTP_compression#Content-Encoding_tokens
)

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

func compressBody(algos []CompressionType, body io.ReadCloser) (*bytes.Buffer, string, error) {
	var contentEncoding string
	var prevBuf io.Reader = body
	var buf *bytes.Buffer
	for _, compressionType := range algos {
		if buf != nil {
			prevBuf = buf
		}
		buf = new(bytes.Buffer)

		if contentEncoding != "" {
			contentEncoding += ", "
		}
		contentEncoding += compressionType.String()
		var w io.WriteCloser
		switch compressionType {
		case CompressionTypeGzip:
			w = gzip.NewWriter(buf)
		case CompressionTypeDeflate:
			w = zlib.NewWriter(buf)
		case CompressionTypeZstd:
			w, _ = zstd.NewWriter(buf)
		case CompressionTypeBr:
			w = brotli.NewWriter(buf)
		default:
			return nil, "", fmt.Errorf("unknown compressionType %s", compressionType)
		}
		// we don't close in defer because zlib will write it's checksum again if it closes twice :(
		var _, err = io.Copy(w, prevBuf)
		if err != nil {
			_ = w.Close()
			return nil, "", err
		}

		if err = w.Close(); err != nil {
			return nil, "", err
		}
	}

	return buf, contentEncoding, body.Close()
}

//nolint:gochecknoglobals
var decompressionErrors = [...]error{
	zlib.ErrChecksum, zlib.ErrDictionary, zlib.ErrHeader,
	gzip.ErrChecksum, gzip.ErrHeader,
	//TODO: handle brotli errors - currently unexported
	zstd.ErrReservedBlockType, zstd.ErrCompressedSizeTooBig, zstd.ErrBlockTooSmall, zstd.ErrMagicMismatch,
	zstd.ErrWindowSizeExceeded, zstd.ErrWindowSizeTooSmall, zstd.ErrDecoderSizeExceeded, zstd.ErrUnknownDictionary,
	zstd.ErrFrameSizeExceeded, zstd.ErrCRCMismatch, zstd.ErrDecoderClosed,
}

func newDecompressionError(originalErr error) K6Error {
	return NewK6Error(
		responseDecompressionErrorCode,
		fmt.Sprintf("error decompressing response body (%s)", originalErr.Error()),
		originalErr,
	)
}

func wrapDecompressionError(err error) error {
	if err == nil {
		return nil
	}

	// TODO: something more optimized? for example, we won't get zstd errors if
	// we don't use it... maybe the code that builds the decompression readers
	// could also add an appropriate error-wrapper layer?
	for _, decErr := range decompressionErrors {
		if err == decErr {
			return newDecompressionError(err)
		}
	}
	if strings.HasPrefix(err.Error(), "brotli: ") { //TODO: submit an upstream patch and fix...
		return newDecompressionError(err)
	}
	return err
}

func readResponseBody(
	state *lib.State, respType ResponseType, resp *http.Response, respErr error,
) (interface{}, error) {

	if resp == nil || respErr != nil {
		return nil, respErr
	}

	if respType == ResponseTypeNone {
		_, err := io.Copy(ioutil.Discard, resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			respErr = err
		}
		return nil, respErr
	}

	rc := &readCloser{resp.Body}
	// Ensure that the entire response body is read and closed, e.g. in case of decoding errors
	defer func(respBody io.ReadCloser) {
		_, _ = io.Copy(ioutil.Discard, respBody)
		_ = respBody.Close()
	}(resp.Body)

	// Transparently decompress the body if it's has a content-encoding we
	// support. If not, simply return it as it is.
	contentEncoding := strings.TrimSpace(resp.Header.Get("Content-Encoding"))
	//TODO: support stacked compressions, e.g. `deflate, gzip`
	if compression, err := CompressionTypeString(contentEncoding); err == nil {
		var decoder io.Reader
		var err error
		switch compression {
		case CompressionTypeDeflate:
			decoder, err = zlib.NewReader(resp.Body)
		case CompressionTypeGzip:
			decoder, err = gzip.NewReader(resp.Body)
		case CompressionTypeZstd:
			decoder, err = zstd.NewReader(resp.Body)
		case CompressionTypeBr:
			decoder = brotli.NewReader(resp.Body)
		default:
			// We have not implemented a compression ... :(
			err = fmt.Errorf(
				"unsupported compression type %s - this is a bug in k6, please report it",
				compression,
			)
		}
		if err != nil {
			return nil, newDecompressionError(err)
		}
		rc = &readCloser{decoder}
	}

	buf := state.BPool.Get()
	defer state.BPool.Put(buf)
	buf.Reset()
	_, err := io.Copy(buf, rc.Reader)
	if err != nil {
		respErr = wrapDecompressionError(err)
	}

	err = rc.Close()
	if err != nil && respErr == nil { // Don't overwrite previous errors
		respErr = wrapDecompressionError(err)
	}

	var result interface{}
	// Binary or string
	switch respType {
	case ResponseTypeText:
		result = buf.String()
	case ResponseTypeBinary:
		// Copy the data to a new slice before we return the buffer to the pool,
		// because buf.Bytes() points to the underlying buffer byte slice.
		binData := make([]byte, buf.Len())
		copy(binData, buf.Bytes())
		result = binData
	default:
		respErr = fmt.Errorf("unknown responseType %s", respType)
	}

	return result, respErr
}

//TODO: move as a response method? or constructor?
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

	tracerTransport := newTransport(state, tags)
	var transport http.RoundTripper = tracerTransport

	if state.Options.HttpDebug.String != "" {
		transport = httpDebugTransport{
			originalTransport: transport,
			httpDebugOption:   state.Options.HttpDebug.String,
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
		Timeout:   preq.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			resp.URL = req.URL.String()

			// Update active jar with cookies found in "Set-Cookie" header(s) of redirect response
			if preq.ActiveJar != nil {
				if respCookies := req.Response.Cookies(); len(respCookies) > 0 {
					preq.ActiveJar.SetCookies(req.URL, respCookies)
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

	mreq := preq.Req.WithContext(ctx)
	res, resErr := client.Do(mreq)

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
