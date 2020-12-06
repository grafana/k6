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
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/tidwall/gjson"

	"github.com/loadimpact/k6/lib/netext"
)

// ResponseType is used in the request to specify how the response body should be treated
// The conversion and validation methods are auto-generated with https://github.com/alvaroloes/enumer:
//nolint: lll
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

type jsonError struct {
	line      int
	character int
	err       error
}

func (j jsonError) Error() string {
	errMessage := "cannot parse json due to an error at line"
	return fmt.Sprintf("%s %d, character %d , error: %v", errMessage, j.line, j.character, j.err)
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

// Response is a representation of an HTTP response
type Response struct {
	ctx context.Context

	RemoteIP       string                   `json:"remote_ip"`
	RemotePort     int                      `json:"remote_port"`
	URL            string                   `json:"url"`
	Status         int                      `json:"status"`
	StatusText     string                   `json:"status_text"`
	Proto          string                   `json:"proto"`
	Headers        map[string]string        `json:"headers"`
	Cookies        map[string][]*HTTPCookie `json:"cookies"`
	Body           interface{}              `json:"body"`
	Timings        ResponseTimings          `json:"timings"`
	TLSVersion     string                   `json:"tls_version"`
	TLSCipherSuite string                   `json:"tls_cipher_suite"`
	OCSP           netext.OCSP              `json:"ocsp"`
	Error          string                   `json:"error"`
	ErrorCode      int                      `json:"error_code"`
	Request        Request                  `json:"request"`

	cachedJSON    interface{}
	validatedJSON bool
}

func (res *Response) setTLSInfo(tlsState *tls.ConnectionState) {
	tlsInfo, oscp := netext.ParseTLSConnState(tlsState)
	res.TLSVersion = tlsInfo.Version
	res.TLSCipherSuite = tlsInfo.CipherSuite
	res.OCSP = oscp
}

// GetCtx return the response context
func (res *Response) GetCtx() context.Context {
	return res.ctx
}

// JSON parses the body of a response as json and returns it to the goja VM
func (res *Response) JSON(selector ...string) (interface{}, error) {
	hasSelector := len(selector) > 0
	if res.cachedJSON == nil || hasSelector {
		var v interface{}
		var body []byte
		switch b := res.Body.(type) {
		case []byte:
			body = b
		case string:
			body = []byte(b)
		default:
			return nil, errors.New("invalid response type")
		}

		if hasSelector {
			if !res.validatedJSON {
				if !gjson.ValidBytes(body) {
					return nil, nil
				}
				res.validatedJSON = true
			}

			result := gjson.GetBytes(body, selector[0])

			if !result.Exists() {
				return nil, nil
			}
			return result.Value(), nil
		}

		if err := json.Unmarshal(body, &v); err != nil {
			if syntaxError, ok := err.(*json.SyntaxError); ok {
				err = checkErrorInJSON(body, int(syntaxError.Offset), err)
			}
			return nil, err
		}
		res.validatedJSON = true
		res.cachedJSON = v
	}
	return res.cachedJSON, nil
}

func checkErrorInJSON(input []byte, offset int, err error) error {
	lf := '\n'
	str := string(input)

	// Humans tend to count from 1.
	line := 1
	character := 0

	for i, b := range str {
		if b == lf {
			line++
			character = 0
		}
		character++
		if i == offset {
			break
		}
	}

	return jsonError{line: line, character: character, err: err}
}
