/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package http

import (
	"context"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/netext/httpext"
)

const (
	HTTP_METHOD_GET     = "GET"
	HTTP_METHOD_POST    = "POST"
	HTTP_METHOD_PUT     = "PUT"
	HTTP_METHOD_DELETE  = "DELETE"
	HTTP_METHOD_HEAD    = "HEAD"
	HTTP_METHOD_PATCH   = "PATCH"
	HTTP_METHOD_OPTIONS = "OPTIONS"
)

// ErrJarForbiddenInInitContext is used when a cookie jar was made in the init context
var ErrJarForbiddenInInitContext = common.NewInitContextError("Making cookie jars in the init context is not supported")

// New returns a new global module instance
func New() *GlobalHTTP {
	return &GlobalHTTP{}
}

// GlobalHTTP is a global HTTP module for a k6 instance/test run
type GlobalHTTP struct{}

// NewModuleInstancePerVU returns an HTTP instance for each VU
func (g *GlobalHTTP) NewModuleInstancePerVU() interface{} { // this here needs to return interface{}
	return &HTTP{ // change the below fields to be not writable or not fields
		TLS_1_0:                            netext.TLS_1_0,
		TLS_1_1:                            netext.TLS_1_1,
		TLS_1_2:                            netext.TLS_1_2,
		TLS_1_3:                            netext.TLS_1_3,
		OCSP_STATUS_GOOD:                   netext.OCSP_STATUS_GOOD,
		OCSP_STATUS_REVOKED:                netext.OCSP_STATUS_REVOKED,
		OCSP_STATUS_SERVER_FAILED:          netext.OCSP_STATUS_SERVER_FAILED,
		OCSP_STATUS_UNKNOWN:                netext.OCSP_STATUS_UNKNOWN,
		OCSP_REASON_UNSPECIFIED:            netext.OCSP_REASON_UNSPECIFIED,
		OCSP_REASON_KEY_COMPROMISE:         netext.OCSP_REASON_KEY_COMPROMISE,
		OCSP_REASON_CA_COMPROMISE:          netext.OCSP_REASON_CA_COMPROMISE,
		OCSP_REASON_AFFILIATION_CHANGED:    netext.OCSP_REASON_AFFILIATION_CHANGED,
		OCSP_REASON_SUPERSEDED:             netext.OCSP_REASON_SUPERSEDED,
		OCSP_REASON_CESSATION_OF_OPERATION: netext.OCSP_REASON_CESSATION_OF_OPERATION,
		OCSP_REASON_CERTIFICATE_HOLD:       netext.OCSP_REASON_CERTIFICATE_HOLD,
		OCSP_REASON_REMOVE_FROM_CRL:        netext.OCSP_REASON_REMOVE_FROM_CRL,
		OCSP_REASON_PRIVILEGE_WITHDRAWN:    netext.OCSP_REASON_PRIVILEGE_WITHDRAWN,
		OCSP_REASON_AA_COMPROMISE:          netext.OCSP_REASON_AA_COMPROMISE,

		responseCallback: defaultExpectedStatuses.match,
	}
}

//nolint: golint
type HTTP struct {
	TLS_1_0                            string `js:"TLS_1_0"`
	TLS_1_1                            string `js:"TLS_1_1"`
	TLS_1_2                            string `js:"TLS_1_2"`
	TLS_1_3                            string `js:"TLS_1_3"`
	OCSP_STATUS_GOOD                   string `js:"OCSP_STATUS_GOOD"`
	OCSP_STATUS_REVOKED                string `js:"OCSP_STATUS_REVOKED"`
	OCSP_STATUS_SERVER_FAILED          string `js:"OCSP_STATUS_SERVER_FAILED"`
	OCSP_STATUS_UNKNOWN                string `js:"OCSP_STATUS_UNKNOWN"`
	OCSP_REASON_UNSPECIFIED            string `js:"OCSP_REASON_UNSPECIFIED"`
	OCSP_REASON_KEY_COMPROMISE         string `js:"OCSP_REASON_KEY_COMPROMISE"`
	OCSP_REASON_CA_COMPROMISE          string `js:"OCSP_REASON_CA_COMPROMISE"`
	OCSP_REASON_AFFILIATION_CHANGED    string `js:"OCSP_REASON_AFFILIATION_CHANGED"`
	OCSP_REASON_SUPERSEDED             string `js:"OCSP_REASON_SUPERSEDED"`
	OCSP_REASON_CESSATION_OF_OPERATION string `js:"OCSP_REASON_CESSATION_OF_OPERATION"`
	OCSP_REASON_CERTIFICATE_HOLD       string `js:"OCSP_REASON_CERTIFICATE_HOLD"`
	OCSP_REASON_REMOVE_FROM_CRL        string `js:"OCSP_REASON_REMOVE_FROM_CRL"`
	OCSP_REASON_PRIVILEGE_WITHDRAWN    string `js:"OCSP_REASON_PRIVILEGE_WITHDRAWN"`
	OCSP_REASON_AA_COMPROMISE          string `js:"OCSP_REASON_AA_COMPROMISE"`

	responseCallback func(int) bool
}

// XCookieJar creates a new cookie jar object.
func (*HTTP) XCookieJar(ctx *context.Context) *HTTPCookieJar {
	return newCookieJar(ctx)
}

// CookieJar returns the active cookie jar for the current VU.
func (*HTTP) CookieJar(ctx context.Context) (*HTTPCookieJar, error) {
	state := lib.GetState(ctx)
	if state == nil {
		return nil, ErrJarForbiddenInInitContext
	}
	return &HTTPCookieJar{state.CookieJar, &ctx}, nil
}

// URL creates a new URL from the provided parts
func (*HTTP) URL(parts []string, pieces ...string) (httpext.URL, error) {
	var name, urlstr string
	for i, part := range parts {
		name += part
		urlstr += part
		if i < len(pieces) {
			name += "${}"
			urlstr += pieces[i]
		}
	}
	return httpext.NewURL(urlstr, name)
}
