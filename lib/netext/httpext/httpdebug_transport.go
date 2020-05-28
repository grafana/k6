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
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/sirupsen/logrus"
)

type httpDebugTransport struct {
	//TODO: get the state and log to its Logger
	originalTransport http.RoundTripper
	httpDebugOption   string
}

// RoundTrip prints passing HTTP requests and received responses
//
// TODO: massively improve this, because the printed information can be wrong:
//  - https://github.com/loadimpact/k6/issues/986
//  - https://github.com/loadimpact/k6/issues/1042
//  - https://github.com/loadimpact/k6/issues/774
func (t httpDebugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.debugRequest(req)
	resp, err := t.originalTransport.RoundTrip(req)
	t.debugResponse(resp)
	return resp, err
}

func (t httpDebugTransport) debugRequest(req *http.Request) {
	dump, err := httputil.DumpRequestOut(req, t.httpDebugOption == "full")
	if err != nil {
		logrus.Error(err)
	}
	fmt.Printf("Request:\n%s\n", dump) //TODO: fix...
}

func (t httpDebugTransport) debugResponse(res *http.Response) {
	if res != nil {
		dump, err := httputil.DumpResponse(res, t.httpDebugOption == "full")
		if err != nil {
			logrus.Error(err)
		}
		fmt.Printf("Response:\n%s\n", dump) //TODO: fix...
	}
}
